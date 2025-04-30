package migration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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
		options.Update().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("failed to initialize versions: %w", err)
	}

	return nil
}

func executeMongoScript(db *mongo.Database, script string) error {
	ctx := context.Background()

	// Split the script into individual commands
	commands := strings.Split(script, ";")

	for _, cmd := range commands {
		cmd = strings.TrimSpace(cmd)
		if cmd == "" {
			continue
		}

		// Parse the command
		if strings.HasPrefix(cmd, "db.") {
			// Handle collection operations
			parts := strings.SplitN(cmd, ".", 2)
			if len(parts) < 2 {
				return fmt.Errorf("invalid command format: %s", cmd)
			}

			// Extract operation and arguments
			operationWithArgs := parts[1]
			operation := strings.Split(operationWithArgs, "(")[0]

			// Extract arguments from command
			argsStart := strings.Index(cmd, "(")
			argsEnd := strings.LastIndex(cmd, ")")
			if argsStart == -1 || argsEnd == -1 {
				return fmt.Errorf("invalid command format: %s", cmd)
			}
			argsStr := strings.TrimSpace(cmd[argsStart+1 : argsEnd])

			switch operation {
			case "createCollection":
				// Get collection name from arguments
				collectionName := strings.Trim(argsStr, `'"`)
				if collectionName == "" {
					return fmt.Errorf("collection name cannot be empty")
				}
				err := db.CreateCollection(ctx, collectionName)
				if err != nil && !strings.Contains(err.Error(), "NamespaceExists") {
					return fmt.Errorf("failed to create collection: %w", err)
				}
			case "insertOne":
				// Parse the document from the command
				doc := bson.M{}
				if err := bson.UnmarshalExtJSON([]byte(argsStr), true, &doc); err != nil {
					return fmt.Errorf("failed to parse document: %w", err)
				}
				_, err := db.Collection(parts[0]).InsertOne(ctx, doc)
				if err != nil {
					return fmt.Errorf("failed to insert document: %w", err)
				}
			case "updateMany":
				// Parse the filter and update from the command
				args := strings.Split(argsStr, ",")
				if len(args) < 2 {
					return fmt.Errorf("invalid updateMany command format: %s", cmd)
				}
				filter := bson.M{}
				update := bson.M{}
				if err := bson.UnmarshalExtJSON([]byte(args[0]), true, &filter); err != nil {
					return fmt.Errorf("failed to parse filter: %w", err)
				}
				if err := bson.UnmarshalExtJSON([]byte(args[1]), true, &update); err != nil {
					return fmt.Errorf("failed to parse update: %w", err)
				}
				_, err := db.Collection(parts[0]).UpdateMany(ctx, filter, update)
				if err != nil {
					return fmt.Errorf("failed to update documents: %w", err)
				}
			case "createIndex":
				// Parse the index specification from the command
				args := strings.Split(argsStr, ",")
				if len(args) < 2 {
					return fmt.Errorf("invalid createIndex command format: %s", cmd)
				}
				keys := bson.M{}
				options := options.Index()

				if err := bson.UnmarshalExtJSON([]byte(args[0]), true, &keys); err != nil {
					return fmt.Errorf("failed to parse index keys: %w", err)
				}

				if len(args) > 1 {
					opts := bson.M{}
					if err := bson.UnmarshalExtJSON([]byte(args[1]), true, &opts); err != nil {
						return fmt.Errorf("failed to parse index options: %w", err)
					}

					if unique, ok := opts["unique"].(bool); ok {
						options.SetUnique(unique)
					}
					if sparse, ok := opts["sparse"].(bool); ok {
						options.SetSparse(sparse)
					}
					if name, ok := opts["name"].(string); ok {
						options.SetName(name)
					}
				}

				_, err := db.Collection(parts[0]).Indexes().CreateOne(ctx, mongo.IndexModel{
					Keys:    keys,
					Options: options,
				})
				if err != nil && !strings.Contains(err.Error(), "IndexOptionsConflict") {
					return fmt.Errorf("failed to create index: %w", err)
				}
			case "dropIndex":
				// Parse the index name from the command
				indexName := strings.Trim(argsStr, `'"`)
				_, err := db.Collection(parts[0]).Indexes().DropOne(ctx, indexName)
				if err != nil && !strings.Contains(err.Error(), "IndexNotFound") {
					return fmt.Errorf("failed to drop index: %w", err)
				}
			case "deleteMany":
				// Parse the filter from the command
				filter := bson.M{}
				if err := bson.UnmarshalExtJSON([]byte(argsStr), true, &filter); err != nil {
					return fmt.Errorf("failed to parse filter: %w", err)
				}
				_, err := db.Collection(parts[0]).DeleteMany(ctx, filter)
				if err != nil {
					return fmt.Errorf("failed to delete documents: %w", err)
				}
			case "dropCollection":
				err := db.Collection(parts[0]).Drop(ctx)
				if err != nil && !strings.Contains(err.Error(), "NamespaceNotFound") {
					return fmt.Errorf("failed to drop collection: %w", err)
				}
			case "insertMany":
				// Parse the documents from the command
				docs := []interface{}{}
				if err := bson.UnmarshalExtJSON([]byte(argsStr), true, &docs); err != nil {
					return fmt.Errorf("failed to parse documents: %w", err)
				}
				_, err := db.Collection(parts[0]).InsertMany(ctx, docs)
				if err != nil {
					return fmt.Errorf("failed to insert documents: %w", err)
				}
			case "updateOne":
				// Parse the filter and update from the command
				args := strings.Split(argsStr, ",")
				if len(args) < 2 {
					return fmt.Errorf("invalid updateOne command format: %s", cmd)
				}
				filter := bson.M{}
				update := bson.M{}
				if err := bson.UnmarshalExtJSON([]byte(args[0]), true, &filter); err != nil {
					return fmt.Errorf("failed to parse filter: %w", err)
				}
				if err := bson.UnmarshalExtJSON([]byte(args[1]), true, &update); err != nil {
					return fmt.Errorf("failed to parse update: %w", err)
				}
				_, err := db.Collection(parts[0]).DeleteOne(ctx, filter)
				if err != nil {
					return fmt.Errorf("failed to delete document: %w", err)
				}
			case "deleteOne":
				// Parse the filter from the command
				filter := bson.M{}
				if err := bson.UnmarshalExtJSON([]byte(argsStr), true, &filter); err != nil {
					return fmt.Errorf("failed to parse filter: %w", err)
				}
				_, err := db.Collection(parts[0]).DeleteOne(ctx, filter)
				if err != nil {
					return fmt.Errorf("failed to delete document: %w", err)
				}
			case "aggregate":
				// Parse the pipeline from the command
				pipeline := []bson.M{}
				if err := bson.UnmarshalExtJSON([]byte(argsStr), true, &pipeline); err != nil {
					return fmt.Errorf("failed to parse pipeline: %w", err)
				}
				cursor, err := db.Collection(parts[0]).Aggregate(ctx, pipeline)
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
			default:
				return fmt.Errorf("unsupported operation: %s", operation)
			}
		} else {
			return fmt.Errorf("unsupported command format: %s", cmd)
		}
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

	// Execute the migration script
	if err := executeMongoScript(m.db, string(content)); err != nil {
		return fmt.Errorf("migration up failed: %w", err)
	}

	// Record migration
	ctx := context.Background()
	_, err = m.db.Collection("versions").UpdateOne(
		ctx,
		bson.M{"version": m.Version},
		bson.M{
			"$set": bson.M{
				"version":   m.Version,
				"appliedAt": time.Now(),
			},
		},
		options.Update().SetUpsert(true),
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

	// Execute the rollback script
	if err := executeMongoScript(m.db, string(content)); err != nil {
		return fmt.Errorf("migration down failed: %w", err)
	}

	// Remove migration record
	ctx := context.Background()
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
