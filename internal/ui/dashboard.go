package ui

import (
	"fmt"
	"io"
	"strings"

	"github.com/lahiruudayakumara/devnode/internal/status"
)

func Dashboard(w io.Writer, host string, s status.Snapshot, color bool) {
	const width = 48
	line := func(text string) { fmt.Fprintf(w, "│ %-46s │\n", truncate(text, 46)) }
	fmt.Fprintln(w, "╭"+strings.Repeat("─", width)+"╮")
	line("               DEVNODE SERVER")
	fmt.Fprintln(w, "├"+strings.Repeat("─", width)+"┤")
	line("")
	line(fmt.Sprintf("  Connection    %s Online", dot(true, color)))
	line(fmt.Sprintf("  Target        %s", host))
	line(fmt.Sprintf("  Host          %s", s.Host))
	line(fmt.Sprintf("  OS            %s", s.OS))
	line(fmt.Sprintf("  Architecture  %s", s.Arch))
	line(fmt.Sprintf("  Uptime        %s", s.Uptime))
	line("")
	line(metric("CPU", s.CPU))
	line(metric("Memory", s.Memory))
	line(metric("Disk", s.Disk))
	line("")
	fmt.Fprintln(w, "├"+strings.Repeat("─", width)+"┤")
	line("  SERVICES")
	line("")
	for _, svc := range []struct{ label, key string }{{"Docker", "docker"}, {"PostgreSQL", "postgresql"}, {"Redis", "redis"}, {"Nginx", "nginx"}} {
		state := s.Services[svc.key]
		line(fmt.Sprintf("  %-13s %s %s", svc.label, dot(state == "active", color), title(state)))
	}
	line("")
	fmt.Fprintln(w, "╰"+strings.Repeat("─", width)+"╯")
}

func metric(label string, value int) string {
	if value < 0 {
		return fmt.Sprintf("  %-13s %s", label, "unavailable")
	}
	filled := value / 10
	return fmt.Sprintf("  %-13s %s%s  %3d%%", label, strings.Repeat("█", filled), strings.Repeat("░", 10-filled), value)
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

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
