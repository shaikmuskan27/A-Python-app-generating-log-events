package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type LogEntry struct {
	Timestamp   time.Time `bson:"timestamp" json:"timestamp"`
	ServiceName string    `bson:"service_name" json:"service_name"`
	Severity    string    `bson:"severity" json:"severity"`
	Message     string    `bson:"message" json:"message"`
	ContainerID string    `bson:"container_id" json:"container_id"`
}

type Stats struct {
	TotalLogs  int64 `json:"total_logs"`
	ErrorLogs  int64 `json:"error_logs"`
	InfoLogs   int64 `json:"info_logs"`
}

var client *mongo.Client
var collection *mongo.Collection

func connectDB() {
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}

	var err error
	for i := 0; i < 5; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		client, err = mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
		cancel()
		if err == nil {
			err = client.Ping(context.Background(), nil)
			if err == nil {
				collection = client.Database("logsdb").Collection("logs")
				log.Println("Connected to MongoDB!")
				return
			}
		}
		log.Printf("MongoDB not ready... retrying (%v)", err)
		time.Sleep(3 * time.Second)
	}
	log.Fatalf("Failed to connect to MongoDB: %v", err)
}

func enableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	(*w).Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func logsHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		return
	}

	w.Header().Set("Content-Type", "application/json")

	findOptions := options.Find()
	findOptions.SetSort(bson.D{{Key: "timestamp", Value: -1}})
	findOptions.SetLimit(100)

	// Optional filtering by severity and search
	severity := r.URL.Query().Get("severity")
	search := r.URL.Query().Get("search")
	filter := bson.M{}
	if severity != "" && severity != "ALL" {
		filter["severity"] = severity
	}
	if search != "" {
		filter["$or"] = []bson.M{
			{"message": bson.M{"$regex": search, "$options": "i"}},
			{"service_name": bson.M{"$regex": search, "$options": "i"}},
			{"container_id": bson.M{"$regex": search, "$options": "i"}},
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cur, err := collection.Find(ctx, filter, findOptions)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cur.Close(ctx)

	var results []LogEntry
	if err = cur.All(ctx, &results); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if results == nil {
		results = []LogEntry{}
	}
	json.NewEncoder(w).Encode(results)
}

func statsHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		return
	}

	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	total, _ := collection.CountDocuments(ctx, bson.M{})
	errors, _ := collection.CountDocuments(ctx, bson.M{"severity": "ERROR"})
	infos, _ := collection.CountDocuments(ctx, bson.M{"severity": "INFO"})

	stats := Stats{
		TotalLogs: total,
		ErrorLogs: errors,
		InfoLogs:  infos,
	}
	json.NewEncoder(w).Encode(stats)
}

func main() {
	log.Println("Starting API Service...")
	connectDB()
	defer client.Disconnect(context.Background())

	http.HandleFunc("/api/logs", logsHandler)
	http.HandleFunc("/api/stats", statsHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	
	log.Printf("Listening on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
