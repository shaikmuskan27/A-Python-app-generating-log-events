package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
)

func TriggerAlert(entry LogEntry) {
	webhookURL := os.Getenv("WEBHOOK_URL")
	if webhookURL == "" {
		// Mock endpoint for demonstration
		webhookURL = "http://httpbin.org/post"
	}

	payload, err := json.Marshal(map[string]interface{}{
		"text":     "Alert: Error detected in " + entry.ServiceName,
		"severity": entry.Severity,
		"message":  entry.Message,
		"time":     entry.Timestamp,
	})
	if err != nil {
		log.Printf("Error marshalling alert payload: %v", err)
		return
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		log.Printf("Error sending alert: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Printf("Alert triggered successfully for log: %s", entry.Message)
	} else {
		log.Printf("Failed to trigger alert. Status code: %d", resp.StatusCode)
	}
}
