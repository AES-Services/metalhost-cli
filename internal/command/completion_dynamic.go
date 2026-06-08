package command

import (
	"context"
	"strings"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	computev1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/compute/v1"
	networkv1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/network/v1"
	storagev1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/storage/v1"
)

// completionFunc is cobra's dynamic-completion signature.
type completionFunc func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective)

// attachNameCompleter wires a resource-name completer onto every direct
// subcommand that takes a NAME positional (its Use contains "NAME"). This keeps
// the wiring out of the individual command bodies — call it once after the
// subcommands are registered.
func attachNameCompleter(parent *cobra.Command, completer completionFunc) {
	for _, sub := range parent.Commands() {
		if strings.Contains(sub.Use, "NAME") {
			sub.ValidArgsFunction = completer
		}
	}
}

// resourceNameCompleter adapts a list function into a cobra completer: it lists
// the resource, filters by the partial token the user has typed, and tells the
// shell not to fall back to filename completion. Any error (no project, network
// failure) yields no suggestions rather than a broken prompt.
func resourceNameCompleter(opts *rootOptions, list func(context.Context, *commandContext) ([]string, error)) completionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		cc, err := loadCommandContext(opts)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		names, err := list(ctx, cc)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		out := make([]string, 0, len(names))
		for _, n := range names {
			if toComplete == "" || strings.Contains(n, toComplete) {
				out = append(out, n)
			}
		}
		return out, cobra.ShellCompDirectiveNoFileComp
	}
}

func vmNameCompleter(opts *rootOptions) completionFunc {
	return resourceNameCompleter(opts, func(ctx context.Context, cc *commandContext) ([]string, error) {
		project, err := requireProject(cc, "")
		if err != nil {
			return nil, err
		}
		client, err := cc.computeClient()
		if err != nil {
			return nil, err
		}
		resp, err := client.ListVirtualMachines(ctx, connect.NewRequest(&computev1.ListVirtualMachinesRequest{ProjectName: project, PageSize: 200}))
		if err != nil {
			return nil, err
		}
		names := make([]string, 0, len(resp.Msg.GetVirtualMachines()))
		for _, vm := range resp.Msg.GetVirtualMachines() {
			names = append(names, vm.GetName())
		}
		return names, nil
	})
}

func sshKeyNameCompleter(opts *rootOptions) completionFunc {
	return resourceNameCompleter(opts, func(ctx context.Context, cc *commandContext) ([]string, error) {
		project, err := requireProject(cc, "")
		if err != nil {
			return nil, err
		}
		client, err := cc.sshKeyClient()
		if err != nil {
			return nil, err
		}
		resp, err := client.ListSSHKeys(ctx, connect.NewRequest(&computev1.ListSSHKeysRequest{ProjectName: project, PageSize: 200}))
		if err != nil {
			return nil, err
		}
		names := make([]string, 0, len(resp.Msg.GetSshKeys()))
		for _, k := range resp.Msg.GetSshKeys() {
			names = append(names, k.GetName())
		}
		return names, nil
	})
}

func diskNameCompleter(opts *rootOptions) completionFunc {
	return resourceNameCompleter(opts, func(ctx context.Context, cc *commandContext) ([]string, error) {
		project, err := requireProject(cc, "")
		if err != nil {
			return nil, err
		}
		client, err := cc.storageClient()
		if err != nil {
			return nil, err
		}
		resp, err := client.ListDisks(ctx, connect.NewRequest(&storagev1.ListDisksRequest{ProjectName: project, PageSize: 200}))
		if err != nil {
			return nil, err
		}
		names := make([]string, 0, len(resp.Msg.GetDisks()))
		for _, d := range resp.Msg.GetDisks() {
			names = append(names, d.GetName())
		}
		return names, nil
	})
}

func fileShareNameCompleter(opts *rootOptions) completionFunc {
	return resourceNameCompleter(opts, func(ctx context.Context, cc *commandContext) ([]string, error) {
		project, err := requireProject(cc, "")
		if err != nil {
			return nil, err
		}
		client, err := cc.storageClient()
		if err != nil {
			return nil, err
		}
		resp, err := client.ListFileShares(ctx, connect.NewRequest(&storagev1.ListFileSharesRequest{ProjectName: project, PageSize: 200}))
		if err != nil {
			return nil, err
		}
		names := make([]string, 0, len(resp.Msg.GetFileShares()))
		for _, f := range resp.Msg.GetFileShares() {
			names = append(names, f.GetName())
		}
		return names, nil
	})
}

func firewallNameCompleter(opts *rootOptions) completionFunc {
	return resourceNameCompleter(opts, func(ctx context.Context, cc *commandContext) ([]string, error) {
		client, err := cc.networkClient()
		if err != nil {
			return nil, err
		}
		resp, err := client.ListFirewallRules(ctx, connect.NewRequest(&networkv1.ListFirewallRulesRequest{PageSize: 200}))
		if err != nil {
			return nil, err
		}
		names := make([]string, 0, len(resp.Msg.GetFirewallRules()))
		for _, r := range resp.Msg.GetFirewallRules() {
			names = append(names, r.GetName())
		}
		return names, nil
	})
}
