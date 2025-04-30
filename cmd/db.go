package cmd

import (
	"fmt"
	"os"
	"strings"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func ConnectDB() (*mongo.Database, error) {
	uri := os.Getenv("DB_URI")
	if uri == "" {
		return nil, fmt.Errorf("DB_URI environment variable is not set")
	}

	// Extract database name from URI
	parts := strings.Split(uri, "/")
	if len(parts) < 4 {
		return nil, fmt.Errorf("invalid DB_URI format")
	}
	dbName := parts[len(parts)-1]

	fmt.Println("uri", uri)
	fmt.Println("dbName", dbName)

	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	db := client.Database(dbName)

	return db, nil
}
