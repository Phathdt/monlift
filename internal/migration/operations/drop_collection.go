package operations

import (
	"context"
	"fmt"
	"strings"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

// DropCollectionOperation represents an operation to drop a collection
type DropCollectionOperation struct {
	CollectionName string
}

// Execute drops a collection
func (d *DropCollectionOperation) Execute(ctx context.Context, db *mongo.Database) error {
	err := db.Collection(d.CollectionName).Drop(ctx)
	if err != nil && !strings.Contains(err.Error(), "NamespaceNotFound") {
		return fmt.Errorf("failed to drop collection: %w", err)
	}
	return nil
}
