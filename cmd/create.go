package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/urfave/cli/v2"
)

func createCommand(c *cli.Context) error {
	if c.NArg() < 1 {
		return fmt.Errorf("migration name is required")
	}

	name := c.Args().Get(0)
	timestamp := time.Now().Format("20060102150405")
	baseFilename := fmt.Sprintf("%s_%s", timestamp, name)

	// Create migrations directory if it doesn't exist
	if err := os.MkdirAll("migrations", 0755); err != nil {
		return fmt.Errorf("failed to create migrations directory: %w", err)
	}

	// Create up migration file
	upPath := filepath.Join("migrations", baseFilename+".up.js")
	upContent := ``

	if err := os.WriteFile(upPath, []byte(upContent), 0644); err != nil {
		return fmt.Errorf("failed to create up migration file: %w", err)
	}

	// Create down migration file
	downPath := filepath.Join("migrations", baseFilename+".down.js")
	downContent := ``

	if err := os.WriteFile(downPath, []byte(downContent), 0644); err != nil {
		return fmt.Errorf("failed to create down migration file: %w", err)
	}

	fmt.Printf("Created migration files:\n- %s\n- %s\n", upPath, downPath)
	return nil
}
