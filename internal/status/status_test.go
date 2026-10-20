package status

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	s, err := Parse("HOST=devbox\nOS=Ubuntu 26.04 LTS\nARCH=x86_64\nUPTIME=up 4 hours\nCPU=71\nMEMORY=50\nDISK=30\nSERVICE_docker=active\n")
	if err != nil {
		t.Fatal(err)
	}
	if s.Host != "devbox" || s.Uptime != "4 hours" || s.CPU != 71 || s.Services["docker"] != "active" {
		t.Fatalf("unexpected snapshot: %+v", s)
	}
}

func TestParseBadPercent(t *testing.T) {
	s, err := Parse("HOST=devbox\nCPU=?\nMEMORY=101\n")
	if err != nil {
		t.Fatal(err)
	}
	if s.CPU != -1 || s.Memory != -1 {
		t.Fatalf("invalid percentages accepted: %+v", s)
	}
}

func TestScriptUsesConfiguredPostgresUnit(t *testing.T) {
	got := Script("postgresql@16-main")
	if !strings.Contains(got, "postgresql:postgresql@16-main") {
		t.Fatalf("configured unit missing from script")
	}
}
