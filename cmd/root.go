package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/urfave/cli/v2"
)

const (
	serviceName = "monlift"
	version     = "1.0.0"
)

func Execute() {
	app := &cli.App{
		Name:    serviceName,
		Usage:   fmt.Sprintf("start %s", serviceName),
		Version: version,
		Commands: []*cli.Command{
			{
				Name:   "up",
				Usage:  "Run all pending migrations",
				Action: UpCommand,
			},
			{
				Name:   "down",
				Usage:  "Rollback last migration",
				Action: DownCommand,
			},
			{
				Name:      "create",
				Usage:     "Create a new migration file",
				ArgsUsage: "[migration_name]",
				Action:    createCommand,
			},
			{
				Name:   "status",
				Usage:  "Show migration status",
				Action: statusCommand,
			},
			{
				Name:   "version",
				Usage:  "Show version information",
				Action: versionCommand,
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
