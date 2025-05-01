package operations

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

// InsertOneOperation represents an operation to insert a single document
type InsertOneOperation struct {
	CollectionName string
	Document       any
}

// Execute inserts a single document into a collection
func (i *InsertOneOperation) Execute(ctx context.Context, db *mongo.Database) error {
	_, err := db.Collection(i.CollectionName).InsertOne(ctx, i.Document)
	if err != nil {
		return fmt.Errorf("failed to insert document: %w", err)
	}
	return nil
}

// InsertManyOperation represents an operation to insert multiple documents
type InsertManyOperation struct {
	CollectionName string
	Documents      []any
}

// Execute inserts multiple documents into a collection
func (i *InsertManyOperation) Execute(ctx context.Context, db *mongo.Database) error {
	_, err := db.Collection(i.CollectionName).InsertMany(ctx, i.Documents)
	if err != nil {
		return fmt.Errorf("failed to insert documents: %w", err)
	}
	return nil
}
