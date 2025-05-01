package operations

import (
	"context"
	"fmt"
	"strings"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

// DropIndexOperation represents an operation to drop an index
type DropIndexOperation struct {
	CollectionName string
	IndexName      string
}

// Execute drops an index on a collection
func (d *DropIndexOperation) Execute(ctx context.Context, db *mongo.Database) error {
	err := db.Collection(d.CollectionName).Indexes().DropOne(ctx, d.IndexName)
	if err != nil && !strings.Contains(err.Error(), "IndexNotFound") {
		return fmt.Errorf("failed to drop index: %w", err)
	}
	return nil
}
