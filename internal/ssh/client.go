package ssh

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/lahiruudayakumara/devnode/internal/config"
)

type Client struct {
	Config config.Config
	In     io.Reader
	Out    io.Writer
	Err    io.Writer
}

func (c Client) Run(ctx context.Context, command []string, tty, batch bool) error {
	args := c.baseArgs(batch)
	if tty {
		args = append(args, "-t")
	}
	args = append(args, config.Target(c.Config))
	if len(command) > 0 {
		args = append(args, JoinCommand(command))
	}
	cmd := exec.CommandContext(ctx, "ssh", args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = c.In, c.Out, c.Err
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ssh: %w", err)
	}
	return nil
}

func (c Client) Output(ctx context.Context, script string) ([]byte, error) {
	args := append(c.baseArgs(false), config.Target(c.Config), script)
	cmd := exec.CommandContext(ctx, "ssh", args...)
	cmd.Stdin, cmd.Stderr = c.In, c.Err
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ssh: %w", err)
	}
	return out, nil
}

func (c Client) baseArgs(batch bool) []string {
	args := []string{"-o", "ConnectTimeout=" + strconv.Itoa(c.Config.ConnectTimeout), "-p", strconv.Itoa(c.Config.Port)}
	if batch {
		args = append(args, "-o", "BatchMode=yes")
	}
	if c.Config.IdentityFile != "" {
		args = append(args, "-i", c.Config.IdentityFile)
	}
	return args
}

// JoinCommand quotes every argument for the remote POSIX shell used by ssh.
func JoinCommand(args []string) string {
	q := make([]string, len(args))
	for i, arg := range args {
		q[i] = "'" + strings.ReplaceAll(arg, "'", `'"'"'`) + "'"
	}
	return strings.Join(q, " ")
}

func IsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
