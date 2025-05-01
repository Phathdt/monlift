package cmd

import (
	"fmt"

	"github.com/urfave/cli/v2"
)

func versionCommand(c *cli.Context) error {
	fmt.Printf("%s version %s\n", serviceName, version)
	return nil
}
