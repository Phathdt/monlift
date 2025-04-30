package migration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/phathdt/mongo-migrate/internal/exec"
)

type Migration struct {
	Version  string
	UpPath   string
	DownPath string
}

func NewMigration(version, upPath, downPath string) *Migration {
	return &Migration{
		Version:  version,
		UpPath:   upPath,
		DownPath: downPath,
	}
}

func initVersionsCollection() error {
	dbURI := os.Getenv("DB_URI")
	if dbURI == "" {
		return fmt.Errorf("DB_URI environment variable is not set")
	}

	cmd := exec.NewCommand("mongosh", dbURI, "--eval", `
		if (!db.getCollectionNames().includes('versions')) {
			db.createCollection('versions');
			db.versions.createIndex({ version: 1 }, { unique: true });
		}
	`)
	_, err := cmd.Run()
	return err
}

func (m *Migration) Up() error {
	if err := initVersionsCollection(); err != nil {
		return fmt.Errorf("failed to initialize versions collection: %w", err)
	}

	content, err := os.ReadFile(m.UpPath)
	if err != nil {
		return fmt.Errorf("failed to read up migration: %w", err)
	}

	dbURI := os.Getenv("DB_URI")
	if dbURI == "" {
		return fmt.Errorf("DB_URI environment variable is not set")
	}

	cmd := exec.NewCommand("mongosh", dbURI, "--eval", string(content))
	output, err := cmd.Run()
	if err != nil {
		return fmt.Errorf("migration up failed: %w", err)
	}
	fmt.Println(output)

	// Record migration
	recordCmd := exec.NewCommand("mongosh", dbURI, "--eval", fmt.Sprintf(`
		db.versions.insertOne({
			version: "%s",
			applied_at: new Date()
		})
	`, m.Version))
	if _, err := recordCmd.Run(); err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	return nil
}

func (m *Migration) Down() error {
	if err := initVersionsCollection(); err != nil {
		return fmt.Errorf("failed to initialize versions collection: %w", err)
	}

	content, err := os.ReadFile(m.DownPath)
	if err != nil {
		return fmt.Errorf("failed to read down migration: %w", err)
	}

	dbURI := os.Getenv("DB_URI")
	if dbURI == "" {
		return fmt.Errorf("DB_URI environment variable is not set")
	}

	cmd := exec.NewCommand("mongosh", dbURI, "--eval", string(content))
	output, err := cmd.Run()
	if err != nil {
		return fmt.Errorf("migration down failed: %w", err)
	}
	fmt.Println(output)

	// Remove migration record
	recordCmd := exec.NewCommand("mongosh", dbURI, "--eval", fmt.Sprintf(`
		db.versions.deleteOne({ version: "%s" })
	`, m.Version))
	if _, err := recordCmd.Run(); err != nil {
		return fmt.Errorf("failed to remove migration record: %w", err)
	}

	return nil
}

func GetMigrations(dir string) ([]*Migration, error) {
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

		migrations = append(migrations, NewMigration(version, upFile, downFile))
	}

	return migrations, nil
}

type VersionRecord struct {
	Version string `json:"version"`
}

func GetExecutedMigrations() (map[string]bool, error) {
	if err := initVersionsCollection(); err != nil {
		return nil, fmt.Errorf("failed to initialize versions collection: %w", err)
	}

	dbURI := os.Getenv("DB_URI")
	if dbURI == "" {
		return nil, fmt.Errorf("DB_URI environment variable is not set")
	}

	cmd := exec.NewCommand("mongosh", dbURI, "--eval", `
		const versions = db.versions.find({}, { version: 1, _id: 0 }).toArray();
		print(JSON.stringify(versions));
	`)
	output, err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to get executed migrations: %w", err)
	}

	// Clean up the output to get valid JSON
	output = strings.TrimSpace(output)
	if output == "" {
		return make(map[string]bool), nil
	}

	var versions []VersionRecord
	if err := json.Unmarshal([]byte(output), &versions); err != nil {
		return nil, fmt.Errorf("failed to parse versions: %w", err)
	}

	executed := make(map[string]bool)
	for _, v := range versions {
		executed[v.Version] = true
	}

	return executed, nil
}
