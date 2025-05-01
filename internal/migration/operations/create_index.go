package operations

import (
	"context"
	"fmt"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// CreateIndexOperation represents an operation to create an index
type CreateIndexOperation struct {
	CollectionName     string
	Keys               bson.D
	Unique             bool
	Sparse             bool
	Hidden             bool
	Name               string
	DefaultLanguage    string
	ExpireAfterSeconds int32
	Weights            bson.D
	WildcardProjection bson.D
	PartialFilter      any
	BucketSize         int32
}

// Execute creates an index on a collection
func (c *CreateIndexOperation) Execute(ctx context.Context, db *mongo.Database) error {
	indexModel := mongo.IndexModel{
		Keys: c.Keys,
	}

	opts := options.Index()

	if c.Unique {
		opts = opts.SetUnique(c.Unique)
	}

	if c.Sparse {
		opts = opts.SetSparse(c.Sparse)
	}

	if c.Hidden {
		opts = opts.SetHidden(c.Hidden)
	}

	if c.Name != "" {
		opts = opts.SetName(c.Name)
	}

	if c.DefaultLanguage != "" {
		opts = opts.SetDefaultLanguage(c.DefaultLanguage)
	}

	if c.ExpireAfterSeconds != 0 {
		opts = opts.SetExpireAfterSeconds(c.ExpireAfterSeconds)
	}

	if c.Weights != nil {
		opts = opts.SetWeights(c.Weights)
	}

	if c.WildcardProjection != nil {
		opts = opts.SetWildcardProjection(c.WildcardProjection)
	}

	if c.PartialFilter != nil {
		opts = opts.SetPartialFilterExpression(c.PartialFilter)
	}

	if c.BucketSize != 0 {
		opts = opts.SetBucketSize(c.BucketSize)
	}

	indexModel.Options = opts

	_, err := db.Collection(c.CollectionName).Indexes().CreateOne(ctx, indexModel)
	if err != nil {
		if strings.Contains(err.Error(), "IndexOptionsConflict") {
			// Index already exists with different options, which might be fine in some cases
			return nil
		}
		return fmt.Errorf("failed to create index: %w", err)
	}

	return nil
}
