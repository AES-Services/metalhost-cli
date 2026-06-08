package command

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"

	"github.com/AES-Services/metalhost-cli/internal/config"
	"github.com/AES-Services/metalhost-cli/internal/output"
	"github.com/AES-Services/metalhost-cli/internal/version"
	iamv1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/iam/v1"
	"github.com/AES-Services/metalhost-sdk/gen/go/aes/iam/v1/iamv1connect"
	"github.com/AES-Services/metalhost-sdk/metalhost"
)

type rootOptions struct {
	configPath  string
	profile     string
	endpoint    string
	format      string
	quiet       bool
	project     string
	org         string
	region      string
	wait        bool
	waitTimeout time.Duration
	use         string
	short       string
	userAgent   string
}

type commandContext struct {
	root    *rootOptions
	config  *config.File
	profile *config.Profile
}

type RootCommandOptions struct {
	Use       string
	Short     string
	UserAgent string
}

func NewRootCommand() *cobra.Command {
	return NewRootCommandWithOptions(RootCommandOptions{})
}

// NewRootCommandWithOptions builds a fully-loaded customer CLI root in one
// call. Used by the in-package tests and as a back-compat single-shot entry
// point. The two-step path (runtime.NewRootCommand → AttachCustomerCommands)
// is preferred for callers who want to opt out of the customer command tree.
func NewRootCommandWithOptions(commandOpts RootCommandOptions) *cobra.Command {
	opts := &rootOptions{}
	opts.use = defaultString(commandOpts.Use, "metalhost")
	opts.short = defaultString(commandOpts.Short, "Metalhost public CLI")
	opts.userAgent = defaultString(commandOpts.UserAgent, "metalhost-cli")
	cmd := &cobra.Command{
		Use:           opts.use,
		Short:         opts.short,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.PersistentFlags().StringVar(&opts.configPath, "config", "", "config file path")
	cmd.PersistentFlags().StringVar(&opts.profile, "profile", "", "profile name")
	cmd.PersistentFlags().StringVar(&opts.endpoint, "endpoint", "", "Metalhost API endpoint")
	cmd.PersistentFlags().StringVarP(&opts.format, "format", "o", "", "output format: table, json, yaml")
	cmd.PersistentFlags().BoolVarP(&opts.quiet, "quiet", "q", false, "print only resource names (for scripting)")
	addScopeFlags(cmd, opts)
	addWaitFlags(cmd, opts)

	cmd.AddCommand(newVersionCommand(opts))
	cmd.AddCommand(newProfileCommand(opts))
	addCustomerCommands(cmd, opts)
	return cmd
}

// AttachCustomerCommands wires the full customer command tree (auth, iam,
// vm, disk, network, ...) onto a root that's already been built by
// pkg/metalhostcli/runtime.NewRootCommand. Use this when something else owns
// the bare root setup — typically the metalhostcli package wrapping a
// runtime-built root for the `metalhost` binary.
//
// The persistent flags on `cmd` are bound to runtime's rootOptions, not ours.
// We bridge by re-reading flag values into our struct in PersistentPreRunE,
// which cobra invokes on the root before any leaf RunE.
func AttachCustomerCommands(cmd *cobra.Command, commandOpts RootCommandOptions) {
	opts := &rootOptions{
		use:       defaultString(commandOpts.Use, "metalhost"),
		short:     defaultString(commandOpts.Short, "Metalhost public CLI"),
		userAgent: defaultString(commandOpts.UserAgent, "metalhost-cli"),
	}

	prev := cmd.PersistentPreRunE
	cmd.PersistentPreRunE = func(c *cobra.Command, args []string) error {
		opts.configPath, _ = c.Root().PersistentFlags().GetString("config")
		opts.profile, _ = c.Root().PersistentFlags().GetString("profile")
		opts.endpoint, _ = c.Root().PersistentFlags().GetString("endpoint")
		opts.format, _ = c.Root().PersistentFlags().GetString("format")
		opts.quiet, _ = c.Root().PersistentFlags().GetBool("quiet")
		opts.project, _ = c.Root().PersistentFlags().GetString("project")
		opts.org, _ = c.Root().PersistentFlags().GetString("org")
		opts.region, _ = c.Root().PersistentFlags().GetString("region")
		opts.wait, _ = c.Root().PersistentFlags().GetBool("wait")
		opts.waitTimeout, _ = c.Root().PersistentFlags().GetDuration("wait-timeout")
		if prev != nil {
			return prev(c, args)
		}
		return nil
	}

	addScopeFlags(cmd, opts)
	addWaitFlags(cmd, opts)
	addCustomerCommands(cmd, opts)
}

// addScopeFlags registers the persistent --project/--org/--region scope flags.
// They set the active scope for every subcommand; a command-local flag of the
// same name (e.g. `vm list --project`) still takes precedence when supplied.
func addScopeFlags(cmd *cobra.Command, opts *rootOptions) {
	cmd.PersistentFlags().StringVar(&opts.project, "project", "", "project scope (overrides profile default)")
	cmd.PersistentFlags().StringVar(&opts.org, "org", "", "organization scope (overrides profile default)")
	cmd.PersistentFlags().StringVar(&opts.region, "region", "", "region/datacenter scope (overrides profile default)")
}

// addWaitFlags registers the persistent --wait/--wait-timeout flags. When --wait
// is set, any command whose response carries an operation is polled to a
// terminal state and the final operation is rendered in its place.
func addWaitFlags(cmd *cobra.Command, opts *rootOptions) {
	cmd.PersistentFlags().BoolVar(&opts.wait, "wait", false, "wait for the returned operation to finish")
	cmd.PersistentFlags().DurationVar(&opts.waitTimeout, "wait-timeout", 10*time.Minute, "max time to wait with --wait (0 = no limit)")
}

// addCustomerCommands is the single source of truth for which subcommands the
// customer CLI exposes. Both NewRootCommandWithOptions and
// AttachCustomerCommands route through it.
func addCustomerCommands(cmd *cobra.Command, opts *rootOptions) {
	cmd.AddCommand(newInitCommand(opts))
	cmd.AddCommand(newAuthCommand(opts))
	cmd.AddCommand(newGetCommand(opts))
	cmd.AddCommand(newDescribeCommand(opts))
	cmd.AddCommand(newDeleteCommand(opts))
	cmd.AddCommand(newApplyCommand(opts))
	cmd.AddCommand(newDynamicDebugCommand(opts))
	cmd.AddCommand(newIAMCommand(opts))
	cmd.AddCommand(newCatalogCommand(opts))
	cmd.AddCommand(newHealthCommand(opts))
	cmd.AddCommand(newProjectCommand(opts))
	cmd.AddCommand(newOrgCommand(opts))
	cmd.AddCommand(newOpsCommand(opts))
	cmd.AddCommand(newComputeCommand(opts))
	cmd.AddCommand(newStorageCommand(opts))
	cmd.AddCommand(newDiskCommand(opts))
	cmd.AddCommand(newFileShareCommand(opts))
	cmd.AddCommand(newNetworkCommand(opts))
	cmd.AddCommand(newFirewallCommand(opts))
	cmd.AddCommand(newWalletCommand(opts))
	cmd.AddCommand(newQuotaCommand(opts))
	cmd.AddCommand(newAuditCommand(opts))
	cmd.AddCommand(newBareMetalCommand(opts))
	cmd.AddCommand(newWebhooksCommand(opts))
	cmd.AddCommand(newSupportCommand(opts))
	cmd.AddCommand(newCompletionCommand(cmd))
}

func loadCommandContext(opts *rootOptions) (*commandContext, error) {
	cfg, err := config.Load(opts.configPath)
	if err != nil {
		return nil, err
	}
	prof, _, err := cfg.Active(opts.profile)
	if err != nil {
		return nil, err
	}
	if opts.endpoint != "" {
		prof.Endpoint = opts.endpoint
	}
	if opts.format != "" {
		prof.Format = opts.format
	}
	if opts.quiet {
		// --quiet overrides any configured/explicit format: emit just names.
		prof.Format = "name"
	}
	// Persistent --project/--org/--region override the profile defaults for this
	// invocation. Command-local flags of the same name still win because they're
	// read directly at their call sites (requireProject, etc.).
	if opts.project != "" {
		prof.Project = opts.project
	}
	if opts.org != "" {
		prof.Organization = opts.org
	}
	if opts.region != "" {
		prof.Region = opts.region
	}
	return &commandContext{root: opts, config: cfg, profile: prof}, nil
}

func (c *commandContext) sdkConfig() (metalhost.Config, error) {
	if strings.TrimSpace(c.profile.Endpoint) == "" {
		return metalhost.Config{}, errors.New("endpoint is required; set METALHOST_ENDPOINT or run `metalhost profile create NAME --endpoint URL`")
	}
	httpClient := &http.Client{
		Transport: metalhost.Config{
			APIKey:    c.profile.APIKey,
			UserAgent: c.root.userAgentString(),
		}.RoundTripper(http.DefaultTransport),
	}
	return metalhost.Config{
		Endpoint:   c.profile.Endpoint,
		APIKey:     c.profile.APIKey,
		HTTPClient: httpClient,
		UserAgent:  c.root.userAgentString(),
	}, nil
}

func (c *commandContext) iamClient() (iamv1connect.IamServiceClient, error) {
	cfg, err := c.sdkConfig()
	if err != nil {
		return nil, err
	}
	return iamv1connect.NewIamServiceClient(cfg.Client(), cfg.BaseURL()), nil
}

func (c *commandContext) write(value any) error {
	// When --wait is set and the value is an operation-bearing response, block
	// until the operation finishes and render the final operation instead.
	if c.root.wait {
		if msg, ok := value.(proto.Message); ok {
			final, err := c.maybeWait(msg)
			if err != nil {
				return err
			}
			value = final
		}
	}
	return output.Write(os.Stdout, c.profile.Format, value)
}

func (o *rootOptions) userAgentString() string {
	return fmt.Sprintf("%s/%s (%s)", o.userAgent, version.Version, version.Commit)
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func newVersionCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print CLI version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return output.Write(cmd.OutOrStdout(), "table", fmt.Sprintf("%s %s (%s, %s)", opts.use, version.Version, version.Commit, version.Date))
		},
	}
}

func newAuthCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authentication helpers",
		Example: examples(`
  metalhost auth login --email you@example.com
  metalhost auth login --oidc google
  METALHOST_API_KEY=mk_... metalhost auth login --api-key
  metalhost auth whoami`),
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "whoami",
		Short: "Show the authenticated principal",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			if strings.TrimSpace(ctx.profile.APIKey) == "" {
				return errors.New("API key is required; set METALHOST_API_KEY or run `metalhost auth login --api-key`")
			}
			client, err := ctx.iamClient()
			if err != nil {
				return err
			}
			resp, err := client.GetCallerIdentity(cmd.Context(), connect.NewRequest(&iamv1.GetCallerIdentityRequest{}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	})
	var (
		loginAPIKey bool
		loginEmail  string
		loginOIDC   string
	)
	login := &cobra.Command{
		Use:   "login",
		Short: "Sign in and store credentials in the active profile (email/password, OIDC, or --api-key).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			modes := 0
			if loginAPIKey {
				modes++
			}
			if strings.TrimSpace(loginEmail) != "" {
				modes++
			}
			if strings.TrimSpace(loginOIDC) != "" {
				modes++
			}
			if modes == 0 {
				return errors.New("pick a mode: --email <addr>, --oidc google|github, or --api-key")
			}
			if modes > 1 {
				return errors.New("--email, --oidc, and --api-key are mutually exclusive")
			}
			if loginAPIKey {
				return loginWithEnvAPIKey(cmd, opts)
			}
			if strings.TrimSpace(loginEmail) != "" {
				return passwordLogin(cmd, opts, loginEmail)
			}
			return oidcLogin(cmd, opts, loginOIDC)
		},
	}
	login.Flags().BoolVar(&loginAPIKey, "api-key", false, "read API key from METALHOST_API_KEY")
	login.Flags().StringVar(&loginEmail, "email", "", "sign in with email + password (prompts for password)")
	login.Flags().StringVar(&loginOIDC, "oidc", "", "sign in via OIDC provider slug (e.g. google, github) — opens browser")
	cmd.AddCommand(login)
	authFlowsCommands(cmd, opts)
	return cmd
}

// loginWithEnvAPIKey is the legacy --api-key path: read METALHOST_API_KEY, save to active
// profile. Kept for back-compat with existing scripted setups.
func loginWithEnvAPIKey(cmd *cobra.Command, opts *rootOptions) error {
	key := strings.TrimSpace(os.Getenv("METALHOST_API_KEY"))
	if key == "" {
		return errors.New("set METALHOST_API_KEY before running `metalhost auth login --api-key`")
	}
	cfg, err := config.Load(opts.configPath)
	if err != nil {
		return err
	}
	name := opts.profile
	if name == "" {
		name = cfg.CurrentProfile
	}
	if name == "" {
		return errors.New("select a profile first with `metalhost profile use NAME`")
	}
	prof := cfg.Profiles[name]
	if prof == nil {
		return errors.New("profile not found: " + name)
	}
	prof.APIKey = key
	if err := config.Save(opts.configPath, cfg); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "stored API key for profile %q\n", name)
	return nil
}

func runWithBackground(cmd *cobra.Command, fn func(context.Context) error) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	return fn(ctx)
}
