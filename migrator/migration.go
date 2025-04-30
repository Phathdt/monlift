package migrator

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

type MigrationFunc func(db *mongo.Database) error
type RollbackFunc func(db *mongo.Database) error

type MigrationEntry struct {
	Version  string
	Name     string
	Migrate  MigrationFunc
	Rollback RollbackFunc
}

type SchemaVersion struct {
	Version      string    `bson:"version" json:"version"`
	AppliedAt    time.Time `bson:"applied_at" json:"applied_at"`
	LastRollback time.Time `bson:"last_rollback,omitempty" json:"last_rollback,omitempty"`
	Status       string    `bson:"status" json:"status"`
}

type Migration struct {
	db *mongo.Database
}

var migrations = make(map[string]MigrationEntry)

func NewMigration(db *mongo.Database) *Migration {
	return &Migration{db: db}
}

func Register(version string, name string, migrate MigrationFunc, rollback RollbackFunc) {
	if _, exists := migrations[version]; exists {
		panic(fmt.Sprintf("Migration version %s already exists", version))
	}
	migrations[version] = MigrationEntry{
		Version:  version,
		Name:     name,
		Migrate:  migrate,
		Rollback: rollback,
	}
}

func GetMigrations() []MigrationEntry {
	var sorted []MigrationEntry
	for _, m := range migrations {
		sorted = append(sorted, m)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Version < sorted[j].Version
	})
	return sorted
}

func GetLatestVersion() string {
	latest := ""
	for version := range migrations {
		if version > latest {
			latest = version
		}
	}
	return latest
}

func GetMigration(version string) (MigrationEntry, bool) {
	fmt.Println("GetMigration")
	fmt.Println("version", version)
	fmt.Println("migrations", migrations)
	migration, ok := migrations[version]
	return migration, ok
}

func (m *Migration) RunSchemaUp() error {
	appliedMigrations, err := m.getAppliedMigrations()
	if err != nil {
		return err
	}

	availableMigrations := GetMigrations()
	if len(availableMigrations) == 0 {
		return fmt.Errorf("no migrations found in migrations directory")
	}

	pendingMigrations := m.getPendingMigrations(appliedMigrations, availableMigrations)
	if len(pendingMigrations) == 0 {
		fmt.Println("No pending migrations to apply")
		return nil
	}

	fmt.Printf("Found %d pending migrations to apply\n", len(pendingMigrations))
	for _, migration := range pendingMigrations {
		fmt.Printf("Applying migration %s: %s\n", migration.Version, migration.Name)
		if err := m.applyMigration(migration); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", migration.Version, err)
		}
	}

	fmt.Println("All migrations applied successfully")
	return nil
}

func (m *Migration) getAppliedMigrations() (map[string]SchemaVersion, error) {
	ctx := context.Background()

	if err := m.db.CreateCollection(ctx, "schema_versions"); err != nil {
		if !strings.Contains(err.Error(), "NamespaceExists") {
			return nil, err
		}
	}

	cursor, err := m.db.Collection("schema_versions").Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	appliedMigrations := make(map[string]SchemaVersion)
	for cursor.Next(ctx) {
		var version SchemaVersion
		if err := cursor.Decode(&version); err != nil {
			return nil, err
		}
		appliedMigrations[version.Version] = version
	}

	return appliedMigrations, nil
}

func (m *Migration) getPendingMigrations(applied map[string]SchemaVersion, available []MigrationEntry) []MigrationEntry {
	var pending []MigrationEntry
	for _, migration := range available {
		if _, exists := applied[migration.Version]; !exists {
			pending = append(pending, migration)
		}
	}
	sort.Slice(pending, func(i, j int) bool {
		return pending[i].Version < pending[j].Version
	})
	return pending
}

func (m *Migration) applyMigration(migration MigrationEntry) error {
	ctx := context.Background()

	if err := migration.Migrate(m.db); err != nil {
		return fmt.Errorf("failed to execute migration: %w", err)
	}

	_, err := m.db.Collection("schema_versions").InsertOne(ctx, SchemaVersion{
		Version:   migration.Version,
		AppliedAt: time.Now(),
		Status:    "applied",
	})

	if err != nil {
		return fmt.Errorf("failed to record migration in schema_versions: %w", err)
	}

	return nil
}

func (m *Migration) Rollback() error {
	appliedMigrations, err := m.getAppliedMigrations()
	if err != nil {
		return err
	}

	if len(appliedMigrations) == 0 {
		fmt.Println("No migrations to rollback")
		return nil
	}

	// Get the latest applied migration
	var latestVersion string
	for version := range appliedMigrations {
		if version > latestVersion {
			latestVersion = version
		}
	}

	fmt.Printf("Rolling back migration %s\n", latestVersion)
	if err := m.rollbackMigration(latestVersion); err != nil {
		return fmt.Errorf("failed to rollback migration %s: %w", latestVersion, err)
	}

	ctx := context.Background()
	_, err = m.db.Collection("schema_versions").UpdateOne(
		ctx,
		bson.M{"version": latestVersion},
		bson.M{
			"$set": bson.M{
				"last_rollback": time.Now(),
				"status":        "rolled_back",
			},
		},
	)

	if err != nil {
		return fmt.Errorf("failed to update schema_versions: %w", err)
	}

	fmt.Printf("Successfully rolled back migration %s\n", latestVersion)
	return nil
}

func (m *Migration) RunSeeding() error {
	return m.seedData()
}

func (m *Migration) getCurrentVersion() (string, error) {
	ctx := context.Background()

	if err := m.db.CreateCollection(ctx, "schema_versions"); err != nil {
		if !strings.Contains(err.Error(), "NamespaceExists") {
			return "", err
		}
	}

	var version SchemaVersion
	err := m.db.Collection("schema_versions").
		FindOne(ctx, bson.M{}).
		Decode(&version)

	if err == mongo.ErrNoDocuments {
		_, err = m.db.Collection("schema_versions").InsertOne(ctx, SchemaVersion{
			Version:   "",
			AppliedAt: time.Now(),
		})
		if err != nil {
			return "", err
		}
		return "", nil
	}

	return version.Version, err
}

func (m *Migration) migrateSchema(currentVersion string) error {
	ctx := context.Background()
	sortedMigrations := GetMigrations()

	for _, migration := range sortedMigrations {
		if migration.Version <= currentVersion {
			continue
		}

		if err := migration.Migrate(m.db); err != nil {
			return fmt.Errorf("failed to apply migration %s (%s): %w",
				migration.Version, migration.Name, err)
		}

		_, err := m.db.Collection("schema_versions").UpdateOne(
			ctx,
			bson.M{},
			bson.M{
				"$set": SchemaVersion{
					Version:   migration.Version,
					AppliedAt: time.Now(),
					Status:    "applied",
				},
			},
			options.Update().SetUpsert(true),
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *Migration) rollbackMigration(version string) error {
	migration, exists := migrations[version]
	if !exists {
		return fmt.Errorf("no rollback implemented for version: %s", version)
	}

	return migration.Rollback(m.db)
}

func (m *Migration) seedData() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := m.seedAdmin(ctx); err != nil {
		return err
	}

	return nil
}

func (m *Migration) seedAdmin(ctx context.Context) error {
	count, err := m.db.Collection("admins").CountDocuments(ctx, bson.M{})
	if err != nil {
		return err
	}

	if count == 0 {
		adminPassword := os.Getenv("ADMIN_PASSWORD")
		if adminPassword == "" {
			return fmt.Errorf("ADMIN_PASSWORD environment variable is not set")
		}

		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
		admin := bson.M{
			"name":     "Super Admin",
			"email":    "admin@example.com",
			"password": string(hashedPassword),
		}

		_, err = m.db.Collection("admins").InsertOne(ctx, admin)
		if err != nil {
			return err
		}
	}

	return nil
}
