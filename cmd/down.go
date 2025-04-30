package cmd

import (
	"fmt"
	"sort"

	"github.com/joho/godotenv"
	"github.com/phathdt/monlift/internal/migration"
	"github.com/urfave/cli/v2"
)

func DownCommand(c *cli.Context) error {
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

	// Filter executed migrations and sort by version in descending order
	var executedMigrations []*migration.Migration
	for _, m := range migrations {
		if executed[m.Version] {
			executedMigrations = append(executedMigrations, m)
		}
	}

	if len(executedMigrations) == 0 {
		fmt.Println("No migrations to rollback")
		return nil
	}

	// Sort by version in descending order to get the most recent migration
	sort.Slice(executedMigrations, func(i, j int) bool {
		return executedMigrations[i].Version > executedMigrations[j].Version
	})

	// Rollback the most recent migration
	lastMigration := executedMigrations[0]
	fmt.Printf("Rolling back migration %s...\n", lastMigration.Version)
	if err := lastMigration.Down(); err != nil {
		return fmt.Errorf("failed to rollback migration %s: %w", lastMigration.Version, err)
	}
	fmt.Printf("Migration %s rolled back successfully\n", lastMigration.Version)

	return nil
}
