package cmd

import (
	"fmt"
	"log"
	"os"
	"text/tabwriter"

	"github.com/joho/godotenv"
	"github.com/phathdt/monlift/internal/migration"
	"github.com/urfave/cli/v2"
)

func statusCommand(c *cli.Context) error {
	if err := godotenv.Load(); err != nil {
		return fmt.Errorf("failed to load .env file: %w", err)
	}

	db, err := ConnectDB()
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Client().Disconnect(c.Context)

	status, err := migration.GetMigrationStatus(db, "migrations")
	if err != nil {
		return fmt.Errorf("failed to get migration status: %w", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Version\tStatus\tApplied At")
	fmt.Fprintln(w, "-------\t------\t----------")

	for _, s := range status {
		appliedAt := ""
		if !s.AppliedAt.IsZero() {
			appliedAt = s.AppliedAt.Format("2006-01-02 15:04:05")
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", s.Version, s.Status, appliedAt)
	}

	if err := w.Flush(); err != nil {
		log.Fatal(err)
	}

	return nil
}
