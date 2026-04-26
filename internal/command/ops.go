package command

import (
	"time"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	opsv1 "github.com/AES-Services/foundry-sdk/gen/go/aes/ops/v1"
)

func newOpsCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "ops", Aliases: []string{"operations"}, Short: "Inspect long-running operations"}
	var pages pageFlags
	var project string
	list := &cobra.Command{
		Use:   "list",
		Short: "List operations",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			projectName, err := requireProject(ctx, project)
			if err != nil {
				return err
			}
			client, err := ctx.opsClient()
			if err != nil {
				return err
			}
			resp, err := client.ListOperations(cmd.Context(), connect.NewRequest(&opsv1.ListOperationsRequest{ProjectName: projectName, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}
	addPageFlags(list, &pages)
	list.Flags().StringVar(&project, "project", "", "project scope")
	cmd.AddCommand(list)
	cmd.AddCommand(&cobra.Command{
		Use:   "get NAME",
		Short: "Get operation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.opsClient()
			if err != nil {
				return err
			}
			resp, err := client.GetOperation(cmd.Context(), connect.NewRequest(&opsv1.GetOperationRequest{Name: args[0]}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "cancel NAME",
		Short: "Cancel operation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.opsClient()
			if err != nil {
				return err
			}
			resp, err := client.CancelOperation(cmd.Context(), connect.NewRequest(&opsv1.CancelOperationRequest{Name: args[0]}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	})
	var timeout time.Duration
	wait := &cobra.Command{
		Use:   "wait NAME",
		Short: "Poll until operation reaches a terminal state",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.opsClient()
			if err != nil {
				return err
			}
			deadline := time.Now().Add(timeout)
			for {
				resp, err := client.GetOperation(cmd.Context(), connect.NewRequest(&opsv1.GetOperationRequest{Name: args[0]}))
				if err != nil {
					return err
				}
				state := resp.Msg.GetOperation().GetState()
				if state == opsv1.State_STATE_SUCCEEDED || state == opsv1.State_STATE_FAILED || state == opsv1.State_STATE_CANCELLED {
					return ctx.write(resp.Msg)
				}
				if timeout > 0 && time.Now().After(deadline) {
					return ctx.write(resp.Msg)
				}
				time.Sleep(2 * time.Second)
			}
		},
	}
	wait.Flags().DurationVar(&timeout, "timeout", 30*time.Minute, "wait timeout")
	cmd.AddCommand(wait)
	return cmd
}
