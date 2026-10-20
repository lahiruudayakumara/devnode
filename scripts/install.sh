#!/bin/sh
set -eu

repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
install_dir=${DEVNODE_INSTALL_DIR:-"$HOME/.local/bin"}
config_path=${DEVNODE_CONFIG:-"$HOME/.config/devnode/config.yaml"}
server_host=${DEVNODE_HOST:-"192.168.x.x"}
server_user=${DEVNODE_USER:-"your-username"}
server_port=${DEVNODE_PORT:-"22"}

mkdir -p "$install_dir"
cd "$repo_dir"
go build -trimpath -ldflags "-s -w" -o "$install_dir/devnode" ./cmd/devnode
printf 'Installed %s/devnode\n' "$install_dir"

if [ ! -e "$config_path" ]; then
  config_dir=$(dirname -- "$config_path")
  mkdir -p "$config_dir"
  chmod 700 "$config_dir"
  umask 077
  {
    printf '%s\n' '# DevNode configuration'
    printf 'host: %s\n' "$server_host"
    printf 'user: %s\n' "$server_user"
    printf 'port: %s\n' "$server_port"
    printf '%s\n' 'identity_file: ""'
    printf '%s\n' 'connect_timeout: 8'
    printf '%s\n' 'postgres_service: postgresql'
  } > "$config_path"
  printf 'Created %s for %s@%s\n' "$config_path" "$server_user" "$server_host"
else
  printf 'Preserved existing configuration at %s\n' "$config_path"
fi

if [ "$install_dir" = "$HOME/.local/bin" ]; then
  zshrc="$HOME/.zshrc"
  path_line='export PATH="$HOME/.local/bin:$PATH"'
  if [ ! -f "$zshrc" ] || ! grep -Fqx "$path_line" "$zshrc"; then
    printf '\n%s\n' "$path_line" >> "$zshrc"
    printf 'Added ~/.local/bin to PATH in %s\n' "$zshrc"
  else
    printf 'PATH is already configured in %s\n' "$zshrc"
  fi
fi

printf '\nSetup complete. Open a new terminal, then run:\n  devnode status\n'

