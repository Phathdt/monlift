package operations

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

// AggregateOperation represents an operation to perform an aggregation pipeline
type AggregateOperation struct {
	CollectionName string
	Pipeline       interface{}
}

// Execute performs an aggregation pipeline on a collection
func (a *AggregateOperation) Execute(ctx context.Context, db *mongo.Database) error {
	cursor, err := db.Collection(a.CollectionName).Aggregate(ctx, a.Pipeline)
	if err != nil {
		return fmt.Errorf("failed to execute aggregation: %w", err)
	}
	defer cursor.Close(ctx)

	// Just execute the aggregation, don't need to process results
	for cursor.Next(ctx) {
		// Do nothing, just consume the cursor
	}

	if err := cursor.Err(); err != nil {
		return fmt.Errorf("aggregation cursor error: %w", err)
	}

	return nil
}
