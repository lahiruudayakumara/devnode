package ui

import (
	"fmt"
	"io"
	"strings"

	"github.com/lahiruudayakumara/devnode/internal/status"
)

type serviceItem struct{ label, key string }

func Dashboard(w io.Writer, target string, s status.Snapshot, color bool) {
	const width = 78
	line := func(text string) { fmt.Fprintf(w, "│ %-76s │\n", truncate(text, 76)) }
	section := func(name string) {
		fmt.Fprintln(w, "├"+strings.Repeat("─", width)+"┤")
		line("  " + name)
		line("")
	}
	fmt.Fprintln(w, "╭"+strings.Repeat("─", width)+"╮")
	line("                 DEVNODE SERVER")
	section("SYSTEM")
	line(fmt.Sprintf("  Connection    %s Online", dot(true, color)))
	line(fmt.Sprintf("  Target        %s", target))
	line(fmt.Sprintf("  Host / IP     %s / %s", fallback(s.Host), fallback(s.IP)))
	line(fmt.Sprintf("  OS / Arch     %s / %s", fallback(s.OS), fallback(s.Arch)))
	line(fmt.Sprintf("  Uptime        %s", fallback(s.Uptime)))
	line(fmt.Sprintf("  CPU model     %s", fallback(s.CPUModel)))
	line(metric("CPU", s.CPU, ""))
	line(metric("Memory", s.Memory, s.MemoryTotal))
	line(metric("Disk", s.Disk, s.DiskTotal))

	groups := []struct {
		title    string
		services []serviceItem
	}{
		{"CORE & DATA", []serviceItem{{"Docker", "docker"}, {"PostgreSQL", "postgresql"}, {"Redis", "redis"}, {"MongoDB", "mongodb"}, {"MinIO", "minio"}, {"LocalStack", "localstack"}, {"Mailpit", "mailpit"}}},
		{"DEVOPS & MONITORING", []serviceItem{{"GitHub Runner", "github-runner"}, {"Portainer", "portainer"}, {"Uptime Kuma", "uptime-kuma"}, {"Dozzle", "dozzle"}, {"Netdata", "netdata"}}},
		{"IOT & ROBOTICS", []serviceItem{{"MQTT Broker", "mosquitto"}, {"Node-RED", "node-red"}, {"ROS 2", "ros2"}}},
	}
	for _, group := range groups {
		section(group.title)
		for i := 0; i < len(group.services); i += 2 {
			left := serviceCell(group.services[i].label, s.Services[group.services[i].key], color)
			right := ""
			if i+1 < len(group.services) {
				right = serviceCell(group.services[i+1].label, s.Services[group.services[i+1].key], color)
			}
			line("  " + left + "  " + right)
		}
	}
	line("")
	fmt.Fprintln(w, "╰"+strings.Repeat("─", width)+"╯")
}

func metric(label string, value int, total string) string {
	if value < 0 {
		return fmt.Sprintf("  %-13s unavailable", label)
	}
	filled := value / 10
	suffix := ""
	if total != "" {
		suffix = " of " + total
	}
	return fmt.Sprintf("  %-13s %s%s  %3d%%%s", label, strings.Repeat("█", filled), strings.Repeat("░", 10-filled), value, suffix)
}

func serviceCell(label string, svc status.Service, color bool) string {
	state := svc.State
	if state == "" {
		state = "unknown"
	}
	running := state == "active" || state == "running"
	source := svc.Source
	if source == "none" || source == "" {
		source = "—"
	}
	stateLabel := title(state)
	if state == "not-installed" {
		stateLabel = "Missing"
	}
	return fmt.Sprintf("%-13s %s %-10s %-8s", truncate(label, 13), dot(running, color), truncate(stateLabel, 10), "["+source+"]")
}

func dot(ok, color bool) string {
	if ok {
		if color {
			return "\x1b[32m●\x1b[0m"
		}
		return "●"
	}
	if color {
		return "\x1b[31m○\x1b[0m"
	}
	return "○"
}
func title(s string) string {
	if s == "" {
		return "Unknown"
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
func fallback(s string) string {
	if s == "" {
		return "unavailable"
	}
	return s
}
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
