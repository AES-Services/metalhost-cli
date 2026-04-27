package command

import (
	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	catalogv1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/catalog/v1"
	healthv1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/health/v1"
)

func newCatalogCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "catalog", Short: "Browse regions and SKUs"}
	cmd.AddCommand(newDatacenterCommand(opts))
	cmd.AddCommand(newSKUCommand(opts))
	return cmd
}

func newDatacenterCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "datacenter", Aliases: []string{"datacenters", "region", "regions"}, Short: "Browse datacenters"}
	var pages pageFlags
	list := &cobra.Command{
		Use:   "list",
		Short: "List datacenters",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.catalogClient()
			if err != nil {
				return err
			}
			resp, err := client.ListDatacenters(cmd.Context(), connect.NewRequest(&catalogv1.ListDatacentersRequest{PageSize: effectivePageSize(pages), PageToken: pages.pageToken}))
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
		Short: "Get a datacenter",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.catalogClient()
			if err != nil {
				return err
			}
			resp, err := client.GetDatacenter(cmd.Context(), connect.NewRequest(&catalogv1.GetDatacenterRequest{Name: args[0]}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "health NAME",
		Short: "Get datacenter health",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.catalogClient()
			if err != nil {
				return err
			}
			resp, err := client.GetRegionHealth(cmd.Context(), connect.NewRequest(&catalogv1.GetRegionHealthRequest{DatacenterName: args[0]}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	})
	return cmd
}

func newSKUCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "sku", Aliases: []string{"skus"}, Short: "Browse SKUs"}
	var pages pageFlags
	list := &cobra.Command{
		Use:   "list",
		Short: "List SKUs",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.catalogClient()
			if err != nil {
				return err
			}
			resp, err := client.ListSkus(cmd.Context(), connect.NewRequest(&catalogv1.ListSkusRequest{PageSize: effectivePageSize(pages), PageToken: pages.pageToken}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}
	addPageFlags(list, &pages)
	cmd.AddCommand(list)
	return cmd
}

func newHealthCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Check API health",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.healthClient()
			if err != nil {
				return err
			}
			resp, err := client.Check(cmd.Context(), connect.NewRequest(&healthv1.CheckRequest{}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}
}
