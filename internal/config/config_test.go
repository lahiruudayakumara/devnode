package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	t.Setenv("DEVNODE_HOST", "")
	p := filepath.Join(t.TempDir(), "config.yaml")
	err := os.WriteFile(p, []byte("host: server.local\nuser: dev\nport: 2222\nidentity_file: ''\nconnect_timeout: 12\npostgres_service: postgresql@16-main\n"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if Target(c) != "dev@server.local" || c.Port != 2222 || c.PostgresService != "postgresql@16-main" {
		t.Fatalf("unexpected config: %+v", c)
	}
}

func TestLoadRejectsUnknownKey(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte("host: server\npassword: secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p); err == nil {
		t.Fatal("expected unknown key error")
	}
}

func TestSafeUnit(t *testing.T) {
	for _, v := range []string{"nginx", "postgresql@16-main", "foo.service"} {
		if !SafeUnit(v) {
			t.Errorf("expected %q safe", v)
		}
	}
	for _, v := range []string{"", "nginx; reboot", "../unit", "unit name"} {
		if SafeUnit(v) {
			t.Errorf("expected %q unsafe", v)
		}
	}
}

func TestLoadRejectsInvalidEnvironmentPort(t *testing.T) {
	t.Setenv("DEVNODE_HOST", "server.local")
	t.Setenv("DEVNODE_PORT", "ssh")
	if _, err := Load(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Fatal("expected invalid environment port error")
	}
}

func TestValidateRejectsOptionLikeHost(t *testing.T) {
	c := Defaults()
	c.Host = "-oProxyCommand=bad"
	if err := c.Validate(); err == nil {
		t.Fatal("expected invalid host error")
	}
}

func TestSaveRoundTrip(t *testing.T) {
	t.Setenv("DEVNODE_HOST", "")
	t.Setenv("DEVNODE_PORT", "")
	p := filepath.Join(t.TempDir(), "nested", "config.yaml")
	want := Defaults()
	want.Host, want.User, want.IdentityFile = "server.local", "dev", "/tmp/key with spaces"
	if err := Save(p, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
}
