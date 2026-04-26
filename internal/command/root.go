package command

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	"github.com/AES-Services/foundry-cli/internal/config"
	"github.com/AES-Services/foundry-cli/internal/output"
	"github.com/AES-Services/foundry-cli/internal/version"
	"github.com/AES-Services/foundry-sdk/foundry"
	iamv1 "github.com/AES-Services/foundry-sdk/gen/go/aes/iam/v1"
	"github.com/AES-Services/foundry-sdk/gen/go/aes/iam/v1/iamv1connect"
)

type rootOptions struct {
	configPath string
	profile    string
	endpoint   string
	format     string
	use        string
	short      string
	userAgent  string
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

func NewRootCommandWithOptions(commandOpts RootCommandOptions) *cobra.Command {
	opts := &rootOptions{}
	opts.use = defaultString(commandOpts.Use, "foundry")
	opts.short = defaultString(commandOpts.Short, "Foundry public CLI")
	opts.userAgent = defaultString(commandOpts.UserAgent, "foundry-cli")
	cmd := &cobra.Command{
		Use:          opts.use,
		Short:        opts.short,
		SilenceUsage: true,
	}
	cmd.PersistentFlags().StringVar(&opts.configPath, "config", "", "config file path")
	cmd.PersistentFlags().StringVar(&opts.profile, "profile", "", "profile name")
	cmd.PersistentFlags().StringVar(&opts.endpoint, "endpoint", "", "Foundry API endpoint")
	cmd.PersistentFlags().StringVarP(&opts.format, "format", "o", "", "output format: table, json, yaml")

	cmd.AddCommand(newVersionCommand(opts))
	cmd.AddCommand(newProfileCommand(opts))
	cmd.AddCommand(newAuthCommand(opts))
	cmd.AddCommand(newIAMCommand(opts))
	cmd.AddCommand(newCatalogCommand(opts))
	cmd.AddCommand(newHealthCommand(opts))
	cmd.AddCommand(newProjectCommand(opts))
	cmd.AddCommand(newOpsCommand(opts))
	cmd.AddCommand(newComputeCommand(opts))
	cmd.AddCommand(newImageCommand(opts))
	cmd.AddCommand(newStorageCommand(opts))
	cmd.AddCommand(newDiskCommand(opts))
	cmd.AddCommand(newSnapshotCommand(opts))
	cmd.AddCommand(newFileShareCommand(opts))
	cmd.AddCommand(newNetworkCommand(opts))
	cmd.AddCommand(newPublicIPCommand(opts))
	cmd.AddCommand(newFirewallCommand(opts))
	cmd.AddCommand(newLoadBalancerCommand(opts))
	cmd.AddCommand(newDNSCommand(opts))
	cmd.AddCommand(newObjectStoreCommand(opts))
	cmd.AddCommand(newWalletCommand(opts))
	cmd.AddCommand(newQuotaCommand(opts))
	cmd.AddCommand(newAuditCommand(opts))
	cmd.AddCommand(newBareMetalCommand(opts))
	cmd.AddCommand(newWebhooksCommand(opts))
	cmd.AddCommand(newCompletionCommand(cmd))
	return cmd
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
	return &commandContext{root: opts, config: cfg, profile: prof}, nil
}

func (c *commandContext) sdkConfig() (foundry.Config, error) {
	if strings.TrimSpace(c.profile.Endpoint) == "" {
		return foundry.Config{}, errors.New("endpoint is required; set FOUNDRY_ENDPOINT or run `foundry profile create NAME --endpoint URL`")
	}
	httpClient := &http.Client{
		Transport: foundry.Config{
			APIKey:    c.profile.APIKey,
			UserAgent: c.root.userAgentString(),
		}.RoundTripper(http.DefaultTransport),
	}
	return foundry.Config{
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
	cmd := &cobra.Command{Use: "auth", Short: "Authentication helpers"}
	cmd.AddCommand(&cobra.Command{
		Use:   "metadata",
		Short: "Show server auth metadata",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.iamClient()
			if err != nil {
				return err
			}
			resp, err := client.GetAuthMetadata(cmd.Context(), connect.NewRequest(&iamv1.GetAuthMetadataRequest{}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "whoami",
		Short: "Show the authenticated principal",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			if strings.TrimSpace(ctx.profile.APIKey) == "" {
				return errors.New("API key is required; set FOUNDRY_API_KEY or run `foundry auth login --api-key`")
			}
			client, err := ctx.iamClient()
			if err != nil {
				return err
			}
			resp, err := client.ValidateApiKey(cmd.Context(), connect.NewRequest(&iamv1.ValidateApiKeyRequest{RawKey: ctx.profile.APIKey}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	})
	var loginAPIKey bool
	login := &cobra.Command{
		Use:   "login",
		Short: "Store credentials in the active profile",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !loginAPIKey {
				return errors.New("only --api-key login is implemented in this scaffold")
			}
			key := strings.TrimSpace(os.Getenv("FOUNDRY_API_KEY"))
			if key == "" {
				return errors.New("set FOUNDRY_API_KEY before running `foundry auth login --api-key`")
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
				return errors.New("select a profile first with `foundry profile use NAME`")
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
		},
	}
	login.Flags().BoolVar(&loginAPIKey, "api-key", false, "read API key from FOUNDRY_API_KEY")
	cmd.AddCommand(login)
	return cmd
}

func runWithBackground(cmd *cobra.Command, fn func(context.Context) error) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	return fn(ctx)
}
