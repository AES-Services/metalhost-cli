package command

import (
	"errors"
	"os"
	"strings"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	computev1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/compute/v1"
)

func newSSHKeyCommands(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "ssh-key", Aliases: []string{"ssh-keys"}, Short: "Manage project SSH public keys"}
	var pages pageFlags
	var project string
	list := &cobra.Command{
		Use: "list", Short: "List SSH keys",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			projectName, err := requireProject(ctx, project)
			if err != nil {
				return err
			}
			client, err := ctx.sshKeyClient()
			if err != nil {
				return err
			}
			resp, err := client.ListSSHKeys(cmd.Context(), connect.NewRequest(&computev1.ListSSHKeysRequest{
				ProjectName: projectName, PageSize: effectivePageSize(pages), PageToken: pages.pageToken,
			}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}
	addPageFlags(list, &pages)
	list.Flags().StringVar(&project, "project", "", "project")

	get := &cobra.Command{
		Use: "get NAME", Short: "Get SSH key", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.sshKeyClient()
			if err != nil {
				return err
			}
			resp, err := client.GetSSHKey(cmd.Context(), connect.NewRequest(&computev1.GetSSHKeyRequest{Name: args[0]}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}

	var createProject, keyID, displayName, publicKey string
	var labelPairs []string
	create := &cobra.Command{
		Use: "create", Short: "Register an SSH public key",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			projectName, err := requireProject(ctx, createProject)
			if err != nil {
				return err
			}
			if strings.TrimSpace(keyID) == "" || strings.TrimSpace(publicKey) == "" {
				return errors.New("--id and --public-key are required")
			}
			client, err := ctx.sshKeyClient()
			if err != nil {
				return err
			}
			resp, err := client.CreateSSHKey(cmd.Context(), connect.NewRequest(&computev1.CreateSSHKeyRequest{
				ProjectName:   projectName,
				SshKeyId:    keyID,
				DisplayName: displayName,
				PublicKey:   publicKey,
				Labels:      stringMapFromPairs(labelPairs),
			}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}
	create.Flags().StringVar(&createProject, "project", "", "project")
	create.Flags().StringVar(&keyID, "id", "", "DNS-safe key id (slug)")
	create.Flags().StringVar(&displayName, "display-name", "", "display name")
	create.Flags().StringVar(&publicKey, "public-key", "", "OpenSSH public key line")
	create.Flags().StringSliceVar(&labelPairs, "label", nil, "labels as key=value (repeatable)")

	var updDisplay, updPub string
	var clearLabels bool
	update := &cobra.Command{
		Use: "update NAME", Short: "Update SSH key", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			req := &computev1.UpdateSSHKeyRequest{Name: args[0], ClearLabels: clearLabels}
			if strings.TrimSpace(updDisplay) != "" {
				d := strings.TrimSpace(updDisplay)
				req.DisplayName = &d
			}
			if strings.TrimSpace(updPub) != "" {
				p := strings.TrimSpace(updPub)
				req.PublicKey = &p
			}
			client, err := ctx.sshKeyClient()
			if err != nil {
				return err
			}
			resp, err := client.UpdateSSHKey(cmd.Context(), connect.NewRequest(req))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}
	update.Flags().StringVar(&updDisplay, "display-name", "", "new display name")
	update.Flags().StringVar(&updPub, "public-key", "", "replace public key")
	update.Flags().BoolVar(&clearLabels, "clear-labels", false, "clear all labels")

	del := &cobra.Command{
		Use: "delete NAME", Short: "Delete SSH key", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.sshKeyClient()
			if err != nil {
				return err
			}
			resp, err := client.DeleteSSHKey(cmd.Context(), connect.NewRequest(&computev1.DeleteSSHKeyRequest{Name: args[0]}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}

	cmd.AddCommand(list, get, create, update, del)
	return cmd
}

func readSnippetBody(inline, path string) (string, error) {
	path = strings.TrimSpace(path)
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	return inline, nil
}

func newUserDataSnippetCommands(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "user-data-snippet",
		Aliases: []string{"user-data-snippets", "snippets"},
		Short:   "Manage reusable cloud-init snippets",
	}
	var pages pageFlags
	var project string
	list := &cobra.Command{
		Use: "list", Short: "List snippets",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			projectName, err := requireProject(ctx, project)
			if err != nil {
				return err
			}
			client, err := ctx.userDataSnippetClient()
			if err != nil {
				return err
			}
			resp, err := client.ListUserDataSnippets(cmd.Context(), connect.NewRequest(&computev1.ListUserDataSnippetsRequest{
				ProjectName: projectName, PageSize: effectivePageSize(pages), PageToken: pages.pageToken,
			}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}
	addPageFlags(list, &pages)
	list.Flags().StringVar(&project, "project", "", "project")

	get := &cobra.Command{
		Use: "get NAME", Short: "Get snippet", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.userDataSnippetClient()
			if err != nil {
				return err
			}
			resp, err := client.GetUserDataSnippet(cmd.Context(), connect.NewRequest(&computev1.GetUserDataSnippetRequest{Name: args[0]}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}

	var createProject, snippetID, displayName, content, contentFile, contentKind string
	var labelPairs []string
	create := &cobra.Command{
		Use: "create", Short: "Create a snippet",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			projectName, err := requireProject(ctx, createProject)
			if err != nil {
				return err
			}
			if strings.TrimSpace(snippetID) == "" {
				return errors.New("--id is required")
			}
			body, err := readSnippetBody(content, contentFile)
			if err != nil {
				return err
			}
			if strings.TrimSpace(body) == "" {
				return errors.New("snippet body is required (--content or --content-file)")
			}
			client, err := ctx.userDataSnippetClient()
			if err != nil {
				return err
			}
			resp, err := client.CreateUserDataSnippet(cmd.Context(), connect.NewRequest(&computev1.CreateUserDataSnippetRequest{
				ProjectName:   projectName,
				SnippetId:   snippetID,
				DisplayName: displayName,
				Content:     body,
				ContentKind: contentKind,
				Labels:      stringMapFromPairs(labelPairs),
			}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}
	create.Flags().StringVar(&createProject, "project", "", "project")
	create.Flags().StringVar(&snippetID, "id", "", "DNS-safe snippet id (slug)")
	create.Flags().StringVar(&displayName, "display-name", "", "display name")
	create.Flags().StringVar(&content, "content", "", "snippet body")
	create.Flags().StringVar(&contentFile, "content-file", "", "read body from file")
	create.Flags().StringVar(&contentKind, "content-kind", "", "cloud-config | shell-script | include-url (optional; server may infer)")

	var updDisplay, updContent, updContentFile, updKind string
	var clearSnippetLabels bool
	update := &cobra.Command{
		Use: "update NAME", Short: "Update snippet", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			req := &computev1.UpdateUserDataSnippetRequest{Name: args[0], ClearLabels: clearSnippetLabels}
			if strings.TrimSpace(updDisplay) != "" {
				d := strings.TrimSpace(updDisplay)
				req.DisplayName = &d
			}
			body, err := readSnippetBody(updContent, updContentFile)
			if err != nil {
				return err
			}
			if strings.TrimSpace(body) != "" {
				req.Content = &body
			}
			if strings.TrimSpace(updKind) != "" {
				k := strings.TrimSpace(updKind)
				req.ContentKind = &k
			}
			client, err := ctx.userDataSnippetClient()
			if err != nil {
				return err
			}
			resp, err := client.UpdateUserDataSnippet(cmd.Context(), connect.NewRequest(req))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}
	update.Flags().StringVar(&updDisplay, "display-name", "", "new display name")
	update.Flags().StringVar(&updContent, "content", "", "replace body")
	update.Flags().StringVar(&updContentFile, "content-file", "", "replace body from file")
	update.Flags().StringVar(&updKind, "content-kind", "", "content kind")
	update.Flags().BoolVar(&clearSnippetLabels, "clear-labels", false, "clear all labels")

	del := &cobra.Command{
		Use: "delete NAME", Short: "Delete snippet", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.userDataSnippetClient()
			if err != nil {
				return err
			}
			resp, err := client.DeleteUserDataSnippet(cmd.Context(), connect.NewRequest(&computev1.DeleteUserDataSnippetRequest{Name: args[0]}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}

	cmd.AddCommand(list, get, create, update, del)
	return cmd
}
