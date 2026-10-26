package command

import (
	"errors"
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
		return doList(cmd, ctx, client.ListNetworks, &networkv1.ListNetworksRequest{ProjectName: projectName, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}, pages.all)
	}}
	addPageFlags(list, &pages)
	list.Flags().StringVar(&project, "project", "", "project")

	var createProject, region, display, subnet, requestID string
	var netLabelPairs, netAnnotationPairs []string
	create := &cobra.Command{Use: "create", Short: "Publish a VPC subnet as a network in a datacenter", Long: "Publishes an owned VPC subnet (see `metalhost vpc subnet`) as a network VMs can join in one datacenter. Pass the same --request-id to retry an uncertain create; a new ID creates another network.", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		projectName, err := requireProject(ctx, createProject)
		if err != nil {
			return err
		}
		if subnet == "" {
			return errors.New("--subnet is required (a VPC subnet resource name, e.g. projects/p/vpcs/v/subnets/s)")
		}
		if region == "" {
			region = ctx.profile.Region
		}
		if requestID == "" {
			requestID = newRequestID()
			fmt.Fprintf(cmd.ErrOrStderr(), "Request ID %s (pass --request-id %s to retry this create).\n", requestID, requestID)
		}
		client, err := ctx.networkClient()
		if err != nil {
			return err
		}
		resp, err := client.CreateNetwork(cmd.Context(), connect.NewRequest(&networkv1.CreateNetworkRequest{
			ProjectName:    projectName,
			SubnetName:     subnet,
			DisplayName:    display,
			DatacenterName: region,
			Labels:         stringMapFromPairs(netLabelPairs),
			Annotations:    stringMapFromPairs(netAnnotationPairs),
			RequestId:      requestID,
		}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	create.Flags().StringVar(&createProject, "project", "", "project")
	create.Flags().StringVar(&subnet, "subnet", "", "VPC subnet resource name (required)")
	create.Flags().StringVar(&region, "region", "", "datacenter, e.g. datacenters/us-dal-1 (defaults to profile region)")
	create.Flags().StringVar(&display, "display-name", "", "display name")
	create.Flags().StringVar(&requestID, "request-id", "", "UUID that makes retries idempotent (generated when empty)")
	create.Flags().StringSliceVar(&netLabelPairs, "label", nil, "labels as key=value (repeatable)")
	create.Flags().StringSliceVar(&netAnnotationPairs, "annotation", nil, "annotations as key=value (repeatable)")

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
		return doList(cmd, ctx, client.ListFirewallRules, &networkv1.ListFirewallRulesRequest{TargetVm: vm, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}, pages.all)
	}}
	addPageFlags(list, &pages)
	list.Flags().StringVar(&vm, "vm", "", "VM resource name (required)")
	cmd.AddCommand(list)

	var (
		createProject, createDC, createVM, createDir, createDisplay  string
		createRequestID, createAction, createFamily, createForwardIP string
		createPriority                                               int32
		createSources, createDestinations, createPorts               []string
		fwLabelPairs, fwAnnotationPairs                              []string
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
		if createRequestID == "" {
			createRequestID = newRequestID()
			fmt.Fprintf(cmd.ErrOrStderr(), "Request ID %s (pass --request-id %s to retry this create).\n", createRequestID, createRequestID)
		}
		client, err := ctx.networkClient()
		if err != nil {
			return err
		}
		resp, err := client.CreateFirewallRule(cmd.Context(), connect.NewRequest(&networkv1.CreateFirewallRuleRequest{
			ProjectName:        projectName,
			DatacenterName:     createDC,
			TargetVm:           createVM,
			Ports:              ports,
			Sources:            createSources,
			Destinations:       createDestinations,
			Direction:          createDir,
			DisplayName:        createDisplay,
			Labels:             stringMapFromPairs(fwLabelPairs),
			Annotations:        stringMapFromPairs(fwAnnotationPairs),
			RequestId:          createRequestID,
			Action:             createAction,
			IpFamily:           createFamily,
			Priority:           createPriority,
			ForwardingPublicIp: createForwardIP,
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
	create.Flags().StringSliceVar(&createPorts, "port", nil, "PORT[-END][/PROTO], or PUBLIC:VM[/PROTO] with --forward-public-ip; e.g. 22/tcp, 30000-32767/tcp, 2222:22/tcp (repeatable)")
	create.Flags().StringVar(&createAction, "action", "", "allow (default) | deny")
	create.Flags().StringVar(&createFamily, "ip-family", "", "ipv4 | ipv6 | both (default: the rule's sources decide)")
	create.Flags().Int32Var(&createPriority, "priority", 0, "rule priority; lower numbers are evaluated first (0 = default)")
	create.Flags().StringVar(&createForwardIP, "forward-public-ip", "", "public IPv4 attached to the VM; ports become PUBLIC:VM forwarding")
	create.Flags().StringVar(&createRequestID, "request-id", "", "UUID that makes retries idempotent (generated when empty)")
	create.Flags().StringSliceVar(&fwLabelPairs, "label", nil, "labels as key=value (repeatable)")
	create.Flags().StringSliceVar(&fwAnnotationPairs, "annotation", nil, "annotations as key=value (repeatable)")
	cmd.AddCommand(create)

	var deleteRequestID string
	del := &cobra.Command{Use: "delete NAME", Short: "Delete a firewall rule", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.networkClient()
		if err != nil {
			return err
		}
		if deleteRequestID == "" {
			deleteRequestID = newRequestID()
			fmt.Fprintf(cmd.ErrOrStderr(), "Request ID %s (pass --request-id %s to retry this delete).\n", deleteRequestID, deleteRequestID)
		}
		resp, err := client.DeleteFirewallRule(cmd.Context(), connect.NewRequest(&networkv1.DeleteFirewallRuleRequest{Name: args[0], RequestId: deleteRequestID}))
		if err != nil {
			return err
		}
		return writeDeleted(cmd, ctx, "firewall", args[0], resp.Msg)
	}}
	del.Flags().StringVar(&deleteRequestID, "request-id", "", "UUID that makes retries idempotent (generated when empty)")
	cmd.AddCommand(del)
	attachNameCompleter(cmd, firewallNameCompleter(opts))
	return cmd
}

// parsePortMappings parses CLI --port flags. Each value is PORT[-END][/PROTO],
// or PUBLIC:VM[/PROTO] for port forwarding. Default proto is tcp. END is
// optional (single port when omitted).
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
		var target int
		if i := strings.IndexByte(s, ':'); i >= 0 {
			t, err := strconv.Atoi(strings.TrimSpace(s[i+1:]))
			if err != nil || t < 1 || t > 65535 {
				return nil, fmt.Errorf("invalid VM port %q: must be 1..65535", s[i+1:])
			}
			target, s = t, s[:i]
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
		pm := &networkv1.PortMapping{Port: int32(start), Protocol: proto, TargetPort: int32(target)}
		if target != 0 && endStr != "" {
			return nil, fmt.Errorf("port forwarding takes a single port, not a range: %q", s)
		}
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
