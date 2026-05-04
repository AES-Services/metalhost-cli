package command

import (
	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	iamv1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/iam/v1"
)

func newIAMCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "iam", Short: "Manage identity resources"}
	cmd.AddCommand(newAPIKeysCommand(opts), newMembersCommand(opts), newSessionsCommand(opts))
	return cmd
}

func newAPIKeysCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "keys", Aliases: []string{"api-keys"}, Short: "Manage API keys"}
	var pages pageFlags
	list := &cobra.Command{Use: "list", Short: "List API keys", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.iamClient()
		if err != nil {
			return err
		}
		resp, err := client.ListApiKeys(cmd.Context(), connect.NewRequest(&iamv1.ListApiKeysRequest{PageSize: effectivePageSize(pages), PageToken: pages.pageToken}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	addPageFlags(list, &pages)
	cmd.AddCommand(list)
	var displayName, project string
	create := &cobra.Command{Use: "create", Short: "Create API key", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		if project == "" {
			project = ctx.profile.Project
		}
		client, err := ctx.iamClient()
		if err != nil {
			return err
		}
		resp, err := client.CreateApiKey(cmd.Context(), connect.NewRequest(&iamv1.CreateApiKeyRequest{DisplayName: displayName, DefaultProject: project}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	create.Flags().StringVar(&displayName, "display-name", "", "display name")
	create.Flags().StringVar(&project, "project", "", "default project")
	cmd.AddCommand(create)
	cmd.AddCommand(&cobra.Command{Use: "revoke ID", Short: "Revoke API key", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.iamClient()
		if err != nil {
			return err
		}
		resp, err := client.RevokeApiKey(cmd.Context(), connect.NewRequest(&iamv1.RevokeApiKeyRequest{Name: args[0]}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}})
	return cmd
}

func newMembersCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "members", Aliases: []string{"member"}, Short: "Manage organization members"}
	var pages pageFlags
	var org string
	list := &cobra.Command{Use: "list", Short: "List org members", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.iamClient()
		if err != nil {
			return err
		}
		resp, err := client.ListOrgMembers(cmd.Context(), connect.NewRequest(&iamv1.ListOrgMembersRequest{OrganizationName: org, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	addPageFlags(list, &pages)
	list.Flags().StringVar(&org, "org", "", "organization")
	cmd.AddCommand(list)
	var inviteOrg, email, role, displayName string
	invite := &cobra.Command{Use: "invite", Short: "Invite org member", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.iamClient()
		if err != nil {
			return err
		}
		resp, err := client.InviteOrgMember(cmd.Context(), connect.NewRequest(&iamv1.InviteOrgMemberRequest{OrganizationName: inviteOrg, Email: email, Role: role, DisplayName: displayName}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	invite.Flags().StringVar(&inviteOrg, "org", "", "organization")
	invite.Flags().StringVar(&email, "email", "", "invitee email")
	invite.Flags().StringVar(&role, "role", "viewer", "member role")
	invite.Flags().StringVar(&displayName, "display-name", "", "display name")
	cmd.AddCommand(invite)
	return cmd
}

func newSessionsCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "sessions", Short: "Manage auth sessions"}
	cmd.AddCommand(&cobra.Command{Use: "list", Short: "List sessions", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.iamClient()
		if err != nil {
			return err
		}
		resp, err := client.ListSessions(cmd.Context(), connect.NewRequest(&iamv1.ListSessionsRequest{}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}})
	cmd.AddCommand(&cobra.Command{Use: "revoke SESSION_ID", Short: "Revoke session", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.iamClient()
		if err != nil {
			return err
		}
		resp, err := client.RevokeSession(cmd.Context(), connect.NewRequest(&iamv1.RevokeSessionRequest{Name: args[0]}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}})
	return cmd
}
