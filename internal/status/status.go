package status

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
)

const scriptTemplate = `set -eu
kv() { printf '%s=%s\n' "$1" "$2"; }
kv HOST "$(hostname)"
kv OS "$(. /etc/os-release; printf '%s' "$PRETTY_NAME")"
kv ARCH "$(uname -m)"
kv UPTIME "$(uptime -p 2>/dev/null || true)"
kv CPU "$(LC_ALL=C top -bn1 | awk '/Cpu\(s\)/ {printf "%.0f", 100-$8; exit}' 2>/dev/null || printf '?')"
kv MEMORY "$(free | awk '/Mem:/ {printf "%.0f", $3*100/$2}')"
kv DISK "$(df -P / | awk 'NR==2 {gsub(/%/,"",$5); print $5}')"
for entry in docker:docker postgresql:__POSTGRES_SERVICE__ redis:redis-server nginx:nginx; do
  name=${entry%%:*}
  svc=${entry#*:}
  state=$(systemctl is-active "$svc" 2>/dev/null || true)
  [ -n "$state" ] || state=unavailable
  kv "SERVICE_$name" "$state"
done`

func Script(postgresService string) string {
	return strings.ReplaceAll(scriptTemplate, "__POSTGRES_SERVICE__", postgresService)
}

type Snapshot struct {
	Host, OS, Arch, Uptime string
	CPU, Memory, Disk      int
	Services               map[string]string
}

func Parse(data string) (Snapshot, error) {
	s := Snapshot{CPU: -1, Memory: -1, Disk: -1, Services: map[string]string{}}
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
		case "UPTIME":
			s.Uptime = strings.TrimPrefix(value, "up ")
		case "CPU":
			s.CPU = percent(value)
		case "MEMORY":
			s.Memory = percent(value)
		case "DISK":
			s.Disk = percent(value)
		default:
			if name, ok := strings.CutPrefix(key, "SERVICE_"); ok {
				s.Services[name] = value
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
