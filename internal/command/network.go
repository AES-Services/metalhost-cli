package command

import (
	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	networkv1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/network/v1"
)

func newNetworkCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "network", Aliases: []string{"networks"}, Short: "Manage networks"}
	cmd.AddCommand(newTenantNetworkCommand(opts)...)
	cmd.AddCommand(newPublicIPCommand(opts))
	cmd.AddCommand(newFirewallCommand(opts))
	cmd.AddCommand(newLoadBalancerCommand(opts))
	cmd.AddCommand(newDNSCommand(opts))
	return cmd
}

func newTenantNetworkCommand(opts *rootOptions) []*cobra.Command {
	var pages pageFlags
	var project string
	list := &cobra.Command{Use: "list", Short: "List networks", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		projectName, err := requireProject(ctx, project)
		if err != nil {
			return err
		}
		client, err := ctx.networkClient()
		if err != nil {
			return err
		}
		resp, err := client.ListNetworks(cmd.Context(), connect.NewRequest(&networkv1.ListNetworksRequest{ProjectId: projectName, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	addPageFlags(list, &pages)
	list.Flags().StringVar(&project, "project", "", "project")
	get := &cobra.Command{Use: "get NAME", Short: "Get network", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.networkClient()
		if err != nil {
			return err
		}
		resp, err := client.GetNetwork(cmd.Context(), connect.NewRequest(&networkv1.GetNetworkRequest{Name: args[0]}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	var createProject, networkID, region, cidr, display string
	var vlan int32
	create := &cobra.Command{Use: "create", Short: "Create network", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		projectName, err := requireProject(ctx, createProject)
		if err != nil {
			return err
		}
		if region == "" {
			region = ctx.profile.Region
		}
		client, err := ctx.networkClient()
		if err != nil {
			return err
		}
		resp, err := client.CreateNetwork(cmd.Context(), connect.NewRequest(&networkv1.CreateNetworkRequest{ProjectId: projectName, NetworkId: networkID, DisplayName: display, DatacenterName: region, SubnetCidr: cidr, VlanId: vlan}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	create.Flags().StringVar(&createProject, "project", "", "project")
	create.Flags().StringVar(&networkID, "id", "", "network id")
	create.Flags().StringVar(&region, "region", "", "datacenter/region")
	create.Flags().StringVar(&cidr, "cidr", "", "subnet CIDR")
	create.Flags().StringVar(&display, "display-name", "", "display name")
	create.Flags().Int32Var(&vlan, "vlan", 0, "VLAN id")
	del := &cobra.Command{Use: "delete NAME", Short: "Delete network", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.networkClient()
		if err != nil {
			return err
		}
		resp, err := client.DeleteNetwork(cmd.Context(), connect.NewRequest(&networkv1.DeleteNetworkRequest{Name: args[0]}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	return []*cobra.Command{list, get, create, del}
}

func newPublicIPCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "ip", Aliases: []string{"ips", "public-ip"}, Short: "Manage public IPs"}
	var pages pageFlags
	var project string
	list := &cobra.Command{Use: "list", Short: "List public IPs", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		projectName, err := requireProject(ctx, project)
		if err != nil {
			return err
		}
		client, err := ctx.networkClient()
		if err != nil {
			return err
		}
		resp, err := client.ListPublicIps(cmd.Context(), connect.NewRequest(&networkv1.ListPublicIpsRequest{ProjectName: projectName, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	addPageFlags(list, &pages)
	list.Flags().StringVar(&project, "project", "", "project")
	cmd.AddCommand(list)
	cmd.AddCommand(&cobra.Command{Use: "get NAME", Short: "Get public IP", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.networkClient()
		if err != nil {
			return err
		}
		resp, err := client.GetPublicIp(cmd.Context(), connect.NewRequest(&networkv1.GetPublicIpRequest{Name: args[0]}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}})
	return cmd
}

func newFirewallCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "firewall", Short: "Manage firewall rules"}
	var pages pageFlags
	var vm string
	list := &cobra.Command{Use: "list", Short: "List firewall rules", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.networkClient()
		if err != nil {
			return err
		}
		resp, err := client.ListFirewallRules(cmd.Context(), connect.NewRequest(&networkv1.ListFirewallRulesRequest{TargetVm: vm, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	addPageFlags(list, &pages)
	list.Flags().StringVar(&vm, "vm", "", "VM name")
	cmd.AddCommand(list)
	return cmd
}

func newLoadBalancerCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "lb", Aliases: []string{"load-balancer", "load-balancers"}, Short: "Manage load balancers"}
	var pages pageFlags
	var project string
	list := &cobra.Command{Use: "list", Short: "List load balancers", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		projectName, err := requireProject(ctx, project)
		if err != nil {
			return err
		}
		client, err := ctx.networkClient()
		if err != nil {
			return err
		}
		resp, err := client.ListLoadBalancers(cmd.Context(), connect.NewRequest(&networkv1.ListLoadBalancersRequest{ProjectName: projectName, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	addPageFlags(list, &pages)
	list.Flags().StringVar(&project, "project", "", "project")
	cmd.AddCommand(list)
	return cmd
}

func newDNSCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "dns", Short: "Manage DNS records"}
	var pages pageFlags
	var network string
	list := &cobra.Command{Use: "list", Short: "List DNS records", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.networkClient()
		if err != nil {
			return err
		}
		resp, err := client.ListDnsRecords(cmd.Context(), connect.NewRequest(&networkv1.ListDnsRecordsRequest{NetworkName: network, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	addPageFlags(list, &pages)
	list.Flags().StringVar(&network, "network", "", "network name")
	cmd.AddCommand(list)
	return cmd
}
