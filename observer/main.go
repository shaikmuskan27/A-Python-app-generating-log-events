package main

import (
	"bufio"
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

func main() {
	log.Println("Starting Observer Service...")

	// Initialize MongoDB Client
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}
	
	mongoClient, err := ConnectMongo(mongoURI)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer mongoClient.Disconnect(context.Background())

	// Initialize Docker Client
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("Failed to create Docker client: %v", err)
	}
	defer dockerClient.Close()

	targetContainerName := os.Getenv("TARGET_CONTAINER")
	if targetContainerName == "" {
		targetContainerName = "target-app"
	}

	logBuffer := make([]LogEntry, 0)
	var mu sync.Mutex
	flushInterval := 5 * time.Second
	maxBufferSize := 1000

	// Start a ticker to flush buffer periodically
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
					logBuffer = make([]LogEntry, 0) // reset buffer
				}
			}
			mu.Unlock()
		}
	}()

	// Wait for container to be available
	log.Printf("Waiting for container %s to be available...", targetContainerName)
	var targetContainerID string
	for {
		containers, err := dockerClient.ContainerList(context.Background(), container.ListOptions{})
		if err != nil {
			log.Printf("Error listing containers: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}
		
		found := false
		for _, ctr := range containers {
			for _, name := range ctr.Names {
				// Docker prepends a slash to the name
				if name == "/"+targetContainerName {
					targetContainerID = ctr.ID
					found = true
					break
				}
			}
		}
		if found {
			break
		}
		time.Sleep(2 * time.Second)
	}

	log.Printf("Found container %s (ID: %s). Tailing logs...", targetContainerName, targetContainerID[:12])

	// Tail logs
	go func() {
		// Attempt to read logs
		for {
			options := container.LogsOptions{
				ShowStdout: true,
				ShowStderr: true,
				Follow:     true,
				Tail:       "0", // start from now
			}
			out, err := dockerClient.ContainerLogs(context.Background(), targetContainerID, options)
			if err != nil {
				log.Printf("Error getting container logs: %v. Retrying in 2 seconds...", err)
				time.Sleep(2 * time.Second)
				continue
			}

			scanner := bufio.NewScanner(out)
			for scanner.Scan() {
				line := scanner.Text()
				
				idx := strings.Index(line, "{")
				if idx == -1 {
					continue
				}
				jsonPart := line[idx:]

				var entry LogEntry
				if err := json.Unmarshal([]byte(jsonPart), &entry); err != nil {
					continue // not a valid log entry or parse error
				}

				// If it's an ERROR, trigger an alert
				if entry.Severity == "ERROR" {
					go TriggerAlert(entry)
				}

				mu.Lock()
				logBuffer = append(logBuffer, entry)
				
				// Force flush if max size reached
				if len(logBuffer) >= maxBufferSize {
					log.Printf("Buffer reached max capacity (%d), flushing...", maxBufferSize)
					err := FlushBuffer(mongoClient, logBuffer)
					if err != nil {
						log.Printf("Error flushing buffer: %v", err)
					} else {
						logBuffer = make([]LogEntry, 0) // reset buffer
					}
				}
				mu.Unlock()
			}
			out.Close()
			time.Sleep(2 * time.Second) // loop and reconnect if stream dies
		}
	}()

	// Channel for graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("Shutting down Observer Service...")
	
	// Final flush
	mu.Lock()
	if len(logBuffer) > 0 {
		log.Printf("Final flush of %d logs to MongoDB...", len(logBuffer))
		FlushBuffer(mongoClient, logBuffer)
	}
	mu.Unlock()
}
