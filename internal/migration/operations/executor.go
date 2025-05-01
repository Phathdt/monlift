package operations

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

// OperationExecutor runs operations on a MongoDB database
type OperationExecutor struct {
	db *mongo.Database
}

// NewOperationExecutor creates a new OperationExecutor
func NewOperationExecutor(db *mongo.Database) *OperationExecutor {
	return &OperationExecutor{
		db: db,
	}
}

// ExecuteOperation runs a single operation
func (e *OperationExecutor) ExecuteOperation(ctx context.Context, op Operation) error {
	return op.Execute(ctx, e.db)
}

// ExecuteOperations runs multiple operations in sequence
func (e *OperationExecutor) ExecuteOperations(ctx context.Context, ops []Operation) error {
	for i, op := range ops {
		if err := op.Execute(ctx, e.db); err != nil {
			return fmt.Errorf("failed to execute operation %d: %w", i+1, err)
		}
	}
	return nil
}

// ExecuteScript parses and executes a MongoDB script
func (e *OperationExecutor) ExecuteScript(ctx context.Context, script string) error {
	parser := NewOperationParser()
	ops, err := parser.ParseScript(script)
	if err != nil {
		return fmt.Errorf("failed to parse script: %w", err)
	}

	return e.ExecuteOperations(ctx, ops)
}
