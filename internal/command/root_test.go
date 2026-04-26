package command

import (
	"bytes"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"version"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if out.String() == "" {
		t.Fatal("empty version output")
	}
}

func TestProfileCreateUseList(t *testing.T) {
	configPath := t.TempDir() + "/config.yaml"
	cmd := NewRootCommand()
	cmd.SetArgs([]string{"--config", configPath, "profile", "create", "dev", "--endpoint", "http://127.0.0.1:8080"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	cmd = NewRootCommand()
	cmd.SetArgs([]string{"--config", configPath, "profile", "use", "dev"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	cmd = NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--config", configPath, "profile", "list"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if out.String() != "* dev\n" {
		t.Fatalf("profile list = %q", out.String())
	}
}

func TestPublicCommandSurface(t *testing.T) {
	cmd := NewRootCommand()
	want := []string{
		"audit",
		"auth",
		"baremetal",
		"bucket",
		"catalog",
		"disk",
		"health",
		"iam",
		"image",
		"network",
		"ops",
		"profile",
		"project",
		"quota",
		"storage",
		"vm",
		"wallet",
		"webhook",
	}
	for _, name := range want {
		if _, _, err := cmd.Find([]string{name}); err != nil {
			t.Fatalf("missing command %q: %v", name, err)
		}
	}
	if _, _, err := cmd.Find([]string{"admin"}); err == nil {
		t.Fatal("public CLI must not expose admin commands")
	}
}
