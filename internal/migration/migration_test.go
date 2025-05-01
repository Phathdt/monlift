package migration

import (
	"context"
	"strings"
	"testing"

	"github.com/phathdt/monlift/internal/migration/operations"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func setupTestDB(t *testing.T) *mongo.Database {
	ctx := context.Background()

	// Create and start MongoDB container
	container, err := mongodb.Run(ctx, "mongo:8.0.8")
	if err != nil {
		t.Fatalf("Failed to start MongoDB container: %v", err)
	}

	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	})

	// Get connection URI
	uri, err := container.ConnectionString(ctx)
	if err != nil {
		if err := testcontainers.TerminateContainer(container); err != nil {
			t.Logf("Failed to terminate container after connection string error: %v", err)
		}
		t.Fatalf("Failed to get MongoDB connection string: %v", err)
	}

	// Connect to MongoDB
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		if err := testcontainers.TerminateContainer(container); err != nil {
			t.Logf("Failed to terminate container after client connection error: %v", err)
		}
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}

	// Ping to verify connection
	if err := client.Ping(ctx, nil); err != nil {
		if err := client.Disconnect(ctx); err != nil {
			t.Logf("Failed to disconnect MongoDB client: %v", err)
		}
		if err := testcontainers.TerminateContainer(container); err != nil {
			t.Logf("Failed to terminate container after ping error: %v", err)
		}
		t.Fatalf("Failed to ping MongoDB: %v", err)
	}

	db := client.Database("test_migration")
	t.Cleanup(func() {
		ctx := context.Background()
		// Clean up collections with proper ordering
		collections, err := db.ListCollectionNames(ctx, bson.M{})
		if err != nil {
			t.Logf("Failed to list collections during cleanup: %v", err)
		} else {
			// First drop non-system collections (including timeseries)
			for _, collection := range collections {
				if !strings.HasPrefix(collection, "system.") {
					if err := db.Collection(collection).Drop(ctx); err != nil {
						t.Logf("Failed to drop collection %s during cleanup: %v", collection, err)
					}
				}
			}

			// Then try system collections
			for _, collection := range collections {
				if strings.HasPrefix(collection, "system.") {
					if err := db.Collection(collection).Drop(ctx); err != nil {
						// Only log non-expected errors
						if !strings.Contains(err.Error(), "cannot drop collection") {
							t.Logf("Failed to drop system collection %s during cleanup: %v", collection, err)
						}
					}
				}
			}
		}

		// Finally drop the entire database
		if err := db.Drop(ctx); err != nil {
			t.Logf("Failed to drop database during cleanup: %v", err)
		}

		if err := client.Disconnect(ctx); err != nil {
			t.Logf("Failed to disconnect MongoDB client: %v", err)
		}
	})

	return db
}

func TestExecuteMongoScript(t *testing.T) {
	db := setupTestDB(t)

	// Cleanup function to reset the database after each test
	cleanup := func() {
		ctx := context.Background()
		collections, err := db.ListCollectionNames(ctx, bson.M{})
		if err != nil {
			t.Logf("Failed to list collections: %v", err)
			return
		}

		// First, drop all non-system collections (including timeseries collections)
		for _, collection := range collections {
			// Skip system collections for now
			if strings.HasPrefix(collection, "system.") {
				continue
			}

			if err := db.Collection(collection).Drop(ctx); err != nil {
				t.Logf("Failed to drop collection %s: %v", collection, err)
			}
		}

		// Now attempt to drop system collections
		for _, collection := range collections {
			if strings.HasPrefix(collection, "system.") {
				if err := db.Collection(collection).Drop(ctx); err != nil {
					// This error is expected for system.views after timeseries collections have been dropped,
					// so we don't need to log it as it creates noise
					if !strings.Contains(err.Error(), "cannot drop collection") {
						t.Logf("Failed to drop system collection %s: %v", collection, err)
					}
				}
			}
		}
	}

	tests := []struct {
		name    string
		script  string
		wantErr bool
		verify  func(t *testing.T, db *mongo.Database)
	}{
		{
			name:   "create collection",
			script: `db.createCollection("test_collection")`,
			verify: func(t *testing.T, db *mongo.Database) {
				names, err := db.ListCollectionNames(context.Background(), bson.M{})
				if err != nil {
					t.Errorf("Failed to list collections: %v", err)
				}
				found := false
				for _, name := range names {
					if name == "test_collection" {
						found = true
						break
					}
				}
				if !found {
					t.Error("Collection was not created")
				}
			},
		},
		{
			name:   "insert one document",
			script: `db.test_collection.insertOne({"name": "test", "value": 123})`,
			verify: func(t *testing.T, db *mongo.Database) {
				var result bson.M
				err := db.Collection("test_collection").FindOne(context.Background(), bson.M{"name": "test"}).Decode(&result)
				if err != nil {
					t.Errorf("Failed to find document: %v", err)
				}
				if result["value"] != int32(123) {
					t.Errorf("Unexpected value: got %v, want 123", result["value"])
				}
			},
		},
		{
			name:   "update many documents",
			script: `db.test_collection.insertOne({"name": "test", "value": 123}); db.test_collection.updateMany({"name": "test"}, {"$set": {"value": 456}})`,
			verify: func(t *testing.T, db *mongo.Database) {
				var result bson.M
				err := db.Collection("test_collection").FindOne(context.Background(), bson.M{"name": "test"}).Decode(&result)
				if err != nil {
					t.Errorf("Failed to find document: %v", err)
				}
				if result["value"] != int32(456) {
					t.Errorf("Unexpected value: got %v, want 456", result["value"])
				}
			},
		},
		{
			name:   "create index",
			script: `db.test_collection.createIndex({"name": 1}, {"unique": true})`,
			verify: func(t *testing.T, db *mongo.Database) {
				cursor, err := db.Collection("test_collection").Indexes().List(context.Background())
				if err != nil {
					t.Errorf("Failed to list indexes: %v", err)
				}
				var indexes []bson.M
				if err = cursor.All(context.Background(), &indexes); err != nil {
					t.Errorf("Failed to decode indexes: %v", err)
				}
				found := false
				for _, idx := range indexes {
					if name, ok := idx["name"].(string); ok && name == "name_1" {
						found = true
						if unique, ok := idx["unique"].(bool); !ok || !unique {
							t.Error("Index should be unique")
						}
						break
					}
				}
				if !found {
					t.Error("Index was not created")
				}
			},
		},
		{
			name:   "create compound index",
			script: `db.test_collection.createIndex({"name": 1, "age": -1}, {"name": "name_age_idx"})`,
			verify: func(t *testing.T, db *mongo.Database) {
				cursor, err := db.Collection("test_collection").Indexes().List(context.Background())
				if err != nil {
					t.Errorf("Failed to list indexes: %v", err)
				}
				var indexes []bson.M
				if err = cursor.All(context.Background(), &indexes); err != nil {
					t.Errorf("Failed to decode indexes: %v", err)
				}
				found := false
				for _, idx := range indexes {
					if name, ok := idx["name"].(string); ok && name == "name_age_idx" {
						found = true
						keys := idx["key"].(bson.D)
						nameFound := false
						ageFound := false
						for _, elem := range keys {
							if elem.Key == "name" && elem.Value == int32(1) {
								nameFound = true
							} else if elem.Key == "age" && elem.Value == int32(-1) {
								ageFound = true
							}
						}
						if !nameFound || !ageFound {
							t.Errorf("Index keys are incorrect: got %v", keys)
						}
						break
					}
				}
				if !found {
					t.Error("Compound index was not created")
				}
			},
		},
		{
			name:   "create text index",
			script: `db.test_collection.createIndex({"description": "text"}, {"default_language": "english", "weights": {"description": 10}})`,
			verify: func(t *testing.T, db *mongo.Database) {
				cursor, err := db.Collection("test_collection").Indexes().List(context.Background())
				if err != nil {
					t.Errorf("Failed to list indexes: %v", err)
				}
				var indexes []bson.M
				if err = cursor.All(context.Background(), &indexes); err != nil {
					t.Errorf("Failed to decode indexes: %v", err)
				}
				found := false
				for _, idx := range indexes {
					if name, ok := idx["name"].(string); ok && name == "description_text" {
						found = true
						if weights, ok := idx["weights"].(bson.D); !ok {
							t.Error("Text index weights are missing")
						} else {
							weightFound := false
							for _, elem := range weights {
								if elem.Key == "description" {
									if num, ok := elem.Value.(int32); ok && num == 10 {
										weightFound = true
										break
									} else if num, ok := elem.Value.(float64); ok && num == 10 {
										weightFound = true
										break
									}
								}
							}
							if !weightFound {
								t.Errorf("Text index weights are incorrect: got %v", weights)
							}
						}
						if defaultLanguage, ok := idx["default_language"].(string); !ok || defaultLanguage != "english" {
							t.Errorf("Text index default language is incorrect: got %v", defaultLanguage)
						}
						break
					}
				}
				if !found {
					t.Error("Text index was not created")
				}
			},
		},
		{
			name:   "create TTL index",
			script: `db.test_collection.createIndex({"createdAt": 1}, {"expireAfterSeconds": 3600})`,
			verify: func(t *testing.T, db *mongo.Database) {
				cursor, err := db.Collection("test_collection").Indexes().List(context.Background())
				if err != nil {
					t.Errorf("Failed to list indexes: %v", err)
				}
				var indexes []bson.M
				if err = cursor.All(context.Background(), &indexes); err != nil {
					t.Errorf("Failed to decode indexes: %v", err)
				}
				found := false
				for _, idx := range indexes {
					if name, ok := idx["name"].(string); ok && name == "createdAt_1" {
						found = true
						if expireAfterSeconds, ok := idx["expireAfterSeconds"].(int32); !ok || expireAfterSeconds != 3600 {
							t.Error("TTL index expireAfterSeconds is incorrect")
						}
						break
					}
				}
				if !found {
					t.Error("TTL index was not created")
				}
			},
		},
		{
			name:   "create sparse index",
			script: `db.test_collection.createIndex({"optionalField": 1}, {"sparse": true})`,
			verify: func(t *testing.T, db *mongo.Database) {
				cursor, err := db.Collection("test_collection").Indexes().List(context.Background())
				if err != nil {
					t.Errorf("Failed to list indexes: %v", err)
				}
				var indexes []bson.M
				if err = cursor.All(context.Background(), &indexes); err != nil {
					t.Errorf("Failed to decode indexes: %v", err)
				}
				found := false
				for _, idx := range indexes {
					if name, ok := idx["name"].(string); ok && name == "optionalField_1" {
						found = true
						if sparse, ok := idx["sparse"].(bool); !ok || !sparse {
							t.Error("Index should be sparse")
						}
						break
					}
				}
				if !found {
					t.Error("Sparse index was not created")
				}
			},
		},
		{
			name:   "create partial index",
			script: `db.test_collection.createIndex({"status": 1}, {"partialFilterExpression": {"status": {"$exists": true}}})`,
			verify: func(t *testing.T, db *mongo.Database) {
				cursor, err := db.Collection("test_collection").Indexes().List(context.Background())
				if err != nil {
					t.Errorf("Failed to list indexes: %v", err)
				}
				var indexes []bson.M
				if err = cursor.All(context.Background(), &indexes); err != nil {
					t.Errorf("Failed to decode indexes: %v", err)
				}
				found := false
				for _, idx := range indexes {
					if name, ok := idx["name"].(string); ok && name == "status_1" {
						found = true

						// Check if partialFilterExpression exists
						if idx["partialFilterExpression"] == nil {
							t.Error("Partial index filter expression is missing")
							continue
						}

						// Function to check filter expression regardless of type (bson.M or bson.D)
						checkFilter := func(filter any) bool {
							switch typedFilter := filter.(type) {
							case bson.M:
								status, ok := typedFilter["status"].(bson.M)
								if !ok {
									return false
								}
								exists, ok := status["$exists"].(bool)
								return ok && exists
							case bson.D:
								for _, elem := range typedFilter {
									if elem.Key == "status" {
										if statusObj, ok := elem.Value.(bson.D); ok {
											for _, statusElem := range statusObj {
												if statusElem.Key == "$exists" {
													if existsVal, ok := statusElem.Value.(bool); ok && existsVal {
														return true
													}
												}
											}
										}
									}
								}
								return false
							default:
								return false
							}
						}

						if !checkFilter(idx["partialFilterExpression"]) {
							t.Error("Partial index filter expression is incorrect")
						}
					}
				}
				if !found {
					t.Error("Partial index was not created")
				}
			},
		},
		{
			name:   "create hidden index",
			script: `db.test_collection.createIndex({"hiddenField": 1}, {"hidden": true})`,
			verify: func(t *testing.T, db *mongo.Database) {
				cursor, err := db.Collection("test_collection").Indexes().List(context.Background())
				if err != nil {
					t.Errorf("Failed to list indexes: %v", err)
				}
				var indexes []bson.M
				if err = cursor.All(context.Background(), &indexes); err != nil {
					t.Errorf("Failed to decode indexes: %v", err)
				}
				found := false
				for _, idx := range indexes {
					if name, ok := idx["name"].(string); ok && name == "hiddenField_1" {
						found = true
						if hidden, ok := idx["hidden"].(bool); !ok || !hidden {
							t.Error("Index should be hidden")
						}
						break
					}
				}
				if !found {
					t.Error("Hidden index was not created")
				}
			},
		},
		{
			name:   "drop index",
			script: `db.test_collection.createIndex({"name": 1}, {"unique": true}); db.test_collection.dropIndex("name_1")`,
			verify: func(t *testing.T, db *mongo.Database) {
				cursor, err := db.Collection("test_collection").Indexes().List(context.Background())
				if err != nil {
					t.Errorf("Failed to list indexes: %v", err)
				}
				var indexes []bson.M
				if err = cursor.All(context.Background(), &indexes); err != nil {
					t.Errorf("Failed to decode indexes: %v", err)
				}
				for _, idx := range indexes {
					if name, ok := idx["name"].(string); ok && name == "name_1" {
						t.Error("Index was not dropped")
					}
				}
			},
		},
		{
			name:   "delete many documents",
			script: `db.test_collection.insertMany([{"name": "test1"}, {"name": "test2"}]); db.test_collection.deleteMany({"name": "test1"})`,
			verify: func(t *testing.T, db *mongo.Database) {
				count, err := db.Collection("test_collection").CountDocuments(context.Background(), bson.M{})
				if err != nil {
					t.Errorf("Failed to count documents: %v", err)
				}
				if count != 1 {
					t.Errorf("Unexpected document count: got %v, want 1", count)
				}
				var result bson.M
				err = db.Collection("test_collection").FindOne(context.Background(), bson.M{}).Decode(&result)
				if err != nil {
					t.Errorf("Failed to find document: %v", err)
				}
				if result["name"] != "test2" {
					t.Errorf("Unexpected document: got %v, want test2", result["name"])
				}
			},
		},
		{
			name:   "drop collection",
			script: `db.createCollection("to_drop"); db.to_drop.insertOne({"test": "value"}); db.to_drop.drop()`,
			verify: func(t *testing.T, db *mongo.Database) {
				names, err := db.ListCollectionNames(context.Background(), bson.M{})
				if err != nil {
					t.Errorf("Failed to list collections: %v", err)
				}
				for _, name := range names {
					if name == "to_drop" {
						t.Error("Collection was not dropped")
					}
				}
			},
		},
		{
			name:   "insert many documents",
			script: `db.test_collection.insertMany([{"name": "test1", "value": 1}, {"name": "test2", "value": 2}])`,
			verify: func(t *testing.T, db *mongo.Database) {
				count, err := db.Collection("test_collection").CountDocuments(context.Background(), bson.M{})
				if err != nil {
					t.Errorf("Failed to count documents: %v", err)
				}
				if count != 2 {
					t.Errorf("Unexpected document count: got %v, want 2", count)
				}
				cursor, err := db.Collection("test_collection").Find(context.Background(), bson.M{})
				if err != nil {
					t.Errorf("Failed to find documents: %v", err)
				}
				var results []bson.M
				if err = cursor.All(context.Background(), &results); err != nil {
					t.Errorf("Failed to decode documents: %v", err)
				}
				if len(results) != 2 {
					t.Errorf("Unexpected number of documents: got %v, want 2", len(results))
				}
			},
		},
		{
			name:   "update one document",
			script: `db.test_collection.insertOne({"name": "test", "value": 1}); db.test_collection.updateOne({"name": "test"}, {"$set": {"value": 2}})`,
			verify: func(t *testing.T, db *mongo.Database) {
				var result bson.M
				err := db.Collection("test_collection").FindOne(context.Background(), bson.M{"name": "test"}).Decode(&result)
				if err != nil {
					t.Errorf("Failed to find document: %v", err)
				}
				if result["value"] != int32(2) {
					t.Errorf("Unexpected value: got %v, want 2", result["value"])
				}
			},
		},
		{
			name:   "delete one document",
			script: `db.test_collection.insertMany([{"name": "test1"}, {"name": "test2"}]); db.test_collection.deleteOne({"name": "test1"})`,
			verify: func(t *testing.T, db *mongo.Database) {
				count, err := db.Collection("test_collection").CountDocuments(context.Background(), bson.M{})
				if err != nil {
					t.Errorf("Failed to count documents: %v", err)
				}
				if count != 1 {
					t.Errorf("Unexpected document count: got %v, want 1", count)
				}
				var result bson.M
				err = db.Collection("test_collection").FindOne(context.Background(), bson.M{}).Decode(&result)
				if err != nil {
					t.Errorf("Failed to find document: %v", err)
				}
				if result["name"] != "test2" {
					t.Errorf("Unexpected document: got %v, want test2", result["name"])
				}
			},
		},
		{
			name:   "aggregate documents",
			script: `db.test_collection.insertMany([{"name": "test1", "value": 1}, {"name": "test2", "value": 2}]); db.test_collection.aggregate([{"$match": {"value": {"$gt": 1}}}, {"$project": {"name": 1}}])`,
			verify: func(t *testing.T, db *mongo.Database) {
				cursor, err := db.Collection("test_collection").Aggregate(context.Background(), []bson.M{
					{"$match": bson.M{"value": bson.M{"$gt": 1}}},
					{"$project": bson.M{"name": 1}},
				})
				if err != nil {
					t.Errorf("Failed to execute aggregation: %v", err)
				}
				var results []bson.M
				if err = cursor.All(context.Background(), &results); err != nil {
					t.Errorf("Failed to decode aggregation results: %v", err)
				}
				if len(results) != 1 {
					t.Errorf("Unexpected number of results: got %v, want 1", len(results))
				}
				if results[0]["name"] != "test2" {
					t.Errorf("Unexpected result: got %v, want test2", results[0]["name"])
				}
			},
		},
		{
			name:    "invalid command format",
			script:  `invalid command`,
			wantErr: true,
		},
		{
			name:    "unsupported operation",
			script:  `db.test_collection.unsupportedOperation()`,
			wantErr: true,
		},
		{
			name:    "invalid JSON in command",
			script:  `db.test_collection.insertOne(invalid json)`,
			wantErr: true,
		},
		{
			name:   "multiple commands",
			script: `db.createCollection("multi_test"); db.multi_test.insertOne({"test": "value"})`,
			verify: func(t *testing.T, db *mongo.Database) {
				var result bson.M
				err := db.Collection("multi_test").FindOne(context.Background(), bson.M{}).Decode(&result)
				if err != nil {
					t.Errorf("Failed to find document: %v", err)
				}
				if result["test"] != "value" {
					t.Errorf("Unexpected value: got %v, want 'value'", result["test"])
				}
			},
		},
		{
			name:   "create timeseries collection",
			script: `db.createCollection("timeseries_test", {"timeseries": {"timeField": "timestamp", "metaField": "sensorId", "granularity": "hours"}})`,
			verify: func(t *testing.T, db *mongo.Database) {
				// Get collection info
				var result bson.M
				err := db.RunCommand(context.Background(), bson.D{
					{Key: "listCollections", Value: 1},
					{Key: "filter", Value: bson.D{{Key: "name", Value: "timeseries_test"}}},
				}).Decode(&result)
				if err != nil {
					t.Errorf("Failed to get collection info: %v", err)
					return
				}

				// Verify timeseries options
				cursor := result["cursor"].(bson.D)
				for _, elem := range cursor {
					if elem.Key == "firstBatch" {
						firstBatch := elem.Value.(bson.A)
						if len(firstBatch) == 0 {
							t.Error("No collection found")
							return
						}

						collection := firstBatch[0].(bson.D)
						for _, colElem := range collection {
							if colElem.Key == "options" {
								options := colElem.Value.(bson.D)
								for _, optElem := range options {
									if optElem.Key == "timeseries" {
										timeseries := optElem.Value.(bson.D)
										for _, tsElem := range timeseries {
											switch tsElem.Key {
											case "timeField":
												if timeField := tsElem.Value.(string); timeField != "timestamp" {
													t.Errorf("Unexpected timeField: got %v, want timestamp", timeField)
												}
											case "metaField":
												if metaField := tsElem.Value.(string); metaField != "sensorId" {
													t.Errorf("Unexpected metaField: got %v, want sensorId", metaField)
												}
											case "granularity":
												if granularity := tsElem.Value.(string); granularity != "hours" {
													t.Errorf("Unexpected granularity: got %v, want hours", granularity)
												}
											}
										}
									}
								}
							}
						}
					}
				}
			},
		},
		{
			name:   "create collection with validation",
			script: `db.createCollection("validation_test", {"validator": {"$jsonSchema": {"bsonType": "object", "required": ["name", "age"], "properties": {"name": {"bsonType": "string"}, "age": {"bsonType": "int"}}}, "validationLevel": "strict", "validationAction": "error"}})`,
			verify: func(t *testing.T, db *mongo.Database) {
				// Get collection info
				var result bson.M
				err := db.RunCommand(context.Background(), bson.D{
					{Key: "listCollections", Value: 1},
					{Key: "filter", Value: bson.D{{Key: "name", Value: "validation_test"}}},
				}).Decode(&result)
				if err != nil {
					t.Errorf("Failed to get collection info: %v", err)
					return
				}

				// Verify validation options
				cursor := result["cursor"].(bson.D)
				for _, elem := range cursor {
					if elem.Key == "firstBatch" {
						firstBatch := elem.Value.(bson.A)
						if len(firstBatch) == 0 {
							t.Error("No collection found")
							return
						}

						collection := firstBatch[0].(bson.D)
						for _, colElem := range collection {
							if colElem.Key == "options" {
								options := colElem.Value.(bson.D)
								for _, optElem := range options {
									if optElem.Key == "validator" {
										validator := optElem.Value.(bson.D)
										for _, valElem := range validator {
											if valElem.Key == "$jsonSchema" {
												schema := valElem.Value.(bson.D)
												for _, schemaElem := range schema {
													switch schemaElem.Key {
													case "bsonType":
														if bsonType := schemaElem.Value.(string); bsonType != "object" {
															t.Errorf("Unexpected bsonType: got %v, want object", bsonType)
														}
													case "required":
														required := schemaElem.Value.(bson.A)
														requiredFields := []string{"name", "age"}
														for i, field := range requiredFields {
															if required[i].(string) != field {
																t.Errorf("Missing required field: %v", field)
															}
														}
													case "properties":
														properties := schemaElem.Value.(bson.D)
														for _, propElem := range properties {
															switch propElem.Key {
															case "name":
																nameType := propElem.Value.(bson.D)[0].Value.(string)
																if nameType != "string" {
																	t.Errorf("Unexpected name type: got %v, want string", nameType)
																}
															case "age":
																ageType := propElem.Value.(bson.D)[0].Value.(string)
																if ageType != "int" {
																	t.Errorf("Unexpected age type: got %v, want int", ageType)
																}
															}
														}
													}
												}
											}
										}
									}
								}
							}
						}
					}
				}
			},
		},
		{
			name:   "create timeseries collection with optional fields",
			script: `db.createCollection("timeseries_optional", {"timeseries": {"timeField": "timestamp", "granularity": "minutes"}})`,
			verify: func(t *testing.T, db *mongo.Database) {
				// Get collection info
				var result bson.M
				err := db.RunCommand(context.Background(), bson.D{
					{Key: "listCollections", Value: 1},
					{Key: "filter", Value: bson.D{{Key: "name", Value: "timeseries_optional"}}},
				}).Decode(&result)
				if err != nil {
					t.Errorf("Failed to get collection info: %v", err)
					return
				}

				// Verify timeseries options
				cursor := result["cursor"].(bson.D)
				for _, elem := range cursor {
					if elem.Key == "firstBatch" {
						firstBatch := elem.Value.(bson.A)
						if len(firstBatch) == 0 {
							t.Error("No collection found")
							return
						}

						collection := firstBatch[0].(bson.D)
						for _, colElem := range collection {
							if colElem.Key == "options" {
								options := colElem.Value.(bson.D)
								tsFound := false
								for _, optElem := range options {
									if optElem.Key == "timeseries" {
										tsFound = true
										timeseries := optElem.Value.(bson.D)
										timeFieldFound := false
										granularityFound := false
										for _, tsElem := range timeseries {
											switch tsElem.Key {
											case "timeField":
												timeFieldFound = true
												if timeField := tsElem.Value.(string); timeField != "timestamp" {
													t.Errorf("Unexpected timeField: got %v, want timestamp", timeField)
												}
											case "metaField":
												t.Error("metaField should not be set")
											case "granularity":
												granularityFound = true
												if granularity := tsElem.Value.(string); granularity != "minutes" {
													t.Errorf("Unexpected granularity: got %v, want minutes", granularity)
												}
											}
										}
										if !timeFieldFound {
											t.Error("timeField not found in timeseries options")
										}
										if !granularityFound {
											t.Error("granularity not found in timeseries options")
										}
									}
								}
								if !tsFound {
									t.Error("timeseries options not found")
								}
							}
						}
					}
				}
			},
		},
		{
			name:   "create timeseries collection with different granularity",
			script: `db.createCollection("timeseries_days", {"timeseries": {"timeField": "time", "metaField": "device", "granularity": "hours"}})`,
			verify: func(t *testing.T, db *mongo.Database) {
				// Get collection info
				var result bson.M
				err := db.RunCommand(context.Background(), bson.D{
					{Key: "listCollections", Value: 1},
					{Key: "filter", Value: bson.D{{Key: "name", Value: "timeseries_days"}}},
				}).Decode(&result)
				if err != nil {
					t.Errorf("Failed to get collection info: %v", err)
					return
				}

				// Verify timeseries options
				cursor := result["cursor"].(bson.D)
				for _, elem := range cursor {
					if elem.Key == "firstBatch" {
						firstBatch := elem.Value.(bson.A)
						if len(firstBatch) == 0 {
							t.Error("No collection found")
							return
						}

						collection := firstBatch[0].(bson.D)
						for _, colElem := range collection {
							if colElem.Key == "options" {
								options := colElem.Value.(bson.D)
								for _, optElem := range options {
									if optElem.Key == "timeseries" {
										timeseries := optElem.Value.(bson.D)
										timeFieldFound := false
										metaFieldFound := false
										granularityFound := false
										for _, tsElem := range timeseries {
											switch tsElem.Key {
											case "timeField":
												timeFieldFound = true
												if timeField := tsElem.Value.(string); timeField != "time" {
													t.Errorf("Unexpected timeField: got %v, want time", timeField)
												}
											case "metaField":
												metaFieldFound = true
												if metaField := tsElem.Value.(string); metaField != "device" {
													t.Errorf("Unexpected metaField: got %v, want device", metaField)
												}
											case "granularity":
												granularityFound = true
												if granularity := tsElem.Value.(string); granularity != "hours" {
													t.Errorf("Unexpected granularity: got %v, want hours", granularity)
												}
											}
										}
										if !timeFieldFound {
											t.Error("timeField not found in timeseries options")
										}
										if !metaFieldFound {
											t.Error("metaField not found in timeseries options")
										}
										if !granularityFound {
											t.Error("granularity not found in timeseries options")
										}
									}
								}
							}
						}
					}
				}
			},
		},
		{
			name:   "create collection with validation levels",
			script: `db.createCollection("validation_levels", {"validator": {"$jsonSchema": {"bsonType": "object", "required": ["email"], "properties": {"email": {"bsonType": "string", "pattern": "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$", "description": "Email address"}}}}, "validationLevel": "moderate", "validationAction": "warn"})`,
			verify: func(t *testing.T, db *mongo.Database) {
				// Get collection info
				var result bson.M
				err := db.RunCommand(context.Background(), bson.D{
					{Key: "listCollections", Value: 1},
					{Key: "filter", Value: bson.D{{Key: "name", Value: "validation_levels"}}},
				}).Decode(&result)
				if err != nil {
					t.Errorf("Failed to get collection info: %v", err)
					return
				}

				// Verify validation options
				cursor := result["cursor"].(bson.D)
				for _, elem := range cursor {
					if elem.Key == "firstBatch" {
						firstBatch := elem.Value.(bson.A)
						if len(firstBatch) == 0 {
							t.Error("No collection found")
							return
						}

						collection := firstBatch[0].(bson.D)
						for _, colElem := range collection {
							if colElem.Key == "options" {
								options := colElem.Value.(bson.D)
								validatorFound := false
								validationLevelFound := false
								validationActionFound := false

								for _, optElem := range options {
									switch optElem.Key {
									case "validator":
										validatorFound = true
										// Verify validator structure
										validator := optElem.Value.(bson.D)
										for _, valElem := range validator {
											if valElem.Key == "$jsonSchema" {
												schema := valElem.Value.(bson.D)
												for _, schemaElem := range schema {
													if schemaElem.Key == "required" {
														required := schemaElem.Value.(bson.A)
														if len(required) != 1 || required[0].(string) != "email" {
															t.Error("Required fields incorrect")
														}
													}
												}
											}
										}
									case "validationLevel":
										validationLevelFound = true
										if level := optElem.Value.(string); level != "moderate" {
											t.Errorf("Unexpected validation level: got %v, want moderate", level)
										}
									case "validationAction":
										validationActionFound = true
										if action := optElem.Value.(string); action != "warn" {
											t.Errorf("Unexpected validation action: got %v, want warn", action)
										}
									}
								}

								if !validatorFound {
									t.Error("validator not found")
								}
								if !validationLevelFound {
									t.Error("validationLevel not found")
								}
								if !validationActionFound {
									t.Error("validationAction not found")
								}
							}
						}
					}
				}
			},
		},
		{
			name:   "create collection with complex validator",
			script: `db.createCollection("complex_validation", {"validator": {"$and": [{"qty": {"$gt": 0}}, {"price": {"$gt": 0}}]}, "validationLevel": "strict"})`,
			verify: func(t *testing.T, db *mongo.Database) {
				// Get collection info
				var result bson.M
				err := db.RunCommand(context.Background(), bson.D{
					{Key: "listCollections", Value: 1},
					{Key: "filter", Value: bson.D{{Key: "name", Value: "complex_validation"}}},
				}).Decode(&result)
				if err != nil {
					t.Errorf("Failed to get collection info: %v", err)
					return
				}

				// Verify validation options
				cursor := result["cursor"].(bson.D)
				for _, elem := range cursor {
					if elem.Key == "firstBatch" {
						firstBatch := elem.Value.(bson.A)
						if len(firstBatch) == 0 {
							t.Error("No collection found")
							return
						}

						collection := firstBatch[0].(bson.D)
						for _, colElem := range collection {
							if colElem.Key == "options" {
								options := colElem.Value.(bson.D)
								validatorFound := false
								validationLevelFound := false

								for _, optElem := range options {
									switch optElem.Key {
									case "validator":
										validatorFound = true
										// Verify validator structure contains $and operator
										validator := optElem.Value.(bson.D)
										andFound := false
										for _, valElem := range validator {
											if valElem.Key == "$and" {
												andFound = true
												break
											}
										}
										if !andFound {
											t.Error("$and operator not found in validator")
										}
									case "validationLevel":
										validationLevelFound = true
										if level := optElem.Value.(string); level != "strict" {
											t.Errorf("Unexpected validation level: got %v, want strict", level)
										}
									}
								}

								if !validatorFound {
									t.Error("validator not found")
								}
								if !validationLevelFound {
									t.Error("validationLevel not found")
								}
							}
						}
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up before each test
			cleanup()

			// Create executor and execute script
			executor := operations.NewOperationExecutor(db)
			err := executor.ExecuteScript(context.Background(), tt.script)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExecuteScript() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.verify != nil {
				tt.verify(t, db)
			}

			// Clean up after each test
			cleanup()
		})
	}
}
