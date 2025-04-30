package exec

import (
	"fmt"
	"os"
	"os/exec"
)

type Command struct {
	cmd *exec.Cmd
}

func NewCommand(name string, args ...string) *Command {
	return &Command{
		cmd: exec.Command(name, args...),
	}
}

func (c *Command) WithEnv(env map[string]string) *Command {
	for k, v := range env {
		c.cmd.Env = append(os.Environ(), fmt.Sprintf("%s=%s", k, v))
	}
	return c
}

func (c *Command) Run() (string, error) {
	output, err := c.cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("command failed: %s\n%s", err, string(output))
	}
	return string(output), nil
}
