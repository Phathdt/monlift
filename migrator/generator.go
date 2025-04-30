package migrator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

const migrationTemplate = `package internal

import (
	"context"

	"github.com/phathdt/mongo-migrate/migrator"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func init() {
	migrator.Register("{{.Version}}", "{{.Description}}",
		func(db *mongo.Database) error {
			_, err := db.Collection("{{.Collection}}").UpdateMany(
				context.Background(),
				bson.M{},
				bson.M{"$set": bson.M{
					// Add your field updates here
				}},
			)
			return err
		},
		func(db *mongo.Database) error {
			_, err := db.Collection("{{.Collection}}").UpdateMany(
				context.Background(),
				bson.M{},
				bson.M{"$unset": bson.M{
					// Add your field removals here
				}},
			)
			return err
		},
	)
}
`

type MigrationGenerator struct {
	Version     string
	Description string
	Collection  string
	FilePath    string
}

func GenerateMigrationFile(name string) error {
	name = strings.ToLower(strings.ReplaceAll(name, " ", "_"))

	version := time.Now().Format("20060102150405")

	migrationsDir := "migrations"
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		return fmt.Errorf("failed to create migrations directory: %w", err)
	}

	filename := fmt.Sprintf("%s_%s.go", version, name)
	fullPath := filepath.Join(migrationsDir, filename)

	if _, err := os.Stat(fullPath); err == nil {
		return fmt.Errorf("migration file already exists: %s", fullPath)
	}

	parts := strings.Split(name, "_")
	collection := "your_collection"
	if len(parts) > 1 {
		for i := len(parts) - 1; i >= 0; i-- {
			if parts[i] == "to" && i+1 < len(parts) {
				collection = parts[i+1]
				break
			}
		}
	}

	migration := MigrationGenerator{
		Version:     version,
		Description: strings.ReplaceAll(name, "_", " "),
		Collection:  collection,
		FilePath:    fullPath,
	}

	tmpl, err := template.New("migration").Parse(migrationTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, migration); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	fmt.Printf("Created migration file: %s\n", fullPath)
	return nil
}
