package command

import (
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	iamv1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/iam/v1"
	projectv1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/project/v1"
)

func newProjectCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "project", Aliases: []string{"projects"}, Short: "Manage projects and organizations"}
	cmd.AddCommand(newProjectListCommand(opts))
	cmd.AddCommand(newProjectGetCommand(opts))
	cmd.AddCommand(newProjectCreateCommand(opts))
	cmd.AddCommand(newProjectUpdateCommand(opts))
	cmd.AddCommand(newProjectDeleteCommand(opts))
	cmd.AddCommand(newOrgCommand(opts))
	return cmd
}

func newProjectUpdateCommand(opts *rootOptions) *cobra.Command {
	var displayName string
	cmd := &cobra.Command{
		Use:   "update NAME",
		Short: "Update a project's display name",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.projectClient()
			if err != nil {
				return err
			}
			req := &projectv1.UpdateProjectRequest{Name: args[0]}
			d := displayName
			req.DisplayName = &d
			resp, err := client.UpdateProject(cmd.Context(), connect.NewRequest(req))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}
	cmd.Flags().StringVar(&displayName, "display-name", "", "new display name (required)")
	return cmd
}

func newProjectListCommand(opts *rootOptions) *cobra.Command {
	var pages pageFlags
	var parent string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List projects (defaults to your accessible orgs)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}

			// If the caller didn't pass --org, the server reads an unscoped ListProjects
			// as "list every project on the platform" — admin-only. Customers want their
			// own org's projects. Resolve via GetCallerIdentity and call once per
			// accessible org, merging the results. Same shape the web frontend uses.
			scopes := []string{strings.TrimSpace(parent)}
			if scopes[0] == "" && strings.TrimSpace(ctx.profile.Organization) != "" {
				scopes = []string{ctx.profile.Organization}
			}
			if scopes[0] == "" {
				iam, err := ctx.iamClient()
				if err != nil {
					return err
				}
				idResp, err := iam.GetCallerIdentity(cmd.Context(), connect.NewRequest(&iamv1.GetCallerIdentityRequest{}))
				if err != nil {
					return err
				}
				orgs := idResp.Msg.GetAccessibleOrganizations()
				if len(orgs) == 0 {
					return fmt.Errorf("no accessible organizations on this account; pass --org to scope explicitly")
				}
				scopes = orgs
			}

			client, err := ctx.projectClient()
			if err != nil {
				return err
			}
			merged := &projectv1.ListProjectsResponse{}
			for _, org := range scopes {
				resp, err := client.ListProjects(cmd.Context(), connect.NewRequest(&projectv1.ListProjectsRequest{
					Parent:    org,
					PageSize:  effectivePageSize(pages),
					PageToken: pages.pageToken,
				}))
				if err != nil {
					return fmt.Errorf("list projects in %s: %w", org, err)
				}
				merged.Projects = append(merged.Projects, resp.Msg.GetProjects()...)
				// Next-page tokens are per-org; we expose only the first org's token to
				// keep the response shape stable. Multi-org pagination via the CLI is a
				// future feature.
				if merged.NextPageToken == "" {
					merged.NextPageToken = resp.Msg.GetNextPageToken()
				}
			}
			return ctx.write(merged)
		},
	}
	addPageFlags(cmd, &pages)
	cmd.Flags().StringVar(&parent, "org", "", "parent organization to scope to (defaults to every org you have access to)")
	return cmd
}

func newProjectGetCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "get NAME",
		Short: "Get project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.projectClient()
			if err != nil {
				return err
			}
			resp, err := client.GetProject(cmd.Context(), connect.NewRequest(&projectv1.GetProjectRequest{Name: args[0]}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}
}

func newProjectCreateCommand(opts *rootOptions) *cobra.Command {
	var displayName, parent string
	cmd := &cobra.Command{
		Use:   "create NAME",
		Short: "Create project (NAME e.g. projects/my-proj)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.projectClient()
			if err != nil {
				return err
			}
			resp, err := client.CreateProject(cmd.Context(), connect.NewRequest(&projectv1.CreateProjectRequest{Name: args[0], DisplayName: displayName, Parent: parent}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}
	cmd.Flags().StringVar(&displayName, "display-name", "", "display name")
	cmd.Flags().StringVar(&parent, "org", "", "parent organization, e.g. organizations/acme (required)")
	return cmd
}

func newProjectDeleteCommand(opts *rootOptions) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete NAME",
		Short: "Delete project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			if err := confirmDestructive(cmd, yes, "Delete project (and all resources within)", args[0]); err != nil {
				return err
			}
			client, err := ctx.projectClient()
			if err != nil {
				return err
			}
			resp, err := client.DeleteProject(cmd.Context(), connect.NewRequest(&projectv1.DeleteProjectRequest{Name: args[0]}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the interactive confirmation prompt")
	return cmd
}

func newOrgCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "org", Aliases: []string{"organization", "organizations", "orgs"}, Short: "Manage organizations"}
	cmd.AddCommand(&cobra.Command{
		Use:   "get NAME",
		Short: "Get organization",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.projectClient()
			if err != nil {
				return err
			}
			resp, err := client.GetOrganization(cmd.Context(), connect.NewRequest(&projectv1.GetOrganizationRequest{Name: args[0]}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	})

	var createDisplay string
	create := &cobra.Command{
		Use:   "create NAME",
		Short: "Create organization (NAME e.g. organizations/acme)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.projectClient()
			if err != nil {
				return err
			}
			resp, err := client.CreateOrganization(cmd.Context(), connect.NewRequest(&projectv1.CreateOrganizationRequest{Name: args[0], DisplayName: createDisplay}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}
	create.Flags().StringVar(&createDisplay, "display-name", "", "display name")
	cmd.AddCommand(create)

	var updateOrgDisplay string
	var updateOrgRequireMFA bool
	updateOrg := &cobra.Command{
		Use:   "update NAME",
		Short: "Update an organization (display name and/or MFA enforcement)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.projectClient()
			if err != nil {
				return err
			}
			req := &projectv1.UpdateOrganizationRequest{Name: args[0]}
			// Apply each optional field only when the operator explicitly passed the flag;
			// cobra's Changed() lets us distinguish "set to empty/false" from "unset". This
			// matters for require_mfa where false is a meaningful operator choice.
			if cmd.Flags().Changed("display-name") {
				d := updateOrgDisplay
				req.DisplayName = &d
			}
			if cmd.Flags().Changed("require-mfa") {
				r := updateOrgRequireMFA
				req.RequireMfa = &r
			}
			if req.DisplayName == nil && req.RequireMfa == nil {
				return errors.New("at least one of --display-name or --require-mfa must be set")
			}
			resp, err := client.UpdateOrganization(cmd.Context(), connect.NewRequest(req))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}
	updateOrg.Flags().StringVar(&updateOrgDisplay, "display-name", "", "new display name")
	updateOrg.Flags().BoolVar(&updateOrgRequireMFA, "require-mfa", false, "require MFA at login for all members with an enrolled TOTP device (P0-4)")
	cmd.AddCommand(updateOrg)

	var orgDelYes bool
	orgDelete := &cobra.Command{
		Use:   "delete NAME",
		Short: "Delete organization",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			if err := confirmDestructive(cmd, orgDelYes, "Delete organization (and every project within)", args[0]); err != nil {
				return err
			}
			client, err := ctx.projectClient()
			if err != nil {
				return err
			}
			resp, err := client.DeleteOrganization(cmd.Context(), connect.NewRequest(&projectv1.DeleteOrganizationRequest{Name: args[0]}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}
	orgDelete.Flags().BoolVar(&orgDelYes, "yes", false, "skip the interactive confirmation prompt")
	cmd.AddCommand(orgDelete)

	var activityPages pageFlags
	activity := &cobra.Command{
		Use:   "activity NAME",
		Short: "List recent activity for an organization",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.projectClient()
			if err != nil {
				return err
			}
			resp, err := client.ListOrgActivity(cmd.Context(), connect.NewRequest(&projectv1.ListOrgActivityRequest{OrganizationName: args[0], PageSize: effectivePageSize(activityPages), PageToken: activityPages.pageToken}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}
	addPageFlags(activity, &activityPages)
	cmd.AddCommand(activity)

	return cmd
}
