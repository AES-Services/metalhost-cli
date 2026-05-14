package config

import (
	"os"
	"testing"
)

func TestLoadMissingFile(t *testing.T) {
	cfg, err := Load(t.TempDir() + "/missing.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Profiles == nil {
		t.Fatal("profiles map is nil")
	}
}

func TestActiveMergesEnvironment(t *testing.T) {
	t.Setenv("METALHOST_ENDPOINT", "https://env.example.com")
	t.Setenv("METALHOST_API_KEY", "env-key")
	cfg := &File{
		CurrentProfile: "dev",
		Profiles: map[string]*Profile{
			"dev": {Endpoint: "https://profile.example.com"},
		},
	}
	prof, name, err := cfg.Active("")
	if err != nil {
		t.Fatal(err)
	}
	if name != "dev" {
		t.Fatalf("name = %q", name)
	}
	if prof.Endpoint != "https://env.example.com" || prof.APIKey != "env-key" {
		t.Fatalf("profile = %+v", prof)
	}
}

func TestSaveUsesOwnerOnlyPermissions(t *testing.T) {
	path := t.TempDir() + "/config.yaml"
	if err := Save(path, &File{Profiles: map[string]*Profile{"dev": {Endpoint: "x"}}}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("perm = %v", got)
	}
}
