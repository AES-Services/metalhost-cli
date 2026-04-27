package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

type File struct {
	CurrentProfile string              `json:"current_profile" yaml:"current_profile"`
	Profiles       map[string]*Profile `json:"profiles" yaml:"profiles"`
}

type Profile struct {
	Endpoint     string `json:"endpoint" yaml:"endpoint"`
	APIKey       string `json:"api_key,omitempty" yaml:"api_key,omitempty"`
	Organization string `json:"organization,omitempty" yaml:"organization,omitempty"`
	Project      string `json:"project,omitempty" yaml:"project,omitempty"`
	Region       string `json:"region,omitempty" yaml:"region,omitempty"`
	Format       string `json:"format,omitempty" yaml:"format,omitempty"`
}

func DefaultPath() (string, error) {
	if runtime.GOOS == "windows" {
		base := os.Getenv("AppData")
		if base == "" {
			return "", errors.New("AppData is not set")
		}
		return filepath.Join(base, "Metalhost", "config.yaml"), nil
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "metalhost", "config.yaml"), nil
}

func Load(path string) (*File, error) {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return nil, err
		}
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &File{Profiles: map[string]*Profile{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var out File
	if err := yaml.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	if out.Profiles == nil {
		out.Profiles = map[string]*Profile{}
	}
	return &out, nil
}

func Save(path string, f *File) error {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return err
		}
	}
	if f.Profiles == nil {
		f.Profiles = map[string]*Profile{}
	}
	b, err := yaml.Marshal(f)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

func (f *File) Active(profileName string) (*Profile, string, error) {
	if f.Profiles == nil {
		f.Profiles = map[string]*Profile{}
	}
	if profileName == "" {
		profileName = os.Getenv("FOUNDRY_PROFILE")
	}
	if profileName == "" {
		profileName = f.CurrentProfile
	}
	if profileName == "" {
		return mergeEnv(&Profile{}), "", nil
	}
	p, ok := f.Profiles[profileName]
	if !ok {
		return nil, profileName, errors.New("profile not found: " + profileName)
	}
	return mergeEnv(p), profileName, nil
}

func mergeEnv(p *Profile) *Profile {
	out := *p
	if v := strings.TrimSpace(os.Getenv("FOUNDRY_ENDPOINT")); v != "" {
		out.Endpoint = v
	}
	if v := strings.TrimSpace(os.Getenv("FOUNDRY_API_KEY")); v != "" {
		out.APIKey = v
	}
	if v := strings.TrimSpace(os.Getenv("FOUNDRY_ORGANIZATION")); v != "" {
		out.Organization = v
	}
	if v := strings.TrimSpace(os.Getenv("FOUNDRY_PROJECT")); v != "" {
		out.Project = v
	}
	if v := strings.TrimSpace(os.Getenv("FOUNDRY_REGION")); v != "" {
		out.Region = v
	}
	if v := strings.TrimSpace(os.Getenv("FOUNDRY_FORMAT")); v != "" {
		out.Format = v
	}
	return &out
}
