package cmd

import (
	"fmt"
	"sort"

	"github.com/joho/godotenv"
	"github.com/phathdt/monlift/internal/migration"
	"github.com/urfave/cli/v2"
)

func UpCommand(c *cli.Context) error {
	if err := godotenv.Load(); err != nil {
		return fmt.Errorf("failed to load .env file: %w", err)
	}

	database, err := ConnectDB()
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	migrations, err := migration.GetMigrations("migrations", database)
	if err != nil {
		return fmt.Errorf("failed to get migrations: %w", err)
	}

	executed, err := migration.GetExecutedMigrations(database)
	if err != nil {
		return fmt.Errorf("failed to get executed migrations: %w", err)
	}

	// Filter out executed migrations
	var pendingMigrations []*migration.Migration
	for _, m := range migrations {
		if !executed[m.Version] {
			pendingMigrations = append(pendingMigrations, m)
		}
	}

	if len(pendingMigrations) == 0 {
		fmt.Println("No pending migrations")
		return nil
	}

	// Sort pending migrations by version
	sort.Slice(pendingMigrations, func(i, j int) bool {
		return pendingMigrations[i].Version < pendingMigrations[j].Version
	})

	for _, m := range pendingMigrations {
		fmt.Printf("Running migration %s...\n", m.Version)
		if err := m.Up(); err != nil {
			return fmt.Errorf("failed to run migration %s: %w", m.Version, err)
		}
		fmt.Printf("Migration %s completed successfully\n", m.Version)
	}

	return nil
}
