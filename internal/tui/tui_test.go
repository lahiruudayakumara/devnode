package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/lahiruudayakumara/devnode/internal/status"
)

func TestViewAndNavigation(t *testing.T) {
	m := model{width: 100, height: 30, interval: 2 * time.Second, target: "dev@server", snapshot: status.Snapshot{Host: "server", IP: "192.168.1.17", OS: "Ubuntu", CPU: 40, Memory: 50, Disk: 20, Services: map[string]status.Service{"docker": {State: "active", Source: "systemd"}, "ros2": {State: "installed", Source: "native"}}}}
	view := m.View().Content
	for _, want := range []string{"DEVNODE", "ONLINE", "ESSENTIAL SERVICES", "ROS 2"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q", want)
		}
	}
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if next.(model).tab != 1 {
		t.Fatalf("tab = %d", next.(model).tab)
	}
}
