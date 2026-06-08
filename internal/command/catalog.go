package command

import (
	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	catalogv1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/catalog/v1"
	healthv1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/health/v1"
)

func newCatalogCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "catalog", Short: "Browse datacenters and pricing"}
	cmd.AddCommand(newDatacenterCommand(opts))
	cmd.AddCommand(newPricingCommand(opts))
	cmd.AddCommand(newCapacityCommand(opts))
	return cmd
}

func newCapacityCommand(opts *rootOptions) *cobra.Command {
	var dc string
	cmd := &cobra.Command{
		Use:   "capacity",
		Short: "Show VM capacity (vCPU/RAM/GPU availability) per datacenter",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.catalogClient()
			if err != nil {
				return err
			}
			resp, err := client.GetVMCapacity(cmd.Context(), connect.NewRequest(&catalogv1.GetVMCapacityRequest{
				DatacenterName: dc,
			}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}
	cmd.Flags().StringVar(&dc, "datacenter", "", "filter to one datacenter, e.g. datacenters/us-dal-1 (default: all READY/DEGRADED)")
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
			return doList(cmd, ctx, client.ListDatacenters, &catalogv1.ListDatacentersRequest{PageSize: effectivePageSize(pages), PageToken: pages.pageToken}, pages.all)
		},
	}
	addPageFlags(list, &pages)
	cmd.AddCommand(list)
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
	cmd.AddCommand(&cobra.Command{
		Use:   "maintenance",
		Short: "List active maintenance windows across datacenters",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.catalogClient()
			if err != nil {
				return err
			}
			resp, err := client.ListMaintenanceWindows(cmd.Context(), connect.NewRequest(&catalogv1.ListMaintenanceWindowsRequest{}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	})
	return cmd
}

func newPricingCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "pricing", Short: "Quote VM configurations against current pricing"}

	var vcpus, ramGib, bootDiskGib, extraDiskGib, publicIPv4Count int32
	var cpuClass, quoteCurrency string
	quote := &cobra.Command{
		Use:   "quote",
		Short: "Quote a VM configuration (vcpus + ram + disk + optional public IPv4)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.catalogClient()
			if err != nil {
				return err
			}
			resp, err := client.QuoteVirtualMachine(cmd.Context(), connect.NewRequest(&catalogv1.QuoteVirtualMachineRequest{
				Vcpus:           vcpus,
				RamGib:          ramGib,
				CpuClass:        cpuClass,
				BootDiskGib:     bootDiskGib,
				ExtraDiskGib:    extraDiskGib,
				PublicIpv4Count: publicIPv4Count,
				Currency:        quoteCurrency,
			}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}
	quote.Flags().Int32Var(&vcpus, "vcpus", 0, "vCPU count (1-16 GiB-per-vCPU ratio enforced)")
	quote.Flags().Int32Var(&ramGib, "ram-gib", 0, "RAM GiB")
	quote.Flags().StringVar(&cpuClass, "cpu-class", "", "CPU class (e.g. cascadelake)")
	quote.Flags().Int32Var(&bootDiskGib, "boot-disk-gib", 0, "boot disk GiB")
	quote.Flags().Int32Var(&extraDiskGib, "extra-disk-gib", 0, "extra disk GiB")
	quote.Flags().Int32Var(&publicIPv4Count, "public-ipv4-count", 0, "public IPv4 count to include in the quote")
	quote.Flags().StringVar(&quoteCurrency, "currency", "", "USD (default) or USDC")
	cmd.AddCommand(quote)
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
