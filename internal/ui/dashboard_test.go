package ui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/lahiruudayakumara/devnode/internal/status"
)

func TestDashboardShowsManagerAndInventory(t *testing.T) {
	s := status.Snapshot{
		Host: "devbox", IP: "192.168.1.17", OS: "Ubuntu", Arch: "x86_64",
		CPU: 40, Memory: 50, Disk: 30,
		Services: map[string]status.Service{
			"postgresql": {State: "running", Source: "docker"},
			"redis":      {State: "inactive", Source: "systemd"},
		},
	}
	var out bytes.Buffer
	Dashboard(&out, "dev@devbox", s, false)
	got := out.String()
	for _, want := range []string{"DEVNODE SERVER", "PostgreSQL", "Running", "[docker]", "[systemd]", "IOT & ROBOTICS", "ROS 2"} {
		if !strings.Contains(got, want) {
			t.Errorf("dashboard missing %q\n%s", want, got)
		}
	}
}
