package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/urfave/cli/v2"
)

const serviceName = "mongo-migrate"

func Execute() {
	app := &cli.App{
		Name:  serviceName,
		Usage: fmt.Sprintf("start %s", serviceName),
		Commands: []*cli.Command{
			{
				Name:   "up",
				Usage:  "Run all pending migrations",
				Action: UpCommand().Action,
			},
			{
				Name:   "down",
				Usage:  "Rollback last migration",
				Action: DownCommand().Action,
			},
			{
				Name:      "new",
				Usage:     "Create a new migration file",
				ArgsUsage: "[migration_name]",
				Action:    newCommand,
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
