package catalog

import (
	"strings"
	"testing"
)

func TestCatalogHasNoBulkInstaller(t *testing.T) {
	if _, ok := Get("all"); ok {
		t.Fatal("bulk installer must not exist")
	}
	for _, name := range Names() {
		s, ok := Get(name)
		if !ok || s.Name != name || s.Label == "" || s.Description == "" {
			t.Fatalf("invalid catalog entry %q: %+v", name, s)
		}
		if (s.Script == "") == (s.Manual == "") {
			t.Fatalf("%s must have exactly one installation path (Script xor Manual)", name)
		}
		if s.Script == "" && len(s.Steps) == 0 {
			t.Fatalf("%s is guided but has no Steps defined", name)
		}
		if strings.Contains(s.Script, "; run ") {
			t.Fatalf("%s has an unwrapped docker command", name)
		}
	}
}
