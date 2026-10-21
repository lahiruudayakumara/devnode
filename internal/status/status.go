package status

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
)

const scriptTemplate = `set -u
kv() { printf '%s=%s\n' "$1" "$2"; }
kv HOST "$(hostname)"
kv OS "$(. /etc/os-release 2>/dev/null; printf '%s' "${PRETTY_NAME:-Linux}")"
kv ARCH "$(uname -m)"
kv IP "$(hostname -I 2>/dev/null | awk '{print $1}')"
kv UPTIME "$(uptime -p 2>/dev/null || true)"
kv CPU_MODEL "$(lscpu 2>/dev/null | awk -F: '/Model name/ {gsub(/^[ \t]+/,"",$2); print $2; exit}')"
kv CPU "$(LC_ALL=C top -bn1 2>/dev/null | awk '/Cpu\(s\)/ {printf "%.0f", 100-$8; exit}')"
kv MEMORY "$(free 2>/dev/null | awk '/Mem:/ {printf "%.0f", $3*100/$2}')"
kv MEMORY_TOTAL "$(free -h 2>/dev/null | awk '/Mem:/ {print $2}')"
kv DISK "$(df -P / 2>/dev/null | awk 'NR==2 {gsub(/%/,"",$5); print $5}')"
kv DISK_TOTAL "$(df -hP / 2>/dev/null | awk 'NR==2 {print $2}')"

# Ensure common runtime directories are in PATH
for _p in "$HOME/.local/bin" "$HOME/.cargo/bin" /snap/bin /usr/local/bin; do
  if [ -d "$_p" ]; then
    case ":$PATH:" in
      *":$_p:"*) ;;
      *) PATH="$_p:$PATH" ;;
    esac
  fi
done
if [ -d "$HOME/.nvm/versions/node" ]; then
  _latest_node=$(ls "$HOME/.nvm/versions/node" 2>/dev/null | tail -n1 || true)
  if [ -n "$_latest_node" ] && [ -d "$HOME/.nvm/versions/node/$_latest_node/bin" ]; then
    PATH="$HOME/.nvm/versions/node/$_latest_node/bin:$PATH"
  fi
fi

if command -v node >/dev/null 2>&1; then
  _v=$(node -v 2>/dev/null || true); [ -n "$_v" ] && kv TOOL_node "$_v"
fi
if command -v npm >/dev/null 2>&1; then
  _v=$(npm -v 2>/dev/null || true); [ -n "$_v" ] && kv TOOL_npm "$_v"
fi
if command -v pnpm >/dev/null 2>&1; then
  _v=$(pnpm -v 2>/dev/null || true); [ -n "$_v" ] && kv TOOL_pnpm "$_v"
fi
if command -v yarn >/dev/null 2>&1; then
  _v=$(yarn -v 2>/dev/null || true); [ -n "$_v" ] && kv TOOL_yarn "$_v"
fi
if command -v bun >/dev/null 2>&1; then
  _v=$(bun -v 2>/dev/null || true); [ -n "$_v" ] && kv TOOL_bun "$_v"
fi
if command -v python3 >/dev/null 2>&1; then
  _v=$(python3 -V 2>&1 | awk '{print $2}'); [ -n "$_v" ] && kv TOOL_python "$_v"
elif command -v python >/dev/null 2>&1; then
  _v=$(python -V 2>&1 | awk '{print $2}'); [ -n "$_v" ] && kv TOOL_python "$_v"
fi
if command -v go >/dev/null 2>&1; then
  _v=$(go version 2>/dev/null | awk '{print $3}' | sed 's/^go//'); [ -n "$_v" ] && kv TOOL_go "$_v"
fi
if command -v rustc >/dev/null 2>&1; then
  _v=$(rustc --version 2>/dev/null | awk '{print $2}'); [ -n "$_v" ] && kv TOOL_rust "$_v"
fi
if command -v git >/dev/null 2>&1; then
  _v=$(git --version 2>/dev/null | awk '{print $3}'); [ -n "$_v" ] && kv TOOL_git "$_v"
fi
if command -v docker >/dev/null 2>&1; then
  _v=$(docker --version 2>/dev/null | awk '{gsub(/,/,""); print $3}'); [ -n "$_v" ] && kv TOOL_docker "$_v"
fi

check_service() {
  key=$1 container=$2 unit=$3
  if command -v docker >/dev/null 2>&1 && docker inspect "$container" >/dev/null 2>&1; then
    state=$(docker inspect -f '{{.State.Status}}' "$container" 2>/dev/null || printf unknown)
    kv "SERVICE_$key" "$state|docker"
    return
  fi
  load=$(systemctl show "$unit" -p LoadState --value 2>/dev/null || true)
  if [ -n "$load" ] && [ "$load" != not-found ]; then
    state=$(systemctl is-active "$unit" 2>/dev/null || true)
    [ -n "$state" ] || state=unknown
    kv "SERVICE_$key" "$state|systemd"
  else
    kv "SERVICE_$key" "not-installed|none"
  fi
}

check_service docker docker docker
check_service postgresql postgres __POSTGRES_SERVICE__
check_service redis redis redis-server
check_service mongodb mongodb mongod
check_service minio minio minio
check_service localstack localstack localstack
check_service mailpit mailpit mailpit
check_service portainer portainer portainer
check_service uptime-kuma uptime-kuma uptime-kuma
check_service dozzle dozzle dozzle
check_service mosquitto mosquitto mosquitto
check_service node-red node-red nodered
check_service netdata netdata netdata
check_service jupyter jupyter jupyter

if command -v ros2 >/dev/null 2>&1; then
  if ros2 daemon status 2>/dev/null | grep -qi running; then
    kv SERVICE_ros2 "running|native"
  else
    kv SERVICE_ros2 "installed|native"
  fi
else
  kv SERVICE_ros2 "not-installed|none"
fi

runner=$(systemctl list-unit-files 'actions.runner.*.service' --no-legend 2>/dev/null | awk 'NR==1 {print $1}')
if [ -n "$runner" ]; then
  state=$(systemctl is-active "$runner" 2>/dev/null || true)
  kv SERVICE_github-runner "${state:-unknown}|systemd"
else
  kv SERVICE_github-runner "not-installed|none"
fi`

func Script(postgresService string) string {
	return strings.ReplaceAll(scriptTemplate, "__POSTGRES_SERVICE__", postgresService)
}

type Service struct{ State, Source string }

type Snapshot struct {
	Host, OS, Arch, IP, Uptime, CPUModel, MemoryTotal, DiskTotal string
	CPU, Memory, Disk                                            int
	Services                                                     map[string]Service
	Tools                                                        map[string]string
}

// ToolItem defines a known development tool.
type ToolItem struct {
	Key   string
	Label string
}

// DefaultTools defines the display order and human labels for runtimes.
var DefaultTools = []ToolItem{
	{Key: "node", Label: "Node.js"},
	{Key: "npm", Label: "npm"},
	{Key: "pnpm", Label: "pnpm"},
	{Key: "yarn", Label: "Yarn"},
	{Key: "bun", Label: "Bun"},
	{Key: "python", Label: "Python"},
	{Key: "go", Label: "Go"},
	{Key: "rust", Label: "Rust"},
	{Key: "git", Label: "Git"},
	{Key: "docker", Label: "Docker"},
}

// ActiveTool holds a tool's resolved display name and detected version.
type ActiveTool struct {
	Key     string
	Label   string
	Version string
}

// DisplayTools resolves tools to display. Node, npm, and pnpm are always included
// (showing missing if not installed), while other tools appear if detected.
func DisplayTools(tools map[string]string) []ActiveTool {
	var result []ActiveTool
	always := map[string]bool{"node": true, "npm": true, "pnpm": true}
	for _, dt := range DefaultTools {
		v, ok := tools[dt.Key]
		if ok && v != "" {
			result = append(result, ActiveTool{Key: dt.Key, Label: dt.Label, Version: v})
		} else if always[dt.Key] {
			result = append(result, ActiveTool{Key: dt.Key, Label: dt.Label, Version: ""})
		}
	}
	known := map[string]bool{}
	for _, dt := range DefaultTools {
		known[dt.Key] = true
	}
	for k, v := range tools {
		if !known[k] && v != "" {
			label := strings.ToUpper(k[:1]) + k[1:]
			result = append(result, ActiveTool{Key: k, Label: label, Version: v})
		}
	}
	return result
}

func Parse(data string) (Snapshot, error) {
	s := Snapshot{
		CPU:      -1,
		Memory:   -1,
		Disk:     -1,
		Services: map[string]Service{},
		Tools:    map[string]string{},
	}
	scanner := bufio.NewScanner(strings.NewReader(data))
	for scanner.Scan() {
		key, value, ok := strings.Cut(scanner.Text(), "=")
		if !ok {
			continue
		}
		switch key {
		case "HOST":
			s.Host = value
		case "OS":
			s.OS = value
		case "ARCH":
			s.Arch = value
		case "IP":
			s.IP = value
		case "UPTIME":
			s.Uptime = strings.TrimPrefix(value, "up ")
		case "CPU_MODEL":
			s.CPUModel = value
		case "CPU":
			s.CPU = percent(value)
		case "MEMORY":
			s.Memory = percent(value)
		case "MEMORY_TOTAL":
			s.MemoryTotal = value
		case "DISK":
			s.Disk = percent(value)
		case "DISK_TOTAL":
			s.DiskTotal = value
		default:
			if name, ok := strings.CutPrefix(key, "SERVICE_"); ok {
				state, source, _ := strings.Cut(value, "|")
				s.Services[name] = Service{State: state, Source: source}
			} else if name, ok := strings.CutPrefix(key, "TOOL_"); ok {
				s.Tools[name] = strings.TrimSpace(value)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return s, err
	}
	if s.Host == "" {
		return s, fmt.Errorf("server returned an incomplete status response")
	}
	return s, nil
}

func percent(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 0 || n > 100 {
		return -1
	}
	return n
}
