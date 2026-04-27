package command

import (
	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	projectv1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/project/v1"
)

func newProjectCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "project", Aliases: []string{"projects"}, Short: "Manage projects, organizations, and folders"}
	cmd.AddCommand(newProjectListCommand(opts))
	cmd.AddCommand(newProjectGetCommand(opts))
	cmd.AddCommand(newProjectCreateCommand(opts))
	cmd.AddCommand(newOrgCommand(opts))
	cmd.AddCommand(newFolderCommand(opts))
	cmd.AddCommand(newProjectResourceCommand(opts))
	return cmd
}

func newProjectListCommand(opts *rootOptions) *cobra.Command {
	var pages pageFlags
	var parent, folder string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List projects",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.projectClient()
			if err != nil {
				return err
			}
			resp, err := client.ListProjects(cmd.Context(), connect.NewRequest(&projectv1.ListProjectsRequest{Parent: parent, Folder: folder, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}
	addPageFlags(cmd, &pages)
	cmd.Flags().StringVar(&parent, "org", "", "parent organization")
	cmd.Flags().StringVar(&folder, "folder", "", "folder")
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
	var displayName, parent, folder string
	cmd := &cobra.Command{
		Use:   "create NAME",
		Short: "Create project",
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
			resp, err := client.CreateProject(cmd.Context(), connect.NewRequest(&projectv1.CreateProjectRequest{Name: args[0], DisplayName: displayName, Parent: parent, Folder: folder}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}
	cmd.Flags().StringVar(&displayName, "display-name", "", "display name")
	cmd.Flags().StringVar(&parent, "org", "", "parent organization")
	cmd.Flags().StringVar(&folder, "folder", "", "folder")
	return cmd
}

func newOrgCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "org", Aliases: []string{"organization", "organizations", "orgs"}, Short: "Manage organizations"}
	var pages pageFlags
	list := &cobra.Command{
		Use:   "list",
		Short: "List organizations",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.projectClient()
			if err != nil {
				return err
			}
			resp, err := client.ListOrganizations(cmd.Context(), connect.NewRequest(&projectv1.ListOrganizationsRequest{PageSize: effectivePageSize(pages), PageToken: pages.pageToken}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}
	addPageFlags(list, &pages)
	cmd.AddCommand(list)
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
	return cmd
}

func newFolderCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "folder", Aliases: []string{"folders"}, Short: "Manage folders"}
	var pages pageFlags
	var parent string
	list := &cobra.Command{
		Use:   "list",
		Short: "List folders",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.projectClient()
			if err != nil {
				return err
			}
			resp, err := client.ListFolders(cmd.Context(), connect.NewRequest(&projectv1.ListFoldersRequest{Parent: parent, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}
	addPageFlags(list, &pages)
	list.Flags().StringVar(&parent, "org", "", "parent organization")
	cmd.AddCommand(list)
	cmd.AddCommand(&cobra.Command{
		Use:   "get NAME",
		Short: "Get folder",
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
			resp, err := client.GetFolder(cmd.Context(), connect.NewRequest(&projectv1.GetFolderRequest{Name: args[0]}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	})
	return cmd
}

func newProjectResourceCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "resource", Aliases: []string{"res"}, Short: "Project resources"}
	var pages pageFlags
	var parent, folder string
	list := &cobra.Command{
		Use:   "list",
		Short: "List projects",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.projectClient()
			if err != nil {
				return err
			}
			resp, err := client.ListProjects(cmd.Context(), connect.NewRequest(&projectv1.ListProjectsRequest{Parent: parent, Folder: folder, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}
	addPageFlags(list, &pages)
	list.Flags().StringVar(&parent, "org", "", "parent organization")
	list.Flags().StringVar(&folder, "folder", "", "folder")
	cmd.AddCommand(list)
	cmd.AddCommand(&cobra.Command{
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
	})
	return cmd
}
