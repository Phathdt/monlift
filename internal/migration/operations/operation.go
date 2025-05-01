package operations

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Operation defines the interface for all MongoDB operations
type Operation interface {
	Execute(ctx context.Context, db *mongo.Database) error
}
