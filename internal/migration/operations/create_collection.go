package operations

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// TimeSeriesOptions represents the configuration for a time series collection
type TimeSeriesOptions struct {
	TimeField   string `validate:"required"`
	MetaField   string `validate:"omitempty"`
	Granularity string `validate:"omitempty,oneof=seconds minutes hours"`
}

// ValidatorOptions represents the configuration for collection validation
type ValidatorOptions struct {
	ValidationLevel  string `validate:"omitempty,oneof=off moderate strict"`
	ValidationAction string `validate:"omitempty,oneof=error warn"`
}

// CreateCollectionOperation represents an operation to create a collection
type CreateCollectionOperation struct {
	CollectionName string
	TimeSeriesOpts *TimeSeriesOptions
	ValidatorOpts  *ValidatorOptions
	Validator      interface{}
}

// Execute creates a collection with the specified options
func (c *CreateCollectionOperation) Execute(ctx context.Context, db *mongo.Database) error {
	opts := options.CreateCollection()

	// Apply timeseries options if provided
	if c.TimeSeriesOpts != nil {
		validate := validator.New()
		if err := validate.Struct(c.TimeSeriesOpts); err != nil {
			return fmt.Errorf("invalid timeseries options: %w", err)
		}

		timeseriesOpts := options.TimeSeries().SetTimeField(c.TimeSeriesOpts.TimeField)

		if c.TimeSeriesOpts.MetaField != "" {
			timeseriesOpts = timeseriesOpts.SetMetaField(c.TimeSeriesOpts.MetaField)
		}

		if c.TimeSeriesOpts.Granularity != "" {
			timeseriesOpts = timeseriesOpts.SetGranularity(c.TimeSeriesOpts.Granularity)
		}

		opts = opts.SetTimeSeriesOptions(timeseriesOpts)
	}

	// Apply validator options if provided
	if c.Validator != nil {
		opts = opts.SetValidator(c.Validator)
	}

	// Apply validation level and action if provided
	if c.ValidatorOpts != nil {
		validate := validator.New()
		if err := validate.Struct(c.ValidatorOpts); err != nil {
			return fmt.Errorf("invalid validation options: %w", err)
		}

		if c.ValidatorOpts.ValidationLevel != "" {
			opts = opts.SetValidationLevel(c.ValidatorOpts.ValidationLevel)
		}

		if c.ValidatorOpts.ValidationAction != "" {
			opts = opts.SetValidationAction(c.ValidatorOpts.ValidationAction)
		}
	}

	// Handle collection creation errors
	if err := db.CreateCollection(ctx, c.CollectionName, opts); err != nil {
		// Log the error for debugging
		fmt.Printf("Error creating collection %s: %v\n", c.CollectionName, err)

		// Special handling for collections that might already exist
		if strings.Contains(err.Error(), "NamespaceExists") {
			return nil
		}
		return fmt.Errorf("failed to create collection: %w", err)
	}

	return nil
}
