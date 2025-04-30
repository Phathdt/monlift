package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/urfave/cli/v2"
)

const serviceName = "monlift"

func Execute() {
	app := &cli.App{
		Name:  serviceName,
		Usage: fmt.Sprintf("start %s", serviceName),
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
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
