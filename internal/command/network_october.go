package command

import (
	"strings"

	"github.com/spf13/cobra"
)

// The October networking surface (aes.network.v1.NetworkService): VPCs and
// subnets, reserved public IPs, load balancers, security groups and per-VM
// network controls. Mutations take a reviewed protobuf JSON request (--file)
// because they carry a request_id and expected generations: retry the exact
// same request rather than building a new one.

const networkService = "aes.network.v1.NetworkService"

type rpcSpec struct {
	use, method string
}

// addRPCs adds one command per spec. Methods named List*/Get* are reads; every
// other method is a mutation and requires --file.
func addRPCs(parent *cobra.Command, opts *rootOptions, service string, specs ...rpcSpec) {
	for _, s := range specs {
		write := !strings.HasPrefix(s.method, "List") && !strings.HasPrefix(s.method, "Get") &&
			!strings.HasPrefix(s.method, "Quote") && !strings.HasPrefix(s.method, "Check")
		parent.AddCommand(observabilityRPC(opts, s.use, service, s.method, write, false))
	}
}

func newVpcCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "vpc", Aliases: []string{"vpcs"}, Short: "Manage VPCs, their subnets and datacenter membership"}
	addRPCs(cmd, opts, networkService,
		rpcSpec{"list", "ListVpcs"},
		rpcSpec{"get", "GetVpc"},
		rpcSpec{"create", "CreateVpc"},
		rpcSpec{"update", "UpdateVpc"},
		rpcSpec{"delete", "DeleteVpc"},
		rpcSpec{"detach-datacenter", "DetachVpcDatacenter"},
	)
	subnets := &cobra.Command{Use: "subnet", Aliases: []string{"subnets"}, Short: "Manage VPC subnets"}
	addRPCs(subnets, opts, networkService,
		rpcSpec{"list", "ListVpcSubnets"},
		rpcSpec{"get", "GetVpcSubnet"},
		rpcSpec{"create", "CreateVpcSubnet"},
		rpcSpec{"update", "UpdateVpcSubnet"},
		rpcSpec{"delete", "DeleteVpcSubnet"},
		rpcSpec{"attach", "AttachVpcSubnet"},
	)
	cmd.AddCommand(subnets)
	return cmd
}

func newPublicIPCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "public-ip", Aliases: []string{"public-ips", "ip", "ips"}, Short: "Reserve, attach, detach and release public IPv4 addresses"}
	addRPCs(cmd, opts, networkService,
		rpcSpec{"list", "ListPublicIps"},
		rpcSpec{"get", "GetPublicIp"},
		rpcSpec{"reserve", "ReservePublicIp"},
		rpcSpec{"attach", "AttachPublicIp"},
		rpcSpec{"detach", "DetachPublicIp"},
		rpcSpec{"release", "ReleasePublicIp"},
	)
	return cmd
}

func newLoadBalancerCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "load-balancer", Aliases: []string{"load-balancers", "lb", "lbs"}, Short: "Manage L4 load balancers"}
	addRPCs(cmd, opts, networkService,
		rpcSpec{"list", "ListLoadBalancers"},
		rpcSpec{"get", "GetLoadBalancer"},
		rpcSpec{"create", "CreateLoadBalancer"},
		rpcSpec{"update", "UpdateLoadBalancer"},
		rpcSpec{"delete", "DeleteLoadBalancer"},
	)
	return cmd
}

func newSecurityGroupCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "security-group", Aliases: []string{"security-groups", "sg", "sgs"}, Short: "Manage security groups and their VM membership"}
	addRPCs(cmd, opts, networkService,
		rpcSpec{"list", "ListSecurityGroups"},
		rpcSpec{"get", "GetSecurityGroup"},
		rpcSpec{"create", "CreateSecurityGroup"},
		rpcSpec{"update", "UpdateSecurityGroup"},
		rpcSpec{"delete", "DeleteSecurityGroup"},
		rpcSpec{"add-vm", "AddVmToSecurityGroup"},
		rpcSpec{"remove-vm", "RemoveVmFromSecurityGroup"},
	)
	return cmd
}

// newVMNetworkCommand is `metalhost vm network`: a VM's interfaces, internet
// policy, IPv4 aliases and default public IP.
func newVMNetworkCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "network", Short: "Inspect and change a VM's network interfaces and internet access"}
	addRPCs(cmd, opts, networkService, rpcSpec{"interfaces", "GetVmNetworkInterfaces"})
	policy := &cobra.Command{Use: "internet-policy", Short: "Outbound IPv4 and IPv6 internet access for a VM"}
	addRPCs(policy, opts, networkService,
		rpcSpec{"get", "GetVmInternetPolicy"},
		rpcSpec{"set", "UpdateVmInternetPolicy"},
	)
	alias := &cobra.Command{Use: "alias", Short: "Additional private IPv4 addresses on a VM interface"}
	addRPCs(alias, opts, networkService,
		rpcSpec{"add", "AddVmIpv4Alias"},
		rpcSpec{"remove", "RemoveVmIpv4Alias"},
	)
	addRPCs(cmd, opts, networkService, rpcSpec{"set-default-public-ip", "SetVmDefaultPublicIp"})
	cmd.AddCommand(policy, alias)
	return cmd
}

// findCommand walks root by subcommand names, e.g. ("iam", "mfa").
func findCommand(root *cobra.Command, path ...string) *cobra.Command {
	cur := root
	for _, name := range path {
		var next *cobra.Command
		for _, c := range cur.Commands() {
			if c.Name() == name {
				next = c
				break
			}
		}
		if next == nil {
			return nil
		}
		cur = next
	}
	return cur
}

// addOctoberSubcommands adds the public RPCs that belong inside existing
// command groups. A missing group is a programming error caught by tests.
func addOctoberSubcommands(root *cobra.Command, opts *rootOptions) {
	attach := func(path []string, service string, specs ...rpcSpec) {
		parent := findCommand(root, path...)
		if parent == nil {
			panic("missing command group " + strings.Join(path, " "))
		}
		addRPCs(parent, opts, service, specs...)
	}
	if vm := findCommand(root, "vm"); vm != nil {
		vm.AddCommand(newVMNetworkCommand(opts))
	}
	attach([]string{"vm"}, "aes.compute.v1.ComputeService",
		rpcSpec{"quote-resize", "QuoteVirtualMachineResize"},
		rpcSpec{"utilization", "ListVMUtilization"},
	)
	attach([]string{"disk"}, "aes.storage.v1.StorageService", rpcSpec{"quote-resize", "QuoteDiskResize"})
	attach([]string{"file-share"}, "aes.storage.v1.StorageService", rpcSpec{"get", "GetFileShare"})
	attach([]string{"quota"}, "aes.quota.v1.QuotaService",
		rpcSpec{"request", "RequestQuotaIncrease"},
		rpcSpec{"requests", "ListMyQuotaRequests"},
	)
	members := &cobra.Command{Use: "members", Short: "Project-level member roles"}
	addRPCs(members, opts, "aes.project.v1.ProjectService",
		rpcSpec{"list", "ListProjectMembers"},
		rpcSpec{"set", "SetProjectMember"},
		rpcSpec{"remove", "RemoveProjectMember"},
	)
	findCommand(root, "project").AddCommand(members)
	attach([]string{"org"}, "aes.project.v1.ProjectService", rpcSpec{"check-slug", "CheckSlugAvailable"})
	attach([]string{"wallet"}, "aes.wallet.v1.WalletService",
		rpcSpec{"spend-by-project", "GetProjectSpendBreakdown"},
		rpcSpec{"redeem-promo", "RedeemPromotionCode"},
		rpcSpec{"promotion", "GetActivePromotion"},
	)
	attach([]string{"webhook"}, "aes.webhooks.v1.WebhooksService", rpcSpec{"rotate-secret", "RotateSubscriptionSecret"})
	attach([]string{"iam", "mfa"}, "aes.iam.v1.IamService", rpcSpec{"regenerate-recovery-codes", "RegenerateRecoveryCodes"})
	attach([]string{"automation", "github"}, "aes.iam.v1.AutomationService",
		rpcSpec{"setup-begin", "BeginGitHubSetup"},
		rpcSpec{"setup-get", "GetGitHubSetup"},
		rpcSpec{"setup-submit", "SubmitGitHubSetup"},
	)
	attach([]string{"monitoring", "guest"}, "aes.monitoring.v1.MonitoringService", rpcSpec{"install-collector", "InstallGuestCollector"})
}
