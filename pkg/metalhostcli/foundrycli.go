package metalhostcli

import (
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/AES-Services/metalhost-cli/internal/command"
	"github.com/AES-Services/metalhost-cli/internal/config"
	"github.com/AES-Services/metalhost-cli/internal/output"
	"github.com/AES-Services/metalhost-cli/internal/version"
	"github.com/AES-Services/metalhost-sdk/metalhost"
)

type Options = command.RootCommandOptions
type Profile = config.Profile

type Runtime struct {
	Profile   *Profile
	UserAgent string
}

func NewRootCommand(opts Options) *cobra.Command {
	return command.NewRootCommandWithOptions(opts)
}

func RuntimeFromCommand(cmd *cobra.Command, userAgent string) (*Runtime, error) {
	configPath, _ := cmd.Root().PersistentFlags().GetString("config")
	profileName, _ := cmd.Root().PersistentFlags().GetString("profile")
	endpoint, _ := cmd.Root().PersistentFlags().GetString("endpoint")
	format, _ := cmd.Root().PersistentFlags().GetString("format")

	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}
	prof, _, err := cfg.Active(profileName)
	if err != nil {
		return nil, err
	}
	if endpoint != "" {
		prof.Endpoint = endpoint
	}
	if format != "" {
		prof.Format = format
	}
	if strings.TrimSpace(userAgent) == "" {
		userAgent = "metalhost-cli/" + version.Version + " (" + version.Commit + ")"
	}
	return &Runtime{Profile: prof, UserAgent: userAgent}, nil
}

func (r *Runtime) SDKConfig() (metalhost.Config, error) {
	if r == nil || r.Profile == nil || strings.TrimSpace(r.Profile.Endpoint) == "" {
		return metalhost.Config{}, errors.New("endpoint is required; set FOUNDRY_ENDPOINT or run `metalhost profile create NAME --endpoint URL`")
	}
	httpClient := &http.Client{
		Transport: metalhost.Config{
			APIKey:    r.Profile.APIKey,
			UserAgent: r.UserAgent,
		}.RoundTripper(http.DefaultTransport),
	}
	return metalhost.Config{
		Endpoint:   r.Profile.Endpoint,
		APIKey:     r.Profile.APIKey,
		HTTPClient: httpClient,
		UserAgent:  r.UserAgent,
	}, nil
}

func (r *Runtime) Write(value any) error {
	return output.Write(os.Stdout, r.Profile.Format, value)
}
