package main

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// LogEntry represents the schema for MongoDB
type LogEntry struct {
	Timestamp   time.Time `bson:"timestamp" json:"timestamp"`
	ServiceName string    `bson:"service_name" json:"service_name"`
	Severity    string    `bson:"severity" json:"severity"`
	Message     string    `bson:"message" json:"message"`
	ContainerID string    `bson:"container_id" json:"container_id"`
}

// ConnectMongo initializes the MongoDB client and creates necessary indexes
func ConnectMongo(uri string) (*mongo.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}

	// Ping the database
	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, err
	}

	// Create Indexes
	db := client.Database("logsdb")
	coll := db.Collection("logs")
	
	_, err = coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "service_name", Value: 1}, {Key: "timestamp", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "severity", Value: 1}, {Key: "timestamp", Value: -1}},
		},
	})
	if err != nil {
		return nil, err
	}

	return client, nil
}

// FlushBuffer bulk inserts the provided logs into MongoDB
func FlushBuffer(client *mongo.Client, entries []LogEntry) error {
	collection := client.Database("logsdb").Collection("logs")
	
	var docs []interface{}
	for _, entry := range entries {
		docs = append(docs, entry)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := collection.InsertMany(ctx, docs)
	return err
}
