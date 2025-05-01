package operations

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

// UpdateOneOperation represents an operation to update a single document
type UpdateOneOperation struct {
	CollectionName string
	Filter         any
	Update         any
}

// Execute updates a single document in a collection
func (u *UpdateOneOperation) Execute(ctx context.Context, db *mongo.Database) error {
	_, err := db.Collection(u.CollectionName).UpdateOne(ctx, u.Filter, u.Update)
	if err != nil {
		return fmt.Errorf("failed to update document: %w", err)
	}
	return nil
}

// UpdateManyOperation represents an operation to update multiple documents
type UpdateManyOperation struct {
	CollectionName string
	Filter         any
	Update         any
}

// Execute updates multiple documents in a collection
func (u *UpdateManyOperation) Execute(ctx context.Context, db *mongo.Database) error {
	_, err := db.Collection(u.CollectionName).UpdateMany(ctx, u.Filter, u.Update)
	if err != nil {
		return fmt.Errorf("failed to update documents: %w", err)
	}
	return nil
}
