package command

import (
	"time"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	walletv1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/wallet/v1"
)

func newWalletCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "wallet", Aliases: []string{"billing"}, Short: "Billing accounts, wallets, top-ups, payments, invoices, usage"}
	cmd.AddCommand(newWalletAccountCommand(opts))
	cmd.AddCommand(newWalletBalanceCommand(opts))
	cmd.AddCommand(newWalletGetCommand(opts))
	cmd.AddCommand(newWalletRatesCommand(opts))
	cmd.AddCommand(newWalletTopUpCommand(opts))
	cmd.AddCommand(newWalletPaymentMethodCommand(opts))
	cmd.AddCommand(newWalletAutoRechargeCommand(opts))
	cmd.AddCommand(newWalletAlertsCommand(opts))
	cmd.AddCommand(newWalletInvoiceCommand(opts))
	cmd.AddCommand(newWalletUsageCommand(opts))
	cmd.AddCommand(newWalletForecastCommand(opts))
	return cmd
}

func newWalletAccountCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "account", Aliases: []string{"accounts"}, Short: "Billing accounts"}
	var pages pageFlags
	list := &cobra.Command{Use: "list", Short: "List billing accounts", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.walletClient()
		if err != nil {
			return err
		}
		return doList(cmd, ctx, client.ListBillingAccounts, &walletv1.ListBillingAccountsRequest{PageSize: effectivePageSize(pages), PageToken: pages.pageToken}, pages.all)
	}}
	addPageFlags(list, &pages)
	cmd.AddCommand(list)

	cmd.AddCommand(&cobra.Command{Use: "get NAME", Short: "Get a billing account", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.walletClient()
		if err != nil {
			return err
		}
		resp, err := client.GetBillingAccount(cmd.Context(), connect.NewRequest(&walletv1.GetBillingAccountRequest{Name: args[0]}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}})
	return cmd
}

func newWalletGetCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "get NAME", Short: "Get a wallet", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
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
	}}
}

func newWalletBalanceCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "balance NAME", Short: "Show available + held balance for a wallet", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.walletClient()
		if err != nil {
			return err
		}
		resp, err := client.GetWalletBalance(cmd.Context(), connect.NewRequest(&walletv1.GetWalletBalanceRequest{Name: args[0]}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
}

func newWalletRatesCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "rates", Short: "List public meter rates", RunE: func(cmd *cobra.Command, _ []string) error {
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
	}}
}

func newWalletTopUpCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "top-up", Aliases: []string{"topup"}, Short: "Top-up flows (Stripe + x402) and history"}

	var pages pageFlags
	var topUpState string
	list := &cobra.Command{Use: "list WALLET", Short: "List top-ups for a wallet", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.walletClient()
		if err != nil {
			return err
		}
		return doList(cmd, ctx, client.ListTopUps, &walletv1.ListTopUpsRequest{WalletName: args[0], State: topUpState, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}, pages.all)
	}}
	addPageFlags(list, &pages)
	list.Flags().StringVar(&topUpState, "state", "", "filter by top-up state")
	cmd.AddCommand(list)

	cmd.AddCommand(&cobra.Command{Use: "get NAME", Short: "Get a top-up by name", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.walletClient()
		if err != nil {
			return err
		}
		resp, err := client.GetTopUp(cmd.Context(), connect.NewRequest(&walletv1.GetTopUpRequest{Name: args[0]}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}})

	var stripeAmount int64
	var stripeWallet, stripeCurrency, stripePM, stripeIdem string
	stripe := &cobra.Command{Use: "stripe", Short: "Create a Stripe PaymentIntent for a top-up", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.walletClient()
		if err != nil {
			return err
		}
		resp, err := client.CreateStripeTopUpIntent(cmd.Context(), connect.NewRequest(&walletv1.CreateStripeTopUpIntentRequest{
			WalletName:        stripeWallet,
			AmountMinor:       stripeAmount,
			Currency:          stripeCurrency,
			PaymentMethodName: stripePM,
			IdempotencyKey:    stripeIdem,
		}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	stripe.Flags().StringVar(&stripeWallet, "wallet", "", "wallet resource name (required)")
	stripe.Flags().Int64Var(&stripeAmount, "amount-minor", 0, "amount in minor units (cents) — required")
	stripe.Flags().StringVar(&stripeCurrency, "currency", "USD", "USD or USDC")
	stripe.Flags().StringVar(&stripePM, "payment-method", "", "saved payment method name (optional)")
	stripe.Flags().StringVar(&stripeIdem, "idempotency-key", "", "client-stamped idempotency key (optional)")
	cmd.AddCommand(stripe)

	var cbAmount int64
	var cbWallet, cbCurrency, cbIdem string
	coinbase := &cobra.Command{Use: "coinbase", Aliases: []string{"crypto", "usdc"}, Short: "Create a Coinbase crypto top-up checkout (settled in USDC)", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.walletClient()
		if err != nil {
			return err
		}
		resp, err := client.CreateCoinbaseTopUpCheckout(cmd.Context(), connect.NewRequest(&walletv1.CreateCoinbaseTopUpCheckoutRequest{
			WalletName:     cbWallet,
			AmountMinor:    cbAmount,
			Currency:       cbCurrency,
			IdempotencyKey: cbIdem,
		}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	coinbase.Flags().StringVar(&cbWallet, "wallet", "", "wallet resource name (required)")
	coinbase.Flags().Int64Var(&cbAmount, "amount-minor", 0, "amount in minor units (cents); min 500 — required")
	coinbase.Flags().StringVar(&cbCurrency, "currency", "USD", "currency (USD; settled 1:1 in USDC)")
	coinbase.Flags().StringVar(&cbIdem, "idempotency-key", "", "client-stamped idempotency key (optional)")
	cmd.AddCommand(coinbase)

	return cmd
}

func newWalletPaymentMethodCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "payment-method", Aliases: []string{"pm"}, Short: "Manage saved payment methods (Stripe)"}

	var pages pageFlags
	var billing string
	list := &cobra.Command{Use: "list", Short: "List saved payment methods on a billing account", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.walletClient()
		if err != nil {
			return err
		}
		return doList(cmd, ctx, client.ListPaymentMethods, &walletv1.ListPaymentMethodsRequest{BillingAccountName: billing, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}, pages.all)
	}}
	addPageFlags(list, &pages)
	list.Flags().StringVar(&billing, "billing-account", "", "billing account resource name (required)")
	cmd.AddCommand(list)

	var setupBilling, setupIdem string
	setup := &cobra.Command{Use: "setup-intent", Short: "Create a Stripe SetupIntent to capture a new card", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.walletClient()
		if err != nil {
			return err
		}
		resp, err := client.CreateCardSetupIntent(cmd.Context(), connect.NewRequest(&walletv1.CreateCardSetupIntentRequest{BillingAccountName: setupBilling, IdempotencyKey: setupIdem}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	setup.Flags().StringVar(&setupBilling, "billing-account", "", "billing account resource name (required)")
	setup.Flags().StringVar(&setupIdem, "idempotency-key", "", "client-stamped idempotency key (optional)")
	cmd.AddCommand(setup)

	var attachName, attachBilling, attachStripePM string
	var attachDefault bool
	attach := &cobra.Command{Use: "attach", Short: "Attach a Stripe payment method to a billing account", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.walletClient()
		if err != nil {
			return err
		}
		resp, err := client.AttachPaymentMethod(cmd.Context(), connect.NewRequest(&walletv1.AttachPaymentMethodRequest{
			Name:                  attachName,
			BillingAccountName:    attachBilling,
			StripePaymentMethodId: attachStripePM,
			MakeDefault:           attachDefault,
		}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	attach.Flags().StringVar(&attachName, "name", "", "resource name for the new payment method (required)")
	attach.Flags().StringVar(&attachBilling, "billing-account", "", "billing account resource name (required)")
	attach.Flags().StringVar(&attachStripePM, "stripe-payment-method", "", "Stripe pm_ ID from confirmed SetupIntent (required)")
	attach.Flags().BoolVar(&attachDefault, "default", false, "make this the default payment method")
	cmd.AddCommand(attach)

	cmd.AddCommand(&cobra.Command{Use: "default NAME", Short: "Set a payment method as default for its billing account", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.walletClient()
		if err != nil {
			return err
		}
		resp, err := client.SetDefaultPaymentMethod(cmd.Context(), connect.NewRequest(&walletv1.SetDefaultPaymentMethodRequest{Name: args[0]}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}})

	cmd.AddCommand(&cobra.Command{Use: "detach NAME", Short: "Detach a payment method", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.walletClient()
		if err != nil {
			return err
		}
		resp, err := client.DetachPaymentMethod(cmd.Context(), connect.NewRequest(&walletv1.DetachPaymentMethodRequest{Name: args[0]}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}})

	return cmd
}

func newWalletAutoRechargeCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "auto-recharge", Short: "Configure auto-recharge rules (top up when balance drops below threshold)"}

	cmd.AddCommand(&cobra.Command{Use: "get WALLET", Short: "Get auto-recharge config for a wallet", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.walletClient()
		if err != nil {
			return err
		}
		resp, err := client.GetAutoRechargeConfig(cmd.Context(), connect.NewRequest(&walletv1.GetAutoRechargeConfigRequest{WalletName: args[0]}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}})

	var arEnabled bool
	var arThreshold, arAmount int64
	var arPM string
	configure := &cobra.Command{Use: "configure WALLET", Short: "Enable / update / disable auto-recharge", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.walletClient()
		if err != nil {
			return err
		}
		resp, err := client.ConfigureAutoRecharge(cmd.Context(), connect.NewRequest(&walletv1.ConfigureAutoRechargeRequest{
			WalletName:        args[0],
			Enabled:           arEnabled,
			ThresholdMinor:    arThreshold,
			AmountMinor:       arAmount,
			PaymentMethodName: arPM,
		}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	configure.Flags().BoolVar(&arEnabled, "enabled", false, "enable auto-recharge")
	configure.Flags().Int64Var(&arThreshold, "threshold-minor", 0, "balance threshold in minor units (cents)")
	configure.Flags().Int64Var(&arAmount, "amount-minor", 0, "recharge amount in minor units (cents)")
	configure.Flags().StringVar(&arPM, "payment-method", "", "payment method resource name to charge")
	cmd.AddCommand(configure)

	return cmd
}

func newWalletAlertsCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "alerts", Short: "Configure low-balance / days-remaining alerts"}

	cmd.AddCommand(&cobra.Command{Use: "get WALLET", Short: "Get current alert thresholds", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.walletClient()
		if err != nil {
			return err
		}
		resp, err := client.GetWalletAlerts(cmd.Context(), connect.NewRequest(&walletv1.GetWalletAlertsRequest{WalletName: args[0]}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}})

	var balanceAlert int64
	var daysAlert int32
	var clearBalance, clearDays bool
	configure := &cobra.Command{Use: "configure WALLET", Short: "Set or clear alert thresholds", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.walletClient()
		if err != nil {
			return err
		}
		req := &walletv1.ConfigureWalletAlertsRequest{WalletName: args[0]}
		if !clearBalance {
			b := balanceAlert
			req.BalanceAlertMinor = &b
		}
		if !clearDays {
			d := daysAlert
			req.DaysRemainingAlert = &d
		}
		resp, err := client.ConfigureWalletAlerts(cmd.Context(), connect.NewRequest(req))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	configure.Flags().Int64Var(&balanceAlert, "balance-alert-minor", 0, "alert when balance drops below (minor units)")
	configure.Flags().Int32Var(&daysAlert, "days-remaining-alert", 0, "alert when runway drops below N days")
	configure.Flags().BoolVar(&clearBalance, "clear-balance-alert", false, "clear the balance alert instead of setting")
	configure.Flags().BoolVar(&clearDays, "clear-days-alert", false, "clear the days-remaining alert instead of setting")
	cmd.AddCommand(configure)

	return cmd
}

func newWalletInvoiceCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "invoice", Aliases: []string{"invoices"}, Short: "Invoices (monthly statements per billing account)"}

	var pages pageFlags
	var billing string
	list := &cobra.Command{Use: "list", Short: "List invoices for a billing account", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.walletClient()
		if err != nil {
			return err
		}
		return doList(cmd, ctx, client.ListInvoices, &walletv1.ListInvoicesRequest{BillingAccountName: billing, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}, pages.all)
	}}
	addPageFlags(list, &pages)
	list.Flags().StringVar(&billing, "billing-account", "", "billing account resource name (required)")
	cmd.AddCommand(list)

	cmd.AddCommand(&cobra.Command{Use: "get NAME", Short: "Get an invoice", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.walletClient()
		if err != nil {
			return err
		}
		resp, err := client.GetInvoice(cmd.Context(), connect.NewRequest(&walletv1.GetInvoiceRequest{Name: args[0]}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}})

	cmd.AddCommand(&cobra.Command{Use: "pdf NAME", Short: "Get a presigned URL to download the invoice PDF (1h TTL)", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.walletClient()
		if err != nil {
			return err
		}
		resp, err := client.DownloadInvoicePDF(cmd.Context(), connect.NewRequest(&walletv1.DownloadInvoicePDFRequest{Name: args[0]}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}})

	return cmd
}

func newWalletUsageCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "usage", Short: "Query + export usage data"}

	var queryProject, queryOrg, queryMeter, queryBucket string
	var queryGroupBy []string
	var querySince time.Duration
	query := &cobra.Command{Use: "query", Short: "Query usage (bucketed, optionally grouped)", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		queryOrg = orgOrDefault(ctx, queryOrg)
		if queryProject == "" && queryOrg == "" {
			projectName, err := requireProject(ctx, "")
			if err != nil {
				return err
			}
			queryProject = projectName
		}
		client, err := ctx.walletClient()
		if err != nil {
			return err
		}
		end := time.Now().Unix()
		start := time.Now().Add(-querySince).Unix()
		resp, err := client.QueryUsage(cmd.Context(), connect.NewRequest(&walletv1.QueryUsageRequest{
			ProjectName:      queryProject,
			OrganizationName: queryOrg,
			MeterPrefix:      queryMeter,
			StartTimeUnix:    start,
			EndTimeUnix:      end,
			Bucket:           queryBucket,
			GroupBy:          queryGroupBy,
		}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	query.Flags().StringVar(&queryProject, "project", "", "project scope (defaults to active project)")
	query.Flags().StringVar(&queryOrg, "org", "", "organization scope (alternative to --project)")
	query.Flags().StringVar(&queryMeter, "meter-prefix", "", "filter by meter prefix (optional)")
	query.Flags().StringVar(&queryBucket, "bucket", "day", "hour | day | month")
	query.Flags().StringSliceVar(&queryGroupBy, "group-by", nil, "group by: project, meter, resource, datacenter (repeatable)")
	query.Flags().DurationVar(&querySince, "since", 7*24*time.Hour, "lookback duration")
	cmd.AddCommand(query)

	var expProject, expOrg, expMeter, expFormat string
	var expSince time.Duration
	var expRows int32
	export := &cobra.Command{Use: "export", Short: "Export usage to a presigned CSV/NDJSON URL", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		expOrg = orgOrDefault(ctx, expOrg)
		if expProject == "" && expOrg == "" {
			projectName, err := requireProject(ctx, "")
			if err != nil {
				return err
			}
			expProject = projectName
		}
		client, err := ctx.walletClient()
		if err != nil {
			return err
		}
		end := time.Now().Unix()
		start := time.Now().Add(-expSince).Unix()
		resp, err := client.ExportUsage(cmd.Context(), connect.NewRequest(&walletv1.ExportUsageRequest{
			ProjectName:      expProject,
			OrganizationName: expOrg,
			MeterPrefix:      expMeter,
			StartTimeUnix:    start,
			EndTimeUnix:      end,
			Format:           expFormat,
			MaxRows:          expRows,
		}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	export.Flags().StringVar(&expProject, "project", "", "project scope (defaults to active project)")
	export.Flags().StringVar(&expOrg, "org", "", "organization scope")
	export.Flags().StringVar(&expMeter, "meter-prefix", "", "filter by meter prefix")
	export.Flags().StringVar(&expFormat, "file-format", "csv", "exported file format: csv | ndjson")
	export.Flags().Int32Var(&expRows, "max-rows", 0, "max rows (0 = server default)")
	export.Flags().DurationVar(&expSince, "since", 30*24*time.Hour, "lookback duration")
	cmd.AddCommand(export)

	return cmd
}

func newWalletForecastCommand(opts *rootOptions) *cobra.Command {
	var billing, period string
	cmd := &cobra.Command{Use: "forecast", Short: "Month-to-date spend + cost forecast for a billing account", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.walletClient()
		if err != nil {
			return err
		}
		resp, err := client.GetCostForecast(cmd.Context(), connect.NewRequest(&walletv1.GetCostForecastRequest{BillingAccountName: billing, Period: period}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	cmd.Flags().StringVar(&billing, "billing-account", "", "billing account resource name (required)")
	cmd.Flags().StringVar(&period, "period", "", "monthly (default) | weekly | daily")
	return cmd
}
