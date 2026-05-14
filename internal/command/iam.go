package command

import (
	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	iamv1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/iam/v1"
)

func newIAMCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "iam", Short: "Identity, API keys, members, sessions, MFA"}
	cmd.AddCommand(
		newAPIKeysCommand(opts),
		newMembersCommand(opts),
		newSessionsCommand(opts),
		newMFACommand(opts),
		newInvitesCommand(opts),
		newUserCommand(opts),
		newPasswordCommand(opts),
		newNotificationsCommand(opts),
		newSshKeyImportCommand(opts),
	)
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
	var projectScoped bool
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
		resp, err := client.CreateApiKey(cmd.Context(), connect.NewRequest(&iamv1.CreateApiKeyRequest{DisplayName: displayName, DefaultProject: project, ProjectScoped: projectScoped}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	create.Flags().StringVar(&displayName, "display-name", "", "display name")
	create.Flags().StringVar(&project, "project", "", "default project")
	create.Flags().BoolVar(&projectScoped, "project-scoped", false, "lock this key to the default project only")
	cmd.AddCommand(create)

	cmd.AddCommand(&cobra.Command{Use: "revoke ID", Short: "Revoke an API key", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
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

	var rotateDisplayName string
	rotate := &cobra.Command{Use: "rotate KEY_PREFIX", Short: "Rotate an API key (returns the new secret)", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.iamClient()
		if err != nil {
			return err
		}
		resp, err := client.RotateApiKey(cmd.Context(), connect.NewRequest(&iamv1.RotateApiKeyRequest{KeyPrefix: args[0], DisplayName: rotateDisplayName}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	rotate.Flags().StringVar(&rotateDisplayName, "display-name", "", "display name override for the rotated key")
	cmd.AddCommand(rotate)

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
	list.Flags().StringVar(&org, "org", "", "organization (required)")
	cmd.AddCommand(list)

	var inviteOrg, email, role, displayName string
	invite := &cobra.Command{Use: "invite", Short: "Invite a member by email", RunE: func(cmd *cobra.Command, _ []string) error {
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
	invite.Flags().StringVar(&inviteOrg, "org", "", "organization (required)")
	invite.Flags().StringVar(&email, "email", "", "invitee email")
	invite.Flags().StringVar(&role, "role", "viewer", "viewer | editor | admin")
	invite.Flags().StringVar(&displayName, "display-name", "", "display name")
	cmd.AddCommand(invite)

	var roleOrg, principal, newRole string
	updateRole := &cobra.Command{Use: "update-role", Short: "Change a member's role", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.iamClient()
		if err != nil {
			return err
		}
		resp, err := client.UpdateOrgMemberRole(cmd.Context(), connect.NewRequest(&iamv1.UpdateOrgMemberRoleRequest{OrganizationName: roleOrg, Principal: principal, Role: newRole}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	updateRole.Flags().StringVar(&roleOrg, "org", "", "organization (required)")
	updateRole.Flags().StringVar(&principal, "principal", "", "user resource name (required)")
	updateRole.Flags().StringVar(&newRole, "role", "", "viewer | editor | admin (required)")
	cmd.AddCommand(updateRole)

	var removeOrg, removePrincipal string
	remove := &cobra.Command{Use: "remove", Short: "Remove a member from an org", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.iamClient()
		if err != nil {
			return err
		}
		resp, err := client.RemoveOrgMember(cmd.Context(), connect.NewRequest(&iamv1.RemoveOrgMemberRequest{OrganizationName: removeOrg, Principal: removePrincipal}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	remove.Flags().StringVar(&removeOrg, "org", "", "organization (required)")
	remove.Flags().StringVar(&removePrincipal, "principal", "", "user resource name (required)")
	cmd.AddCommand(remove)

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
	cmd.AddCommand(&cobra.Command{Use: "revoke SESSION_ID", Short: "Revoke a session", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
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
	cmd.AddCommand(&cobra.Command{Use: "revoke-others", Short: "Revoke every session except the calling one", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.iamClient()
		if err != nil {
			return err
		}
		resp, err := client.RevokeAllOtherSessions(cmd.Context(), connect.NewRequest(&iamv1.RevokeAllOtherSessionsRequest{}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}})
	cmd.AddCommand(&cobra.Command{Use: "logout", Short: "Sign out the current session", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.iamClient()
		if err != nil {
			return err
		}
		resp, err := client.Logout(cmd.Context(), connect.NewRequest(&iamv1.LogoutRequest{}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}})
	return cmd
}

func newMFACommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "mfa", Short: "Manage MFA (TOTP) devices"}

	cmd.AddCommand(&cobra.Command{Use: "list", Short: "List MFA devices", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.iamClient()
		if err != nil {
			return err
		}
		resp, err := client.ListMFADevices(cmd.Context(), connect.NewRequest(&iamv1.ListMFADevicesRequest{}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}})

	var enrollDisplay string
	enroll := &cobra.Command{Use: "enroll", Short: "Start TOTP enrollment (returns provisioning URI / QR seed)", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.iamClient()
		if err != nil {
			return err
		}
		resp, err := client.EnrollMFA(cmd.Context(), connect.NewRequest(&iamv1.EnrollMFARequest{DisplayName: enrollDisplay}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	enroll.Flags().StringVar(&enrollDisplay, "display-name", "", "label for this device (e.g. 'iPhone 15')")
	cmd.AddCommand(enroll)

	var verifyName, verifyCode string
	verify := &cobra.Command{Use: "verify NAME", Short: "Confirm an MFA enrollment with a TOTP code", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		_ = verifyName
		client, err := ctx.iamClient()
		if err != nil {
			return err
		}
		resp, err := client.VerifyMFAEnrollment(cmd.Context(), connect.NewRequest(&iamv1.VerifyMFAEnrollmentRequest{Name: args[0], TotpCode: verifyCode}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	verify.Flags().StringVar(&verifyCode, "code", "", "6-digit TOTP code (required)")
	cmd.AddCommand(verify)

	cmd.AddCommand(&cobra.Command{Use: "revoke NAME", Short: "Revoke an MFA device", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.iamClient()
		if err != nil {
			return err
		}
		resp, err := client.RevokeMFADevice(cmd.Context(), connect.NewRequest(&iamv1.RevokeMFADeviceRequest{Name: args[0]}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}})

	return cmd
}

func newInvitesCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "invites", Aliases: []string{"invite"}, Short: "List, accept, and revoke org invites"}

	var pages pageFlags
	var listOrg string
	list := &cobra.Command{Use: "list", Short: "List pending invites for an org", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.iamClient()
		if err != nil {
			return err
		}
		resp, err := client.ListPendingInvites(cmd.Context(), connect.NewRequest(&iamv1.ListPendingInvitesRequest{OrganizationName: listOrg, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	addPageFlags(list, &pages)
	list.Flags().StringVar(&listOrg, "org", "", "organization (required)")
	cmd.AddCommand(list)

	var acceptToken string
	accept := &cobra.Command{Use: "accept", Short: "Accept an invite token", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.iamClient()
		if err != nil {
			return err
		}
		resp, err := client.AcceptInvite(cmd.Context(), connect.NewRequest(&iamv1.AcceptInviteRequest{AcceptToken: acceptToken}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	accept.Flags().StringVar(&acceptToken, "token", "", "accept token from the invite email (required)")
	cmd.AddCommand(accept)

	var revokeOrg, revokeInviteID string
	revoke := &cobra.Command{Use: "revoke", Short: "Revoke a pending invite", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.iamClient()
		if err != nil {
			return err
		}
		resp, err := client.RevokeInvite(cmd.Context(), connect.NewRequest(&iamv1.RevokeInviteRequest{OrganizationName: revokeOrg, InviteId: revokeInviteID}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	revoke.Flags().StringVar(&revokeOrg, "org", "", "organization (required)")
	revoke.Flags().StringVar(&revokeInviteID, "invite-id", "", "invite id from `invites list` (required)")
	cmd.AddCommand(revoke)

	return cmd
}

func newUserCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "user", Short: "Manage your user record"}

	cmd.AddCommand(&cobra.Command{Use: "get [NAME]", Short: "Get user (no NAME = caller)", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		name := ""
		if len(args) == 1 {
			name = args[0]
		}
		client, err := ctx.iamClient()
		if err != nil {
			return err
		}
		resp, err := client.GetUser(cmd.Context(), connect.NewRequest(&iamv1.GetUserRequest{Name: name}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}})

	var updateName, updateDisplay string
	update := &cobra.Command{Use: "update NAME", Short: "Update user display name", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		_ = updateName
		req := &iamv1.UpdateUserRequest{Name: args[0]}
		d := updateDisplay
		req.DisplayName = &d
		client, err := ctx.iamClient()
		if err != nil {
			return err
		}
		resp, err := client.UpdateUser(cmd.Context(), connect.NewRequest(req))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	update.Flags().StringVar(&updateDisplay, "display-name", "", "new display name (required)")
	cmd.AddCommand(update)

	var emailNew string
	emailReq := &cobra.Command{Use: "request-email-change", Short: "Inline-swap to a new email address (sends notice to old address)", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.iamClient()
		if err != nil {
			return err
		}
		resp, err := client.RequestEmailChange(cmd.Context(), connect.NewRequest(&iamv1.RequestEmailChangeRequest{NewEmail: emailNew}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	emailReq.Flags().StringVar(&emailNew, "new-email", "", "new email address (required)")
	cmd.AddCommand(emailReq)

	return cmd
}

func newPasswordCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "password", Short: "Change / reset password"}

	var curr, next string
	change := &cobra.Command{Use: "change", Short: "Change password (requires current password)", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.iamClient()
		if err != nil {
			return err
		}
		resp, err := client.ChangePassword(cmd.Context(), connect.NewRequest(&iamv1.ChangePasswordRequest{CurrentPassword: curr, NewPassword: next}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	change.Flags().StringVar(&curr, "current", "", "current password (required)")
	change.Flags().StringVar(&next, "new", "", "new password (required)")
	cmd.AddCommand(change)

	var setInit string
	setInitial := &cobra.Command{Use: "set-initial", Short: "Set the initial password (for OIDC users adopting a password)", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.iamClient()
		if err != nil {
			return err
		}
		resp, err := client.SetInitialPassword(cmd.Context(), connect.NewRequest(&iamv1.SetInitialPasswordRequest{NewPassword: setInit}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	setInitial.Flags().StringVar(&setInit, "new", "", "new password (required)")
	cmd.AddCommand(setInitial)

	var resetEmail string
	requestReset := &cobra.Command{Use: "forgot", Short: "Send a password-reset email", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.iamClient()
		if err != nil {
			return err
		}
		resp, err := client.RequestPasswordReset(cmd.Context(), connect.NewRequest(&iamv1.RequestPasswordResetRequest{Email: resetEmail}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	requestReset.Flags().StringVar(&resetEmail, "email", "", "account email (required)")
	cmd.AddCommand(requestReset)

	var resetToken, resetNew string
	reset := &cobra.Command{Use: "reset", Short: "Reset password with a token from the email", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.iamClient()
		if err != nil {
			return err
		}
		resp, err := client.ResetPassword(cmd.Context(), connect.NewRequest(&iamv1.ResetPasswordRequest{Token: resetToken, NewPassword: resetNew}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	reset.Flags().StringVar(&resetToken, "token", "", "reset token from email (required)")
	reset.Flags().StringVar(&resetNew, "new", "", "new password (required)")
	cmd.AddCommand(reset)

	return cmd
}

func newNotificationsCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "notifications", Aliases: []string{"notifs"}, Short: "Bell-icon notification feed"}

	var pages pageFlags
	var listOrg string
	list := &cobra.Command{Use: "list", Short: "List notifications for an org", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.iamClient()
		if err != nil {
			return err
		}
		resp, err := client.ListNotifications(cmd.Context(), connect.NewRequest(&iamv1.ListNotificationsRequest{OrganizationName: listOrg, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	addPageFlags(list, &pages)
	list.Flags().StringVar(&listOrg, "org", "", "organization (defaults to primary org)")
	cmd.AddCommand(list)

	var markOrg string
	var markIDs []string
	var markAll bool
	mark := &cobra.Command{Use: "mark-read", Short: "Mark notifications as read", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.iamClient()
		if err != nil {
			return err
		}
		resp, err := client.MarkNotificationsRead(cmd.Context(), connect.NewRequest(&iamv1.MarkNotificationsReadRequest{OrganizationName: markOrg, NotificationIds: markIDs, MarkAll: markAll}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	mark.Flags().StringVar(&markOrg, "org", "", "organization (defaults to primary org)")
	mark.Flags().StringSliceVar(&markIDs, "id", nil, "specific notification ids (repeatable)")
	mark.Flags().BoolVar(&markAll, "all", false, "mark every visible notification read")
	cmd.AddCommand(mark)

	return cmd
}

func newSshKeyImportCommand(opts *rootOptions) *cobra.Command {
	var project, username string
	var useLinked bool
	var prefix string
	cmd := &cobra.Command{Use: "import-github-ssh-keys", Short: "Bulk-import a GitHub user's public SSH keys into a project", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		projectName, err := requireProject(ctx, project)
		if err != nil {
			return err
		}
		client, err := ctx.iamClient()
		if err != nil {
			return err
		}
		resp, err := client.ImportGitHubSshKeys(cmd.Context(), connect.NewRequest(&iamv1.ImportGitHubSshKeysRequest{
			ProjectName:       projectName,
			GithubUsername:    username,
			UseLinkedIdentity: useLinked,
			DisplayNamePrefix: prefix,
		}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	cmd.Flags().StringVar(&project, "project", "", "destination project (defaults to active project)")
	cmd.Flags().StringVar(&username, "github-username", "", "GitHub username (required unless --use-linked-identity)")
	cmd.Flags().BoolVar(&useLinked, "use-linked-identity", false, "resolve username from caller's linked OIDC identity")
	cmd.Flags().StringVar(&prefix, "display-name-prefix", "", "display-name prefix for the imported keys (default 'github-<username>-')")
	return cmd
}
