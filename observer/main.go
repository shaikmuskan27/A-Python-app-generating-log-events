package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
)

func getDockerClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", "/var/run/docker.sock")
			},
		},
	}
}

type DockerContainer struct {
	Id    string   `json:"Id"`
	Names []string `json:"Names"`
}

func getTargetContainerID(client *http.Client, targetName string) (string, error) {
	resp, err := client.Get("http://localhost/containers/json")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var containers []DockerContainer
	if err := json.NewDecoder(resp.Body).Decode(&containers); err != nil {
		return "", err
	}

	for _, ctr := range containers {
		for _, name := range ctr.Names {
			if name == "/"+targetName {
				return ctr.Id, nil
			}
		}
	}
	return "", fmt.Errorf("container %s not found", targetName)
}

func main() {
	log.Println("Starting Observer Service...")

	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}

	// Retry connecting to MongoDB up to 5 times
	var mongoClient *mongo.Client
	var err error
	for i := 0; i < 5; i++ {
		mongoClient, err = ConnectMongo(mongoURI)
		if err == nil {
			break
		}
		log.Printf("MongoDB not ready yet... retrying in 3 seconds (%v)", err)
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB after retries: %v", err)
	}
	defer mongoClient.Disconnect(context.Background())

	dockerClient := getDockerClient()

	targetContainerName := os.Getenv("TARGET_CONTAINER")
	if targetContainerName == "" {
		targetContainerName = "target-app"
	}

	logBuffer := make([]LogEntry, 0)
	var mu sync.Mutex
	flushInterval := 5 * time.Second
	maxBufferSize := 1000

	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			mu.Lock()
			if len(logBuffer) > 0 {
				log.Printf("Flushing %d logs to MongoDB...", len(logBuffer))
				err := FlushBuffer(mongoClient, logBuffer)
				if err != nil {
					log.Printf("Error flushing buffer: %v", err)
				} else {
					logBuffer = make([]LogEntry, 0)
				}
			}
			mu.Unlock()
		}
	}()

	log.Printf("Waiting for container %s to be available...", targetContainerName)
	var targetContainerID string
	for {
		id, err := getTargetContainerID(dockerClient, targetContainerName)
		if err == nil {
			targetContainerID = id
			break
		}
		log.Printf("Looking for container: %v", err)
		time.Sleep(2 * time.Second)
	}

	log.Printf("Found container %s (ID: %s). Tailing logs...", targetContainerName, targetContainerID[:12])

	go func() {
		for {
			url := fmt.Sprintf("http://localhost/containers/%s/logs?stdout=1&stderr=1&follow=1&tail=0", targetContainerID)
			resp, err := dockerClient.Get(url)
			if err != nil {
				log.Printf("Error getting container logs: %v. Retrying in 2 seconds...", err)
				time.Sleep(2 * time.Second)
				continue
			}

			scanner := bufio.NewScanner(resp.Body)
			for scanner.Scan() {
				line := scanner.Text()

				idx := strings.Index(line, "{")
				if idx == -1 {
					continue
				}
				jsonPart := line[idx:]

				var entry LogEntry
				if err := json.Unmarshal([]byte(jsonPart), &entry); err != nil {
					continue
				}

				if entry.Severity == "ERROR" {
					go TriggerAlert(entry)
				}

				mu.Lock()
				logBuffer = append(logBuffer, entry)

				if len(logBuffer) >= maxBufferSize {
					log.Printf("Buffer reached max capacity (%d), flushing...", maxBufferSize)
					err := FlushBuffer(mongoClient, logBuffer)
					if err != nil {
						log.Printf("Error flushing buffer: %v", err)
					} else {
						logBuffer = make([]LogEntry, 0)
					}
				}
				mu.Unlock()
			}
			resp.Body.Close()
			time.Sleep(2 * time.Second)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("Shutting down Observer Service...")

	mu.Lock()
	if len(logBuffer) > 0 {
		log.Printf("Final flush of %d logs to MongoDB...", len(logBuffer))
		FlushBuffer(mongoClient, logBuffer)
	}
	mu.Unlock()
}
