package migration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/phathdt/monlift/internal/migration/operations"
)

type Migration struct {
	Version  string
	UpPath   string
	DownPath string
	db       *mongo.Database
}

func NewMigration(version, upPath, downPath string, db *mongo.Database) *Migration {
	return &Migration{
		Version:  version,
		UpPath:   upPath,
		DownPath: downPath,
		db:       db,
	}
}

func initVersionsCollection(db *mongo.Database) error {
	ctx := context.Background()

	// Create versions collection if not exists
	err := db.CreateCollection(ctx, "versions")
	if err != nil && !strings.Contains(err.Error(), "NamespaceExists") {
		return fmt.Errorf("failed to create versions collection: %w", err)
	}

	// Create index on version field
	_, err = db.Collection("versions").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "version", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil && !strings.Contains(err.Error(), "IndexOptionsConflict") {
		return fmt.Errorf("failed to create version index: %w", err)
	}

	// Ensure versions has initial document
	_, err = db.Collection("versions").UpdateOne(
		ctx,
		bson.M{"version": "0"},
		bson.M{
			"$setOnInsert": bson.M{
				"version":   "0",
				"appliedAt": time.Now(),
			},
		},
		options.UpdateOne().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("failed to initialize versions: %w", err)
	}

	return nil
}

func (m *Migration) Up() error {
	if err := initVersionsCollection(m.db); err != nil {
		return fmt.Errorf("failed to initialize versions collection: %w", err)
	}

	content, err := os.ReadFile(m.UpPath)
	if err != nil {
		return fmt.Errorf("failed to read up migration: %w", err)
	}

	// Execute the migration script using the operations executor
	executor := operations.NewOperationExecutor(m.db)
	ctx := context.Background()
	if err := executor.ExecuteScript(ctx, string(content)); err != nil {
		return fmt.Errorf("migration up failed: %w", err)
	}

	// Record migration
	_, err = m.db.Collection("versions").UpdateOne(
		ctx,
		bson.M{"version": m.Version},
		bson.M{
			"$set": bson.M{
				"version":   m.Version,
				"appliedAt": time.Now(),
			},
		},
		options.UpdateOne().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	return nil
}

func (m *Migration) Down() error {
	if err := initVersionsCollection(m.db); err != nil {
		return fmt.Errorf("failed to initialize versions collection: %w", err)
	}

	content, err := os.ReadFile(m.DownPath)
	if err != nil {
		return fmt.Errorf("failed to read down migration: %w", err)
	}

	// Execute the rollback script using the operations executor
	executor := operations.NewOperationExecutor(m.db)
	ctx := context.Background()
	if err := executor.ExecuteScript(ctx, string(content)); err != nil {
		return fmt.Errorf("migration down failed: %w", err)
	}

	// Remove migration record
	_, err = m.db.Collection("versions").DeleteOne(ctx, bson.M{"version": m.Version})
	if err != nil {
		return fmt.Errorf("failed to remove migration record: %w", err)
	}

	return nil
}

func GetMigrations(dir string, db *mongo.Database) ([]*Migration, error) {
	upFiles, err := filepath.Glob(filepath.Join(dir, "*.up.js"))
	if err != nil {
		return nil, fmt.Errorf("failed to read migrations directory: %w", err)
	}

	var migrations []*Migration
	for _, upFile := range upFiles {
		version := strings.TrimSuffix(filepath.Base(upFile), ".up.js")
		downFile := filepath.Join(dir, version+".down.js")

		if _, err := os.Stat(downFile); err != nil {
			return nil, fmt.Errorf("missing down migration for version %s", version)
		}

		migrations = append(migrations, NewMigration(version, upFile, downFile, db))
	}

	return migrations, nil
}

func GetExecutedMigrations(db *mongo.Database) (map[string]bool, error) {
	if err := initVersionsCollection(db); err != nil {
		return nil, fmt.Errorf("failed to initialize versions collection: %w", err)
	}

	ctx := context.Background()
	cursor, err := db.Collection("versions").Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{"version": 1, "_id": 0}))
	if err != nil {
		return nil, fmt.Errorf("failed to get executed migrations: %w", err)
	}
	defer cursor.Close(ctx)

	executed := make(map[string]bool)
	for cursor.Next(ctx) {
		var result struct {
			Version string `bson:"version"`
		}
		if err := cursor.Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode version: %w", err)
		}
		executed[result.Version] = true
	}

	return executed, nil
}

type MigrationStatus struct {
	Version   string
	AppliedAt time.Time
	Status    string
}

func GetMigrationStatus(db *mongo.Database, dir string) ([]MigrationStatus, error) {
	if err := initVersionsCollection(db); err != nil {
		return nil, fmt.Errorf("failed to initialize versions collection: %w", err)
	}

	// Get all migrations from directory
	migrations, err := GetMigrations(dir, db)
	if err != nil {
		return nil, fmt.Errorf("failed to get migrations: %w", err)
	}

	// Get applied timestamps
	ctx := context.Background()
	cursor, err := db.Collection("versions").Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{"version": 1, "appliedAt": 1, "_id": 0}))
	if err != nil {
		return nil, fmt.Errorf("failed to get migration timestamps: %w", err)
	}
	defer cursor.Close(ctx)

	appliedAt := make(map[string]time.Time)
	for cursor.Next(ctx) {
		var result struct {
			Version   string    `bson:"version"`
			AppliedAt time.Time `bson:"appliedAt"`
		}
		if err := cursor.Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode migration timestamp: %w", err)
		}
		appliedAt[result.Version] = result.AppliedAt
	}

	// Build status list
	status := make([]MigrationStatus, 0, len(migrations))
	for _, m := range migrations {
		s := MigrationStatus{
			Version: m.Version,
		}
		if at, ok := appliedAt[m.Version]; ok {
			s.AppliedAt = at
			s.Status = "Applied"
		} else {
			s.Status = "Pending"
		}
		status = append(status, s)
	}

	return status, nil
}
