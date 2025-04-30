package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func connectDB() (*mongo.Database, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

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

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	db := client.Database(dbName)

	// Create schema_versions collection if not exists
	err = db.CreateCollection(ctx, "schema_versions")
	if err != nil && !mongo.IsDuplicateKeyError(err) && err.Error() != "NamespaceExists" {
		return nil, fmt.Errorf("failed to create schema_versions collection: %w", err)
	}

	// Ensure schema_versions has initial document
	_, err = db.Collection("schema_versions").UpdateOne(
		ctx,
		bson.M{},
		bson.M{
			"$setOnInsert": bson.M{
				"version":   0,
				"appliedAt": time.Now(),
				"status":    "applied",
			},
		},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize schema_versions: %w", err)
	}

	return db, nil
}
