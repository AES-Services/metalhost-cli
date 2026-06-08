package command

import (
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	auditv1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/audit/v1"
	quotav1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/quota/v1"
	webhooksv1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/webhooks/v1"
)

func newQuotaCommand(opts *rootOptions) *cobra.Command {
	var project string
	cmd := &cobra.Command{Use: "quota", Short: "View quotas", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		projectName, err := requireProject(ctx, project)
		if err != nil {
			return err
		}
		client, err := ctx.quotaClient()
		if err != nil {
			return err
		}
		resp, err := client.GetMyQuotas(cmd.Context(), connect.NewRequest(&quotav1.GetMyQuotasRequest{ProjectName: projectName}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	cmd.Flags().StringVar(&project, "project", "", "project")
	return cmd
}

func newAuditCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "audit", Short: "Search audit events"}
	var pages pageFlags
	var project, action, principal string
	var since time.Duration
	search := &cobra.Command{Use: "search", Aliases: []string{"list"}, Short: "Search audit events", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		projectName, err := requireProject(ctx, project)
		if err != nil {
			return err
		}
		client, err := ctx.auditClient()
		if err != nil {
			return err
		}
		var min int64
		if since > 0 {
			min = time.Now().Add(-since).Unix()
		}
		return doList(cmd, ctx, client.SearchEvents, &auditv1.SearchEventsRequest{ProjectName: projectName, ActionPrefix: action, PrincipalPrefix: principal, MinTimestampUnix: min, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}, pages.all)
	}}
	addPageFlags(search, &pages)
	search.Flags().StringVar(&project, "project", "", "project")
	search.Flags().StringVar(&action, "action-prefix", "", "action prefix")
	search.Flags().StringVar(&principal, "principal-prefix", "", "principal prefix")
	search.Flags().DurationVar(&since, "since", 24*time.Hour, "lookback duration")
	cmd.AddCommand(search)
	return cmd
}

func newWebhooksCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "webhook", Aliases: []string{"webhooks"}, Short: "Manage webhook subscriptions"}
	var pages pageFlags
	var project string
	list := &cobra.Command{Use: "list", Short: "List webhook subscriptions", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		projectName, err := requireProject(ctx, project)
		if err != nil {
			return err
		}
		client, err := ctx.webhooksClient()
		if err != nil {
			return err
		}
		return doList(cmd, ctx, client.ListSubscriptions, &webhooksv1.ListSubscriptionsRequest{ProjectName: projectName, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}, pages.all)
	}}
	addPageFlags(list, &pages)
	list.Flags().StringVar(&project, "project", "", "project")
	cmd.AddCommand(list)
	cmd.AddCommand(&cobra.Command{Use: "get NAME", Short: "Get webhook subscription", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.webhooksClient()
		if err != nil {
			return err
		}
		resp, err := client.GetSubscription(cmd.Context(), connect.NewRequest(&webhooksv1.GetSubscriptionRequest{Name: args[0]}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}})

	var (
		createName, createProject, createEndpoint string
		createEventTypes                          []string
	)
	create := &cobra.Command{Use: "create", Short: "Create a webhook subscription (secret returned ONCE)", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		projectName, err := requireProject(ctx, createProject)
		if err != nil {
			return err
		}
		client, err := ctx.webhooksClient()
		if err != nil {
			return err
		}
		resp, err := client.CreateSubscription(cmd.Context(), connect.NewRequest(&webhooksv1.CreateSubscriptionRequest{
			Name:        createName,
			ProjectName: projectName,
			EndpointUrl: createEndpoint,
			EventTypes:  createEventTypes,
		}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	create.Flags().StringVar(&createName, "name", "", "resource name for the subscription, e.g. projects/p/webhooks/my-hook (required)")
	create.Flags().StringVar(&createProject, "project", "", "project (defaults to active project)")
	create.Flags().StringVar(&createEndpoint, "endpoint", "", "HTTPS endpoint URL Metalhost will POST to (required)")
	create.Flags().StringSliceVar(&createEventTypes, "event", nil, "event type to subscribe to, e.g. vm.created (repeatable)")
	cmd.AddCommand(create)

	var (
		updEndpoint, updState string
		updEventTypes         []string
		updReplaceEventTypes  bool
	)
	update := &cobra.Command{Use: "update NAME", Short: "Update a webhook subscription", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.webhooksClient()
		if err != nil {
			return err
		}
		req := &webhooksv1.UpdateSubscriptionRequest{Name: args[0], ReplaceEventTypes: updReplaceEventTypes, EventTypes: updEventTypes}
		if strings.TrimSpace(updEndpoint) != "" {
			s := updEndpoint
			req.EndpointUrl = &s
		}
		if strings.TrimSpace(updState) != "" {
			s := updState
			req.State = &s
		}
		resp, err := client.UpdateSubscription(cmd.Context(), connect.NewRequest(req))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	update.Flags().StringVar(&updEndpoint, "endpoint", "", "replace the HTTPS endpoint URL")
	update.Flags().StringSliceVar(&updEventTypes, "event", nil, "event type(s) (repeatable; combine with --replace-events to overwrite)")
	update.Flags().BoolVar(&updReplaceEventTypes, "replace-events", false, "replace event_types entirely (default: merge)")
	update.Flags().StringVar(&updState, "state", "", "set state to ACTIVE or DISABLED")
	cmd.AddCommand(update)

	cmd.AddCommand(&cobra.Command{Use: "delete NAME", Short: "Delete a webhook subscription", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.webhooksClient()
		if err != nil {
			return err
		}
		resp, err := client.DeleteSubscription(cmd.Context(), connect.NewRequest(&webhooksv1.DeleteSubscriptionRequest{Name: args[0]}))
		if err != nil {
			return err
		}
		return writeDeleted(cmd, ctx, "webhook", args[0], resp.Msg)
	}})

	var delPages pageFlags
	deliveries := &cobra.Command{Use: "deliveries NAME", Short: "List delivery attempts for a subscription", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.webhooksClient()
		if err != nil {
			return err
		}
		return doList(cmd, ctx, client.ListDeliveries, &webhooksv1.ListDeliveriesRequest{
			SubscriptionName: args[0],
			PageSize:         effectivePageSize(delPages),
			PageToken:        delPages.pageToken,
		}, delPages.all)
	}}
	addPageFlags(deliveries, &delPages)
	cmd.AddCommand(deliveries)

	return cmd
}
