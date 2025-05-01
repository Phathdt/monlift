package operations

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

// DeleteOneOperation represents an operation to delete a single document
type DeleteOneOperation struct {
	CollectionName string
	Filter         interface{}
}

// Execute deletes a single document from a collection
func (d *DeleteOneOperation) Execute(ctx context.Context, db *mongo.Database) error {
	_, err := db.Collection(d.CollectionName).DeleteOne(ctx, d.Filter)
	if err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}
	return nil
}

// DeleteManyOperation represents an operation to delete multiple documents
type DeleteManyOperation struct {
	CollectionName string
	Filter         interface{}
}

// Execute deletes multiple documents from a collection
func (d *DeleteManyOperation) Execute(ctx context.Context, db *mongo.Database) error {
	_, err := db.Collection(d.CollectionName).DeleteMany(ctx, d.Filter)
	if err != nil {
		return fmt.Errorf("failed to delete documents: %w", err)
	}
	return nil
}
