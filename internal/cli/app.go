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
	"time"

	"github.com/lahiruudayakumara/devnode/internal/catalog"
	"github.com/lahiruudayakumara/devnode/internal/config"
	devssh "github.com/lahiruudayakumara/devnode/internal/ssh"
	"github.com/lahiruudayakumara/devnode/internal/status"
	devtui "github.com/lahiruudayakumara/devnode/internal/tui"
	"github.com/lahiruudayakumara/devnode/internal/ui"
)

const Version = "0.4.0"

const hardwareScript = `printf '%s\n' '========== CPU =========='; lscpu | grep -E 'Model name|CPU\(s\)|Architecture'; printf '%s\n' '========== RAM =========='; free -h; printf '%s\n' '========== STORAGE =========='; lsblk -o NAME,SIZE,TYPE,FSTYPE,MOUNTPOINTS; printf '%s\n' '========== DISK SPACE =========='; df -h /; printf '%s\n' '========== GPU =========='; lspci | grep -Ei 'vga|3d|display' || printf '%s\n' 'Not detected'; printf '%s\n' '========== DOCKER =========='; docker --version 2>/dev/null || printf '%s\n' 'Not installed'`

const networkScript = `printf '%s\n' '========== ADDRESSES =========='; hostname -I; ip -brief address; printf '%s\n' '========== ROUTES =========='; ip route; printf '%s\n' '========== LISTENING PORTS =========='; ss -lntup 2>/dev/null || ss -lnt 2>/dev/null; printf '%s\n' '========== DNS =========='; if command -v resolvectl >/dev/null 2>&1; then resolvectl status | sed -n '1,80p'; else cat /etc/resolv.conf; fi`

const projectsScript = `printf '%s\n' 'Git repositories:'; found=0; for root in "$HOME/projects" "$HOME/dev" "$HOME/workspace"; do [ -d "$root" ] || continue; find "$root" -maxdepth 4 -type d -name .git -print 2>/dev/null | sed 's#/.git$##'; found=1; done; [ "$found" -eq 1 ] || printf '%s\n' 'No standard project directories found.'`

const doctorScript = `printf '%-20s %s\n' CHECK RESULT; for cmd in ssh systemctl docker git curl; do if command -v "$cmd" >/dev/null 2>&1; then printf '%-20s %s\n' "$cmd" OK; else printf '%-20s %s\n' "$cmd" MISSING; fi; done; if docker info >/dev/null 2>&1; then printf '%-20s %s\n' docker-access OK; else printf '%-20s %s\n' docker-access UNAVAILABLE; fi; printf '%-20s %s\n' disk-root "$(df -hP / | awk 'NR==2 {print $5 " used"}')"; printf '%-20s %s\n' memory "$(free -h | awk '/Mem:/ {print $3 " / " $2}')"`

const serviceScript = `container=$1 unit=$2 action=$3
if command -v docker >/dev/null 2>&1 && docker inspect "$container" >/dev/null 2>&1; then
  printf 'manager: docker\nservice: %s\n' "$container"
  if [ "$action" = status ]; then docker inspect -f 'state: {{.State.Status}}' "$container"; else docker "$action" "$container"; fi
  exit
fi
load=$(systemctl show "$unit" -p LoadState --value 2>/dev/null || true)
if [ -z "$load" ] || [ "$load" = not-found ]; then printf 'Service %s is not installed as Docker or systemd.\n' "$container" >&2; exit 3; fi
printf 'manager: systemd\nservice: %s\n' "$unit"
if [ "$action" = status ]; then systemctl status "$unit" --no-pager; else sudo systemctl "$action" "$unit"; fi`

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
	case "status", "dashboard":
		return a.status(ctx, client, cfg, args[1:])
	case "monitor":
		return noArgs(args[1:], func() error {
			return client.Run(ctx, []string{"sh", "-c", "command -v btop >/dev/null && exec btop; command -v htop >/dev/null && exec htop; exec top"}, true, false)
		})
	case "docker":
		if len(args) < 2 {
			return errors.New("usage: devnode docker <arguments...>")
		}
		return client.Run(ctx, append([]string{"docker"}, args[1:]...), needsTTY(args[1:]) && devssh.IsTerminal(a.Out), false)
	case "hardware":
		return noArgs(args[1:], func() error { return client.Run(ctx, []string{"sh", "-c", hardwareScript}, false, false) })
	case "network":
		return noArgs(args[1:], func() error { return client.Run(ctx, []string{"sh", "-c", networkScript}, false, false) })
	case "projects":
		return noArgs(args[1:], func() error { return client.Run(ctx, []string{"sh", "-c", projectsScript}, false, false) })
	case "doctor":
		return noArgs(args[1:], func() error { return client.Run(ctx, []string{"sh", "-c", doctorScript}, false, false) })
	case "ros":
		return a.ros(ctx, client, args[1:])
	case "install":
		return a.install(ctx, client, cfg, args[1:])
	case "tunnel":
		return a.tunnel(ctx, client, args[1:])
	case "service":
		if len(args) != 3 || !config.SafeUnit(args[1]) || !validAction(args[2]) {
			return errors.New("usage: devnode service <name> <status|start|stop|restart>")
		}
		return a.managedService(ctx, client, args[1], args[1], args[2])
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

func (a *App) status(ctx context.Context, client devssh.Client, cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	once := fs.Bool("once", false, "render once and exit")
	interval := fs.Duration("interval", 2*time.Second, "live refresh interval")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: devnode status [--once] [--interval 2s]")
	}
	if *interval < 500*time.Millisecond || *interval > time.Minute {
		return errors.New("--interval must be between 500ms and 1m")
	}
	if !*once && devssh.IsTerminal(a.Out) {
		return devtui.Run(ctx, client, cfg, *interval, a.In, a.Out)
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
}

func (a *App) install(ctx context.Context, client devssh.Client, cfg config.Config, args []string) error {
	if len(args) > 1 {
		return errors.New("usage: devnode install [service]")
	}
	name := ""
	if len(args) == 1 {
		name = args[0]
	}
	if name == "all" {
		return errors.New("bulk installation is intentionally unsupported; choose one service")
	}
	out, err := client.Output(ctx, status.Script(cfg.PostgresService))
	if err != nil {
		return err
	}
	snapshot, err := status.Parse(string(out))
	if err != nil {
		return err
	}
	reader := bufio.NewReader(a.In)

	if name == "" {
		missing := make([]catalog.Service, 0)
		for _, candidate := range catalog.Names() {
			svc, _ := catalog.Get(candidate)
			state := snapshot.Services[candidate].State
			if state == "" || state == "not-installed" {
				missing = append(missing, svc)
			}
		}
		if len(missing) == 0 {
			fmt.Fprintln(a.Out, "All catalog services are already installed.")
			return nil
		}
		fmt.Fprintln(a.Out, "Missing services (choose one):")
		for i, svc := range missing {
			fmt.Fprintf(a.Out, "  %2d) %-18s %-16s %s\n", i+1, svc.Name, "["+svc.Method+"]", svc.Description)
		}
		fmt.Fprint(a.Out, "Selection number or name (blank cancels): ")
		answer, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		answer = strings.TrimSpace(answer)
		if answer == "" {
			fmt.Fprintln(a.Out, "Cancelled.")
			return nil
		}
		if n, err := strconv.Atoi(answer); err == nil {
			if n < 1 || n > len(missing) {
				return errors.New("selection is out of range")
			}
			name = missing[n-1].Name
		} else {
			name = answer
		}
	}

	svc, ok := catalog.Get(name)
	if !ok {
		return fmt.Errorf("unknown installable service %q", name)
	}
	if state := snapshot.Services[name].State; state != "" && state != "not-installed" {
		return fmt.Errorf("%s is already installed (state: %s)", svc.Label, state)
	}
	fmt.Fprintf(a.Out, "\n%s\nMethod: %s\nChange: %s\n", svc.Label, svc.Method, svc.Description)
	if svc.Script == "" {
		fmt.Fprintf(a.Out, "\nWhy guided? %s\n", svc.Manual)
		if len(svc.Steps) > 0 {
			fmt.Fprintf(a.Out, "\nRun these commands on the server (use `devnode connect` to open a shell):\n\n")
			for _, step := range svc.Steps {
				if step == "" {
					fmt.Fprintln(a.Out)
				} else {
					fmt.Fprintf(a.Out, "  %s\n", step)
				}
			}
			fmt.Fprintln(a.Out)
		}
		fmt.Fprintf(a.Out, "Once installed, run `devnode status` to confirm.\n")
		return nil
	}
	fmt.Fprintf(a.Out, "Type %q to install only this service: ", name)
	answer, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	if strings.TrimSpace(answer) != name {
		fmt.Fprintln(a.Out, "Cancelled.")
		return nil
	}
	if err := client.Run(ctx, []string{"sh", "-c", svc.Script}, true, false); err != nil {
		return fmt.Errorf("install %s: %w", name, err)
	}
	fmt.Fprintf(a.Out, "%s installation completed. Check it with `devnode status`.\n", svc.Label)
	return nil
}

func (a *App) ros(ctx context.Context, client devssh.Client, args []string) error {
	if len(args) != 1 || (args[0] != "status" && args[0] != "nodes" && args[0] != "topics") {
		return errors.New("usage: devnode ros <status|nodes|topics>")
	}
	var script string
	switch args[0] {
	case "status":
		script = `if ! command -v ros2 >/dev/null 2>&1; then printf '%s\n' 'ROS 2 is not installed.'; exit 3; fi; printf 'ROS_DISTRO=%s\n' "${ROS_DISTRO:-unknown}"; ros2 daemon status`
	case "nodes":
		script = `command -v ros2 >/dev/null 2>&1 || { printf '%s\n' 'ROS 2 is not installed.' >&2; exit 3; }; ros2 node list`
	case "topics":
		script = `command -v ros2 >/dev/null 2>&1 || { printf '%s\n' 'ROS 2 is not installed.' >&2; exit 3; }; ros2 topic list`
	}
	return client.Run(ctx, []string{"sh", "-c", script}, false, false)
}

func (a *App) tunnel(ctx context.Context, client devssh.Client, args []string) error {
	if len(args) < 1 || len(args) > 2 {
		return errors.New("usage: devnode tunnel <service> [local-port]")
	}
	remotePort, ok := tunnelPorts[args[0]]
	if !ok {
		return fmt.Errorf("unknown tunnel %q; available: %s", args[0], tunnelNames())
	}
	localPort := remotePort
	if len(args) == 2 {
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 1 || n > 65535 {
			return errors.New("local port must be between 1 and 65535")
		}
		localPort = n
	}
	fmt.Fprintf(a.Out, "Forwarding 127.0.0.1:%d to %s:%d. Press Ctrl-C to stop.\n", localPort, args[0], remotePort)
	return client.Forward(ctx, localPort, remotePort)
}

var tunnelPorts = map[string]int{"postgres": 5432, "postgresql": 5432, "mysql": 3306, "redis": 6379, "mongodb": 27017, "adminer": 8080, "pgadmin": 5050, "minio": 9000, "mailpit": 8025, "portainer": 9443, "uptime-kuma": 3001, "netdata": 19999, "dozzle": 8080, "node-red": 1880, "localstack": 4566, "mosquitto": 1883}

func tunnelNames() string {
	names := []string{"adminer", "dozzle", "localstack", "mailpit", "minio", "mongodb", "mosquitto", "mysql", "netdata", "node-red", "pgadmin", "portainer", "postgres", "redis", "uptime-kuma"}
	return strings.Join(names, ", ")
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
	return a.managedService(ctx, client, "postgres", service, args[0])
}

func (a *App) managedService(ctx context.Context, client devssh.Client, container, unit, action string) error {
	tty := action != "status" && devssh.IsTerminal(a.Out)
	return client.Run(ctx, []string{"sh", "-c", serviceScript, "devnode-service", container, unit, action}, tty, false)
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

func validAction(action string) bool {
	return action == "status" || action == "start" || action == "stop" || action == "restart"
}

const usage = `DevNode controls a remote Ubuntu development server over SSH.

Usage:
  devnode [--config PATH] <command> [arguments]

Commands:
  setup                           Configure passwordless SSH (one-time)
  connect                         Open an interactive SSH session
  status, dashboard               Open the live server dashboard
  hardware                        Show CPU, RAM, storage, GPU, and Docker
  network                         Show addresses, routes, ports, and DNS
  projects                        Find Git projects in standard directories
  doctor                          Check server development prerequisites
  ros <status|nodes|topics>       Inspect the remote ROS 2 environment
  install [service]               Choose and install one missing service
  tunnel SERVICE [LOCAL_PORT]     Securely forward a service to your Mac
  monitor                         Open btop, htop, or top
  docker <arguments...>           Run a remote Docker command
  service NAME ACTION             Control Docker or systemd automatically
  postgres <status|start|stop|restart>
  services                        List running systemd services
  logs [--follow] [--lines N] SERVICE
  exec <command> [arguments...]   Run an explicitly trusted command
  shutdown [--yes]                Shut down after confirmation
  config <init|show>              Create or locate the config file
  version                         Print the version
`
