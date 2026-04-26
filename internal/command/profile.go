package command

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/AES-Services/foundry-cli/internal/config"
)

func newProfileCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "profile", Short: "Manage CLI profiles"}
	cmd.AddCommand(newProfileListCommand(opts))
	cmd.AddCommand(newProfileCreateCommand(opts))
	cmd.AddCommand(newProfileUseCommand(opts))
	cmd.AddCommand(newProfileSetCommand(opts))
	cmd.AddCommand(newProfileDeleteCommand(opts))
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
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "Foundry API endpoint")
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
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "Foundry API endpoint")
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
