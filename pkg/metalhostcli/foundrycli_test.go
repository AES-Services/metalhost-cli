package metalhostcli

import "testing"

func TestPublicRootDoesNotExposeAdmin(t *testing.T) {
	cmd := NewRootCommand(Options{})
	if _, _, err := cmd.Find([]string{"admin"}); err == nil {
		t.Fatal("public CLI must not expose admin commands")
	}
}
