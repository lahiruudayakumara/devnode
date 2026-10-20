# DevNode

DevNode is a small, dependency-free macOS CLI for managing an Ubuntu development server over SSH. It uses the system `ssh` client, so your existing SSH agent, `~/.ssh/config`, host-key verification, and hardware-backed keys continue to work.

## Requirements

- macOS with OpenSSH (included by default)
- Go 1.23 or newer to build
- An Ubuntu host with `sshd` running and key-based login configured
- `sudo` rights on Ubuntu for PostgreSQL control and shutdown

## Install

```sh
./scripts/install.sh
```

The installer performs the local setup automatically:

- Builds and installs `~/.local/bin/devnode`
- Creates `~/.config/devnode/config.yaml` for `lahiru@192.168.1.17`
- Adds `~/.local/bin` to `~/.zshrc` once
- Preserves an existing configuration on later runs

Open a new terminal and run the one-time SSH setup, followed by the dashboard:

```sh
devnode setup
devnode status
```

`devnode setup` generates a dedicated key at `~/.ssh/devnode_ed25519`, installs its public half on Ubuntu, and saves its path in the configuration. Ubuntu may ask for your account password once. DevNode never stores that password. Verify the server's host-key fingerprint before accepting it.

To install with different connection details without editing a file:

```sh
DEVNODE_HOST=server.local DEVNODE_USER=myuser DEVNODE_PORT=22 ./scripts/install.sh
```

## Commands

```text
devnode dashboard
devnode status
devnode status --interval 5s
devnode status --once
devnode hardware
devnode network
devnode doctor
devnode projects
devnode ros status
devnode ros nodes
devnode ros topics
devnode install
devnode install postgresql
devnode connect
devnode monitor
devnode docker ps
devnode docker stats
devnode service redis status
devnode service redis restart
devnode postgres status
devnode postgres start
devnode services
devnode logs --lines 200 nginx
devnode logs --follow nginx
devnode tunnel postgres
devnode tunnel portainer 9443
devnode exec uptime
devnode shutdown
```

The dashboard is a full-screen interactive terminal UI. It refreshes every two seconds, adapts to terminal resizing, and provides Overview, Services, and Help views. Use Tab or arrow keys to navigate, `r` to refresh, `+`/`-` to change the interval, and `q` to exit. `--interval 5s` sets the initial rate; `--once` produces a non-interactive snapshot for scripts. Redirected or piped output also renders only once. The dashboard labels services `[docker]`, `[systemd]`, `[native]`, or missing.

ROS 2 appears under **IoT & Robotics** as running, installed, or not installed. The `ros` command reports the daemon state and lists currently discoverable nodes or topics without installing or changing ROS.

`devnode install` discovers missing services and presents a one-at-a-time chooser. There is deliberately no `all` option. Before changing Ubuntu it shows the installation method and requires you to type the selected service name. PostgreSQL, Redis, Mosquitto, Docker, and selected localhost-only Docker services have automatic recipes. Services requiring credentials, vendor scripts, or compatibility decisions provide guided instructions instead.

Named tunnels bind only to your Mac's loopback address. For example, `devnode tunnel postgres` makes a PostgreSQL server bound to Ubuntu's `127.0.0.1:5432` available temporarily at `127.0.0.1:5432` on the Mac. Keep the command running and press Ctrl-C when finished. Supported names include PostgreSQL, MySQL, Redis, MongoDB, Adminer, pgAdmin, MinIO, Mailpit, Portainer, Uptime Kuma, and Netdata.

Use `devnode --config /path/to/config.yaml status` for another server. For automation, `DEVNODE_HOST`, `DEVNODE_USER`, `DEVNODE_PORT`, `DEVNODE_IDENTITY_FILE`, and `DEVNODE_CONFIG` override file settings.

`devnode exec` intentionally runs the exact command and arguments you provide. Other commands quote remote arguments and validate systemd unit names before invoking SSH.

## Ubuntu setup

Install and enable SSH, then copy your Mac's public key to the server:

```sh
sudo apt install openssh-server
sudo systemctl enable --now ssh
```

For unattended `postgres` and `shutdown` operations, add narrowly scoped `sudoers` rules with `visudo`; do not grant blanket passwordless sudo. Interactive commands can request your sudo password through the allocated terminal.

Reserve the server address in your router's DHCP settings or use a stable local DNS hostname. If the laptop runs with its lid closed, configure systemd-logind appropriately and provide adequate ventilation.

## Development

```sh
make test
make vet
make build
```

The status collector emits a small `KEY=value` protocol on Ubuntu and parses it locally. The CLI layer depends on narrow `config`, `ssh`, `status`, and `ui` packages, keeping command execution and rendering independently testable.

