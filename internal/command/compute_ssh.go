package command

import (
	"errors"
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

	var createProject, keyID, displayName, publicKey, idempotencyKey string
	var labelPairs, annotationPairs []string
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
				ProjectName:    projectName,
				SshKeyId:       keyID,
				DisplayName:    displayName,
				PublicKey:      publicKey,
				Labels:         stringMapFromPairs(labelPairs),
				Annotations:    stringMapFromPairs(annotationPairs),
				IdempotencyKey: idempotencyKey,
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
	create.Flags().StringSliceVar(&annotationPairs, "annotation", nil, "annotations as key=value (repeatable)")
	create.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "client-stamped idempotency key (optional)")

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

	cmd.AddCommand(list, create, del)
	return cmd
}
