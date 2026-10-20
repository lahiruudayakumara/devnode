package ui

import (
	"fmt"
	"io"
	"strings"

	"github.com/lahiruudayakumara/devnode/internal/status"
)

type serviceItem struct{ label, key string }

func Dashboard(w io.Writer, target string, s status.Snapshot, color bool) {
	const width = 58
	line := func(text string) { fmt.Fprintf(w, "│ %-56s │\n", truncate(text, 56)) }
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
		for _, item := range group.services {
			line(serviceLine(item.label, s.Services[item.key], color))
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

func serviceLine(label string, svc status.Service, color bool) string {
	state := svc.State
	if state == "" {
		state = "unknown"
	}
	running := state == "active" || state == "running"
	source := svc.Source
	if source == "none" || source == "" {
		source = "—"
	}
	return fmt.Sprintf("  %-16s %s %-14s [%s]", label, dot(running, color), title(state), source)
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
