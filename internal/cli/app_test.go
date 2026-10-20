package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHelpAndVersion(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"help"}} {
		var out, errOut bytes.Buffer
		if err := New(strings.NewReader(""), &out, &errOut).Run(context.Background(), args); err != nil {
			t.Fatalf("Run(%v): %v", args, err)
		}
		if !strings.Contains(out.String(), "DevNode controls") {
			t.Fatalf("missing help for %v", args)
		}
	}
	var out bytes.Buffer
	if err := New(strings.NewReader(""), &out, &bytes.Buffer{}).Run(context.Background(), []string{"--version"}); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != "devnode "+Version {
		t.Fatalf("unexpected version: %q", out.String())
	}
}

func TestConfigInit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.yaml")
	var out bytes.Buffer
	app := New(strings.NewReader(""), &out, &bytes.Buffer{})
	if err := app.Run(context.Background(), []string{"--config", path, "config", "init"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
	if err := app.Run(context.Background(), []string{"--config", path, "config", "init"}); err == nil {
		t.Fatal("expected existing config error")
	}
}

func TestNeedsTTY(t *testing.T) {
	if !needsTTY([]string{"exec", "-it", "container", "sh"}) {
		t.Fatal("expected -it to request tty")
	}
	if needsTTY([]string{"run", "--rm", "alpine"}) {
		t.Fatal("unexpected tty")
	}
}
