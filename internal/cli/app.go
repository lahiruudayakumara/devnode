package cli

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/lahiruudayakumara/devnode/internal/config"
	devssh "github.com/lahiruudayakumara/devnode/internal/ssh"
	"github.com/lahiruudayakumara/devnode/internal/status"
	"github.com/lahiruudayakumara/devnode/internal/ui"
)

const Version = "0.1.0"

type App struct {
	In       io.Reader
	Out, Err io.Writer
}

func New(in io.Reader, out, errOut io.Writer) *App { return &App{In: in, Out: out, Err: errOut} }
func (a *App) PrintError(err error)                { fmt.Fprintln(a.Err, "devnode:", err) }

func (a *App) Run(ctx context.Context, args []string) error {
	root := flag.NewFlagSet("devnode", flag.ContinueOnError)
	root.SetOutput(a.Err)
	configPath := root.String("config", config.DefaultPath(), "configuration file")
	version := root.Bool("version", false, "print the version")
	root.Usage = func() { fmt.Fprint(a.Out, usage) }
	if err := root.Parse(args); errors.Is(err, flag.ErrHelp) {
		return nil
	} else if err != nil {
		return err
	}
	if *version {
		fmt.Fprintln(a.Out, "devnode "+Version)
		return nil
	}
	args = root.Args()
	if len(args) == 0 {
		root.Usage()
		return nil
	}
	if args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		root.Usage()
		return nil
	}
	if args[0] == "version" || args[0] == "--version" {
		fmt.Fprintln(a.Out, "devnode "+Version)
		return nil
	}
	if args[0] == "config" {
		return a.configCommand(*configPath, args[1:])
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	client := devssh.Client{Config: cfg, In: a.In, Out: a.Out, Err: a.Err}

	switch args[0] {
	case "setup":
		return noArgs(args[1:], func() error { return a.setup(ctx, client, cfg, *configPath) })
	case "connect":
		return noArgs(args[1:], func() error { return client.Run(ctx, nil, true, false) })
	case "status":
		if len(args) != 1 {
			return errors.New("usage: devnode status")
		}
		out, err := client.Output(ctx, status.Script(cfg.PostgresService))
		if err != nil {
			return err
		}
		snap, err := status.Parse(string(out))
		if err != nil {
			return err
		}
		ui.Dashboard(a.Out, config.Target(cfg), snap, devssh.IsTerminal(a.Out))
		return nil
	case "monitor":
		return noArgs(args[1:], func() error {
			return client.Run(ctx, []string{"sh", "-c", "command -v btop >/dev/null && exec btop; command -v htop >/dev/null && exec htop; exec top"}, true, false)
		})
	case "docker":
		if len(args) < 2 {
			return errors.New("usage: devnode docker <arguments...>")
		}
		return client.Run(ctx, append([]string{"docker"}, args[1:]...), needsTTY(args[1:]) && devssh.IsTerminal(a.Out), false)
	case "postgres":
		return a.postgres(ctx, client, cfg.PostgresService, args[1:])
	case "services":
		return noArgs(args[1:], func() error {
			return client.Run(ctx, []string{"systemctl", "list-units", "--type=service", "--state=running", "--no-pager"}, false, false)
		})
	case "logs":
		return a.logs(ctx, client, args[1:])
	case "exec":
		if len(args) < 2 {
			return errors.New("usage: devnode exec <command> [arguments...]")
		}
		return client.Run(ctx, args[1:], false, false)
	case "shutdown":
		return a.shutdown(ctx, client, args[1:])
	default:
		return fmt.Errorf("unknown command %q; run `devnode help`", args[0])
	}
}

func (a *App) setup(ctx context.Context, client devssh.Client, cfg config.Config, configPath string) error {
	keyPath := cfg.IdentityFile
	if keyPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("locate home directory: %w", err)
		}
		keyPath = filepath.Join(home, ".ssh", "devnode_ed25519")
	}
	if _, err := os.Stat(keyPath); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err != nil {
			return fmt.Errorf("create SSH directory: %w", err)
		}
		fmt.Fprintln(a.Out, "Creating a dedicated DevNode SSH key...")
		cmd := exec.CommandContext(ctx, "ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "devnode", "-f", keyPath)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = a.In, a.Out, a.Err
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("generate SSH key: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("inspect SSH key: %w", err)
	}

	publicKey, err := os.ReadFile(keyPath + ".pub")
	if err != nil {
		return fmt.Errorf("read public key: %w", err)
	}
	publicKeyText := strings.TrimSpace(string(publicKey))
	if !strings.HasPrefix(publicKeyText, "ssh-ed25519 ") {
		return errors.New("generated public key has an unexpected format")
	}

	fmt.Fprintf(a.Out, "Installing the key on %s (enter the Ubuntu password once if prompted)...\n", config.Target(cfg))
	client.Config.IdentityFile = keyPath
	installScript := `umask 077; mkdir -p "$HOME/.ssh"; touch "$HOME/.ssh/authorized_keys"; grep -qxF "$1" "$HOME/.ssh/authorized_keys" || printf '%s\n' "$1" >> "$HOME/.ssh/authorized_keys"`
	if err := client.Run(ctx, []string{"sh", "-c", installScript, "devnode-setup", publicKeyText}, false, false); err != nil {
		return fmt.Errorf("install public key: %w", err)
	}

	cfg.IdentityFile = keyPath
	if err := config.Save(configPath, cfg); err != nil {
		return err
	}
	if _, err := client.Output(ctx, "printf 'ok\\n'"); err != nil {
		return fmt.Errorf("verify passwordless SSH: %w", err)
	}
	fmt.Fprintln(a.Out, "SSH setup complete. DevNode can now connect without a password.")
	return nil
}

func (a *App) postgres(ctx context.Context, client devssh.Client, service string, args []string) error {
	if len(args) != 1 || (args[0] != "status" && args[0] != "start" && args[0] != "stop" && args[0] != "restart") {
		return errors.New("usage: devnode postgres <status|start|stop|restart>")
	}
	if args[0] == "status" {
		return client.Run(ctx, []string{"systemctl", "status", service, "--no-pager"}, false, false)
	}
	return client.Run(ctx, []string{"sudo", "systemctl", args[0], service}, true, false)
}

func (a *App) logs(ctx context.Context, client devssh.Client, args []string) error {
	fs := flag.NewFlagSet("logs", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	follow := fs.Bool("follow", false, "follow new entries")
	lines := fs.Int("lines", 100, "number of lines")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 || !config.SafeUnit(fs.Arg(0)) {
		return errors.New("usage: devnode logs [--follow] [--lines N] <service>")
	}
	if *lines < 1 || *lines > 100000 {
		return errors.New("--lines must be between 1 and 100000")
	}
	cmd := []string{"journalctl", "-u", fs.Arg(0), "-n", strconv.Itoa(*lines), "--no-pager"}
	if *follow {
		cmd = append(cmd, "-f")
	}
	return client.Run(ctx, cmd, false, false)
}

func (a *App) shutdown(ctx context.Context, client devssh.Client, args []string) error {
	yes := false
	if len(args) == 1 && args[0] == "--yes" {
		yes = true
	} else if len(args) != 0 {
		return errors.New("usage: devnode shutdown [--yes]")
	}
	if !yes {
		fmt.Fprint(a.Out, "Shut down the remote server? Type 'shutdown' to confirm: ")
		answer, err := bufio.NewReader(a.In).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		if strings.TrimSpace(answer) != "shutdown" {
			fmt.Fprintln(a.Out, "Cancelled.")
			return nil
		}
	}
	return client.Run(ctx, []string{"sudo", "shutdown", "-h", "now"}, true, false)
}

func (a *App) configCommand(path string, args []string) error {
	if len(args) != 1 || (args[0] != "init" && args[0] != "show") {
		return errors.New("usage: devnode config <init|show>")
	}
	if args[0] == "show" {
		fmt.Fprintln(a.Out, path)
		return nil
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config already exists: %s", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(config.Example), 0600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	fmt.Fprintln(a.Out, "Created", path)
	return nil
}

func noArgs(args []string, fn func() error) error {
	if len(args) != 0 {
		return errors.New("unexpected arguments")
	}
	return fn()
}
func needsTTY(args []string) bool {
	for _, arg := range args {
		if arg == "--tty" || arg == "-t" || (strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && strings.Contains(arg[1:], "t")) {
			return true
		}
	}
	return false
}

const usage = `DevNode controls a remote Ubuntu development server over SSH.

Usage:
  devnode [--config PATH] <command> [arguments]

Commands:
  setup                           Configure passwordless SSH (one-time)
  connect                         Open an interactive SSH session
  status                          Show the server dashboard
  monitor                         Open btop, htop, or top
  docker <arguments...>           Run a remote Docker command
  postgres <status|start|stop|restart>
  services                        List running systemd services
  logs [--follow] [--lines N] SERVICE
  exec <command> [arguments...]   Run an explicitly trusted command
  shutdown [--yes]                Shut down after confirmation
  config <init|show>              Create or locate the config file
  version                         Print the version
`
