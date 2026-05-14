package command

import (
	"fmt"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	networkv1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/network/v1"
)

func newNetworkCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "network", Aliases: []string{"networks"}, Short: "Manage tenant networks"}
	cmd.AddCommand(newTenantNetworkCommand(opts)...)
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
		resp, err := client.ListNetworks(cmd.Context(), connect.NewRequest(&networkv1.ListNetworksRequest{ProjectName: projectName, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	addPageFlags(list, &pages)
	list.Flags().StringVar(&project, "project", "", "project")

	var createProject, networkID, region, display string
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
		resp, err := client.CreateNetwork(cmd.Context(), connect.NewRequest(&networkv1.CreateNetworkRequest{ProjectName: projectName, NetworkId: networkID, DisplayName: display, DatacenterName: region}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	create.Flags().StringVar(&createProject, "project", "", "project")
	create.Flags().StringVar(&networkID, "id", "", "network id (optional, auto-minted if empty)")
	create.Flags().StringVar(&region, "region", "", "datacenter/region")
	create.Flags().StringVar(&display, "display-name", "", "display name")

	return []*cobra.Command{list, create}
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
	list.Flags().StringVar(&vm, "vm", "", "VM resource name (required)")
	cmd.AddCommand(list)

	var (
		createProject, createDC, createVM, createDir, createDisplay string
		createSources, createDestinations, createPorts              []string
	)
	create := &cobra.Command{Use: "create", Short: "Create a firewall rule", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		projectName, err := requireProject(ctx, createProject)
		if err != nil {
			return err
		}
		if createDC == "" {
			createDC = ctx.profile.Region
		}
		ports, err := parsePortMappings(createPorts)
		if err != nil {
			return err
		}
		client, err := ctx.networkClient()
		if err != nil {
			return err
		}
		resp, err := client.CreateFirewallRule(cmd.Context(), connect.NewRequest(&networkv1.CreateFirewallRuleRequest{
			ProjectName:    projectName,
			DatacenterName: createDC,
			TargetVm:       createVM,
			Ports:          ports,
			Sources:        createSources,
			Destinations:   createDestinations,
			Direction:      createDir,
			DisplayName:    createDisplay,
		}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	create.Flags().StringVar(&createProject, "project", "", "project (defaults to active project)")
	create.Flags().StringVar(&createDC, "datacenter", "", "datacenter (defaults to profile region)")
	create.Flags().StringVar(&createVM, "vm", "", "target VM resource name (required for ingress rules)")
	create.Flags().StringVar(&createDir, "direction", "ingress", "ingress | egress")
	create.Flags().StringVar(&createDisplay, "display-name", "", "display name")
	create.Flags().StringSliceVar(&createSources, "source", nil, "source CIDR or VM (repeatable)")
	create.Flags().StringSliceVar(&createDestinations, "destination", nil, "destination CIDR (egress only; repeatable)")
	create.Flags().StringSliceVar(&createPorts, "port", nil, "PORT[/PROTO][-END] e.g. 22/tcp, 30000-32767/tcp (repeatable)")
	cmd.AddCommand(create)

	del := &cobra.Command{Use: "delete NAME", Short: "Delete a firewall rule", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.networkClient()
		if err != nil {
			return err
		}
		resp, err := client.DeleteFirewallRule(cmd.Context(), connect.NewRequest(&networkv1.DeleteFirewallRuleRequest{Name: args[0]}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	cmd.AddCommand(del)
	return cmd
}

// parsePortMappings parses CLI --port flags. Each value is PORT[-END][/PROTO].
// Default proto is tcp. END is optional (single port when omitted).
func parsePortMappings(raw []string) ([]*networkv1.PortMapping, error) {
	out := make([]*networkv1.PortMapping, 0, len(raw))
	for _, s := range raw {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		proto := "tcp"
		if i := strings.IndexByte(s, '/'); i >= 0 {
			proto = strings.ToLower(strings.TrimSpace(s[i+1:]))
			s = s[:i]
		}
		var startStr, endStr string
		if i := strings.IndexByte(s, '-'); i >= 0 {
			startStr, endStr = s[:i], s[i+1:]
		} else {
			startStr = s
		}
		start, err := strconv.Atoi(strings.TrimSpace(startStr))
		if err != nil || start < 1 || start > 65535 {
			return nil, fmt.Errorf("invalid port %q: must be 1..65535", startStr)
		}
		pm := &networkv1.PortMapping{Port: int32(start), Protocol: proto}
		if endStr != "" {
			end, err := strconv.Atoi(strings.TrimSpace(endStr))
			if err != nil || end < start || end > 65535 {
				return nil, fmt.Errorf("invalid end port %q: must be >= start and <= 65535", endStr)
			}
			pm.EndPort = int32(end)
		}
		out = append(out, pm)
	}
	return out, nil
}
