package ssh

import "testing"

func TestJoinCommand(t *testing.T) {
	got := JoinCommand([]string{"printf", "%s", "hello world", "it's safe"})
	want := `'printf' '%s' 'hello world' 'it'"'"'s safe'`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
