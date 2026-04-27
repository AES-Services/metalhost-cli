// Package runtime is the proto-free half of the Metalhost CLI library.
//
// It exposes:
//   - Options / Profile aliases consumers need
//   - NewRootCommand — builds a BARE cobra root (persistent flags + version +
//     profile management). No customer commands are attached here, and no
//     SDK gen/aes/* packages are imported transitively.
//   - Runtime helpers (RuntimeFromCommand, SDKConfig, Write) for callers that
//     want to make API calls themselves once authenticated.
//
// The customer-facing CLI (`metalhost` binary) wraps this package in
// pkg/metalhostcli, which adds the full vm/disk/network/... command tree.
//
// Internal admin tools (the `mh` binary in the Metalhost backend repo) import
// this package directly to avoid pulling in customer proto descriptors and
// triggering a duplicate-registration panic — the backend already has its own
// generated copy of those proto packages.
package runtime

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/AES-Services/metalhost-cli/internal/config"
	"github.com/AES-Services/metalhost-cli/internal/output"
	"github.com/AES-Services/metalhost-cli/internal/version"
	"github.com/AES-Services/metalhost-sdk/metalhost"
)

// Options configures the bare cobra root produced by NewRootCommand.
type Options struct {
	// Use is the cobra Use string (binary name).
	Use string
	// Short is the one-line description shown in --help.
	Short string
	// UserAgent is sent on outgoing API calls. Falls back to a derived value.
	UserAgent string
}

// Profile is re-exported for callers that want to type Profile pointers
// without depending on internal/config directly.
type Profile = config.Profile

// Runtime carries the active profile + the user-agent string for outgoing
// requests. Build one with RuntimeFromCommand at the start of a RunE handler.
type Runtime struct {
	Profile   *Profile
	UserAgent string
}

type rootOptions struct {
	configPath string
	profile    string
	endpoint   string
	format     string
	use        string
	userAgent  string
}

// NewRootCommand returns a bare cobra root command — persistent flags, plus
// the truly common subcommands (`version`, `profile`). Callers attach their
// own command tree on top.
func NewRootCommand(opts Options) *cobra.Command {
	ro := &rootOptions{
		use:       firstNonEmpty(opts.Use, "metalhost"),
		userAgent: firstNonEmpty(opts.UserAgent, "metalhost-cli"),
	}
	short := firstNonEmpty(opts.Short, "Metalhost CLI")
	cmd := &cobra.Command{
		Use:          ro.use,
		Short:        short,
		SilenceUsage: true,
	}
	cmd.PersistentFlags().StringVar(&ro.configPath, "config", "", "config file path")
	cmd.PersistentFlags().StringVar(&ro.profile, "profile", "", "profile name")
	cmd.PersistentFlags().StringVar(&ro.endpoint, "endpoint", "", "Metalhost API endpoint")
	cmd.PersistentFlags().StringVarP(&ro.format, "format", "o", "", "output format: table, json, yaml")

	cmd.AddCommand(newVersionCommand(ro))
	cmd.AddCommand(newProfileCommand(ro))
	return cmd
}

// RuntimeFromCommand reads the persistent flags off the root command and
// builds a Runtime tied to the active profile. RunE handlers should call this
// first; surface its error to the user.
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

// SDKConfig converts the active Runtime into a metalhost SDK Config. Returns
// a clear error when no endpoint is set (the most common misconfiguration).
func (r *Runtime) SDKConfig() (metalhost.Config, error) {
	if r == nil || r.Profile == nil || strings.TrimSpace(r.Profile.Endpoint) == "" {
		return metalhost.Config{}, errors.New("endpoint is required; set METALHOST_ENDPOINT or run `metalhost profile create NAME --endpoint URL`")
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

// Write renders a value to stdout in the active profile's format.
func (r *Runtime) Write(value any) error {
	return output.Write(os.Stdout, r.Profile.Format, value)
}

// ── helpers ───────────────────────────────────────────────────────────────

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func newVersionCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print CLI version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return output.Write(cmd.OutOrStdout(), "table",
				fmt.Sprintf("%s %s (%s, %s)", opts.use, version.Version, version.Commit, version.Date))
		},
	}
}

// ── profile subcommands ───────────────────────────────────────────────────
//
// Inlined here (rather than imported from internal/command) so this package
// stays free of SDK proto descriptors. The logic is the same as the customer
// CLI's profile commands; if behavior diverges, keep both in sync.

func newProfileCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "profile", Short: "Manage CLI profiles"}
	cmd.AddCommand(
		newProfileListCommand(opts),
		newProfileCreateCommand(opts),
		newProfileUseCommand(opts),
		newProfileSetCommand(opts),
		newProfileDeleteCommand(opts),
	)
	return cmd
}

func newProfileListCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List profiles",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(opts.configPath)
			if err != nil {
				return err
			}
			for name := range cfg.Profiles {
				marker := " "
				if name == cfg.CurrentProfile {
					marker = "*"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", marker, name)
			}
			return nil
		},
	}
}

func newProfileCreateCommand(opts *rootOptions) *cobra.Command {
	var endpoint, project, org, region string
	cmd := &cobra.Command{
		Use:   "create NAME",
		Short: "Create a profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if endpoint == "" {
				return errors.New("--endpoint is required")
			}
			cfg, err := config.Load(opts.configPath)
			if err != nil {
				return err
			}
			if cfg.Profiles == nil {
				cfg.Profiles = map[string]*config.Profile{}
			}
			name := args[0]
			cfg.Profiles[name] = &config.Profile{
				Endpoint:     endpoint,
				Organization: org,
				Project:      project,
				Region:       region,
			}
			if cfg.CurrentProfile == "" {
				cfg.CurrentProfile = name
			}
			if err := config.Save(opts.configPath, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "created profile %q\n", name)
			return nil
		},
	}
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "Metalhost API endpoint")
	cmd.Flags().StringVar(&project, "project", "", "default project")
	cmd.Flags().StringVar(&org, "organization", "", "default organization")
	cmd.Flags().StringVar(&region, "region", "", "default region/datacenter")
	return cmd
}

func newProfileUseCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "use NAME",
		Short: "Select active profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(opts.configPath)
			if err != nil {
				return err
			}
			if cfg.Profiles[args[0]] == nil {
				return errors.New("profile not found: " + args[0])
			}
			cfg.CurrentProfile = args[0]
			if err := config.Save(opts.configPath, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "using profile %q\n", args[0])
			return nil
		},
	}
}

func newProfileSetCommand(opts *rootOptions) *cobra.Command {
	var endpoint, project, org, region, format string
	cmd := &cobra.Command{
		Use:   "set NAME",
		Short: "Update profile defaults",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(opts.configPath)
			if err != nil {
				return err
			}
			prof := cfg.Profiles[args[0]]
			if prof == nil {
				return errors.New("profile not found: " + args[0])
			}
			if endpoint != "" {
				prof.Endpoint = endpoint
			}
			if project != "" {
				prof.Project = project
			}
			if org != "" {
				prof.Organization = org
			}
			if region != "" {
				prof.Region = region
			}
			if format != "" {
				prof.Format = format
			}
			if err := config.Save(opts.configPath, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "updated profile %q\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "Metalhost API endpoint")
	cmd.Flags().StringVar(&project, "project", "", "default project")
	cmd.Flags().StringVar(&org, "organization", "", "default organization")
	cmd.Flags().StringVar(&region, "region", "", "default region/datacenter")
	cmd.Flags().StringVar(&format, "format", "", "default output format")
	return cmd
}

func newProfileDeleteCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "delete NAME",
		Short: "Delete a profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(opts.configPath)
			if err != nil {
				return err
			}
			delete(cfg.Profiles, args[0])
			if cfg.CurrentProfile == args[0] {
				cfg.CurrentProfile = ""
			}
			if err := config.Save(opts.configPath, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted profile %q\n", args[0])
			return nil
		},
	}
}
