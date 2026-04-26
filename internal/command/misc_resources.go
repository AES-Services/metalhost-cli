package command

import (
	"time"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	auditv1 "github.com/AES-Services/foundry-sdk/gen/go/aes/audit/v1"
	baremetalv1 "github.com/AES-Services/foundry-sdk/gen/go/aes/baremetal/v1"
	quotav1 "github.com/AES-Services/foundry-sdk/gen/go/aes/quota/v1"
	walletv1 "github.com/AES-Services/foundry-sdk/gen/go/aes/wallet/v1"
	webhooksv1 "github.com/AES-Services/foundry-sdk/gen/go/aes/webhooks/v1"
)

func newWalletCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "wallet", Aliases: []string{"billing"}, Short: "View billing and wallet resources"}
	cmd.AddCommand(&cobra.Command{Use: "rates", Short: "List public meter rates", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.walletClient()
		if err != nil {
			return err
		}
		resp, err := client.ListPublicMeterRates(cmd.Context(), connect.NewRequest(&walletv1.ListPublicMeterRatesRequest{PageSize: 100}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}})
	var pages pageFlags
	var billing string
	wallets := &cobra.Command{Use: "list", Short: "List wallets", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.walletClient()
		if err != nil {
			return err
		}
		resp, err := client.ListWallets(cmd.Context(), connect.NewRequest(&walletv1.ListWalletsRequest{BillingAccountName: billing, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	addPageFlags(wallets, &pages)
	wallets.Flags().StringVar(&billing, "billing-account", "", "billing account")
	cmd.AddCommand(wallets)
	cmd.AddCommand(&cobra.Command{Use: "get NAME", Short: "Get wallet", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.walletClient()
		if err != nil {
			return err
		}
		resp, err := client.GetWallet(cmd.Context(), connect.NewRequest(&walletv1.GetWalletRequest{Name: args[0]}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}})
	return cmd
}

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
		resp, err := client.SearchEvents(cmd.Context(), connect.NewRequest(&auditv1.SearchEventsRequest{Project: projectName, ActionPrefix: action, PrincipalPrefix: principal, MinTimestampUnix: min, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	addPageFlags(search, &pages)
	search.Flags().StringVar(&project, "project", "", "project")
	search.Flags().StringVar(&action, "action-prefix", "", "action prefix")
	search.Flags().StringVar(&principal, "principal-prefix", "", "principal prefix")
	search.Flags().DurationVar(&since, "since", 24*time.Hour, "lookback duration")
	cmd.AddCommand(search)
	return cmd
}

func newBareMetalCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "baremetal", Aliases: []string{"bare-metal"}, Short: "Manage bare-metal instances"}
	var pages pageFlags
	var project string
	list := &cobra.Command{Use: "list", Short: "List bare-metal instances", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		projectName, err := requireProject(ctx, project)
		if err != nil {
			return err
		}
		client, err := ctx.bareMetalClient()
		if err != nil {
			return err
		}
		resp, err := client.ListBareMetalInstances(cmd.Context(), connect.NewRequest(&baremetalv1.ListBareMetalInstancesRequest{ProjectName: projectName, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	addPageFlags(list, &pages)
	list.Flags().StringVar(&project, "project", "", "project")
	cmd.AddCommand(list)
	cmd.AddCommand(&cobra.Command{Use: "get NAME", Short: "Get bare-metal instance", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.bareMetalClient()
		if err != nil {
			return err
		}
		resp, err := client.GetBareMetalInstance(cmd.Context(), connect.NewRequest(&baremetalv1.GetBareMetalInstanceRequest{Name: args[0]}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}})
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
		resp, err := client.ListSubscriptions(cmd.Context(), connect.NewRequest(&webhooksv1.ListSubscriptionsRequest{ProjectName: projectName, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
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
	return cmd
}
