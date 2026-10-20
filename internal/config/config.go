package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const Example = `# DevNode configuration
host: 192.168.1.17
user: lahiru
port: 22
identity_file: ""
connect_timeout: 8
postgres_service: postgresql
`

type Config struct {
	Host            string
	User            string
	Port            int
	IdentityFile    string
	ConnectTimeout  int
	PostgresService string
}

func Defaults() Config {
	return Config{Port: 22, ConnectTimeout: 8, PostgresService: "postgresql"}
}

func DefaultPath() string {
	if p := os.Getenv("DEVNODE_CONFIG"); p != "" {
		return p
	}
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "devnode", "config.yaml")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "devnode.yaml"
	}
	return filepath.Join(home, ".config", "devnode", "config.yaml")
}

func Save(path string, c Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	data := fmt.Sprintf("# DevNode configuration\nhost: %s\nuser: %s\nport: %d\nidentity_file: %s\nconnect_timeout: %d\npostgres_service: %s\n",
		c.Host, c.User, c.Port, strconv.Quote(c.IdentityFile), c.ConnectTimeout, c.PostgresService)
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.WriteString(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close config: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	ok = true
	return nil
}

// Load reads the intentionally small YAML subset used by DevNode. Unknown keys
// are rejected so misspelled security-sensitive settings cannot go unnoticed.
func Load(path string) (Config, error) {
	cfg := Defaults()
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := applyEnv(&cfg); err != nil {
			return cfg, err
		}
		return cfg, cfg.Validate()
	}
	if err != nil {
		return cfg, fmt.Errorf("open config: %w", err)
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for line := 1; s.Scan(); line++ {
		raw := strings.TrimSpace(s.Text())
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		key, value, ok := strings.Cut(raw, ":")
		if !ok {
			return cfg, fmt.Errorf("%s:%d: expected key: value", path, line)
		}
		key, value = strings.TrimSpace(key), trimValue(value)
		switch key {
		case "host":
			cfg.Host = value
		case "user":
			cfg.User = value
		case "port":
			cfg.Port, err = parseInt(path, line, key, value)
		case "identity_file":
			cfg.IdentityFile = expandHome(value)
		case "connect_timeout":
			cfg.ConnectTimeout, err = parseInt(path, line, key, value)
		case "postgres_service":
			cfg.PostgresService = value
		default:
			return cfg, fmt.Errorf("%s:%d: unknown setting %q", path, line, key)
		}
		if err != nil {
			return cfg, err
		}
	}
	if err := s.Err(); err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}
	if err := applyEnv(&cfg); err != nil {
		return cfg, err
	}
	return cfg, cfg.Validate()
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Host) == "" {
		return errors.New("server host is not configured; run `devnode config init` or set DEVNODE_HOST")
	}
	if strings.HasPrefix(c.Host, "-") || strings.ContainsAny(c.Host, " \t\r\n") {
		return fmt.Errorf("invalid SSH host %q", c.Host)
	}
	if c.User != "" && !safeSSHUser(c.User) {
		return fmt.Errorf("invalid SSH user %q", c.User)
	}
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("invalid SSH port %d", c.Port)
	}
	if c.ConnectTimeout < 1 || c.ConnectTimeout > 300 {
		return fmt.Errorf("connect_timeout must be between 1 and 300 seconds")
	}
	if !SafeUnit(c.PostgresService) {
		return fmt.Errorf("invalid postgres_service %q", c.PostgresService)
	}
	return nil
}

func SafeUnit(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("@_.:-", r)) {
			return false
		}
	}
	return true
}

func Target(c Config) string {
	if c.User == "" {
		return c.Host
	}
	return c.User + "@" + c.Host
}

func applyEnv(c *Config) error {
	if v := os.Getenv("DEVNODE_HOST"); v != "" {
		c.Host = v
	}
	if v := os.Getenv("DEVNODE_USER"); v != "" {
		c.User = v
	}
	if v := os.Getenv("DEVNODE_IDENTITY_FILE"); v != "" {
		c.IdentityFile = expandHome(v)
	}
	if v := os.Getenv("DEVNODE_PORT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("DEVNODE_PORT must be a number")
		}
		c.Port = n
	}
	return nil
}

func safeSSHUser(s string) bool {
	if s == "" || strings.HasPrefix(s, "-") {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("_.-", r)) {
			return false
		}
	}
	return true
}

func parseInt(path string, line int, key, value string) (int, error) {
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s:%d: %s must be a number", path, line, key)
	}
	return n, nil
}

func trimValue(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		return s[1 : len(s)-1]
	}
	if before, _, ok := strings.Cut(s, " #"); ok {
		return strings.TrimSpace(before)
	}
	return s
}

func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}
