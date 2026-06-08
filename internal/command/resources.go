package command

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"

	baremetalv1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/baremetal/v1"
	computev1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/compute/v1"
	networkv1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/network/v1"
	projectv1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/project/v1"
	storagev1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/storage/v1"
	webhooksv1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/webhooks/v1"
)

// resourceHandler describes how one resource kind maps onto its connect RPCs so
// the generic verbs (get/describe/delete) can operate over any kind uniformly.
//
// This registry is the target shape for the codegen step: every entry here is
// mechanically derivable from the proto service descriptors (List/Get/Delete
// methods + the resource message), so a generator can eventually emit it.
type resourceHandler struct {
	kind    string   // canonical short name, e.g. "vm"
	aliases []string // accepted alternatives, e.g. "vms", "virtual-machine"
	title   string   // human label for messages

	// parent resolves the list scope (project/org name). nil means the list is
	// unscoped (no parent argument).
	parent func(*commandContext) (string, error)

	list   func(cmd *cobra.Command, c *commandContext, parent string, pages pageFlags) (proto.Message, error)
	get    func(cmd *cobra.Command, c *commandContext, name string) (proto.Message, error)
	remove func(cmd *cobra.Command, c *commandContext, name string) (proto.Message, error)

	// names is an optional completer builder for resource names (verb arg 2).
	names func(*rootOptions) completionFunc
}

func (h *resourceHandler) matches(kind string) bool {
	if strings.EqualFold(h.kind, kind) {
		return true
	}
	for _, a := range h.aliases {
		if strings.EqualFold(a, kind) {
			return true
		}
	}
	return false
}

// ── scope resolvers ─────────────────────────────────────────────────────────

func projectParent(c *commandContext) (string, error) { return requireProject(c, "") }

func orgParent(c *commandContext) (string, error) {
	if strings.TrimSpace(c.profile.Organization) == "" {
		return "", errors.New("organization is required; pass --org or set a profile default")
	}
	return c.profile.Organization, nil
}

// ── registry ────────────────────────────────────────────────────────────────

// resourceRegistry is the single source of truth for which kinds the unified
// verbs understand. Order controls `metalhost get` (no-kind) help listing.
func resourceRegistry() []*resourceHandler {
	return []*resourceHandler{
		{
			kind: "vm", aliases: []string{"vms", "virtual-machine", "virtual-machines", "compute"}, title: "VM",
			parent: projectParent,
			list: func(cmd *cobra.Command, c *commandContext, parent string, pages pageFlags) (proto.Message, error) {
				cl, err := c.computeClient()
				if err != nil {
					return nil, err
				}
				return invokeList(cmd, cl.ListVirtualMachines, &computev1.ListVirtualMachinesRequest{ProjectName: parent, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}, pages.all)
			},
			get: func(cmd *cobra.Command, c *commandContext, name string) (proto.Message, error) {
				cl, err := c.computeClient()
				if err != nil {
					return nil, err
				}
				return invoke(cmd, cl.GetVirtualMachine, &computev1.GetVirtualMachineRequest{Name: name})
			},
			remove: func(cmd *cobra.Command, c *commandContext, name string) (proto.Message, error) {
				cl, err := c.computeClient()
				if err != nil {
					return nil, err
				}
				return invoke(cmd, cl.DeleteVirtualMachine, &computev1.DeleteVirtualMachineRequest{Name: name})
			},
			names: vmNameCompleter,
		},
		{
			kind: "ssh-key", aliases: []string{"ssh-keys", "sshkey", "key", "keys"}, title: "SSH key",
			parent: projectParent,
			list: func(cmd *cobra.Command, c *commandContext, parent string, pages pageFlags) (proto.Message, error) {
				cl, err := c.sshKeyClient()
				if err != nil {
					return nil, err
				}
				return invokeList(cmd, cl.ListSSHKeys, &computev1.ListSSHKeysRequest{ProjectName: parent, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}, pages.all)
			},
			remove: func(cmd *cobra.Command, c *commandContext, name string) (proto.Message, error) {
				cl, err := c.sshKeyClient()
				if err != nil {
					return nil, err
				}
				return invoke(cmd, cl.DeleteSSHKey, &computev1.DeleteSSHKeyRequest{Name: name})
			},
			names: sshKeyNameCompleter,
		},
		{
			kind: "disk", aliases: []string{"disks"}, title: "Disk",
			parent: projectParent,
			list: func(cmd *cobra.Command, c *commandContext, parent string, pages pageFlags) (proto.Message, error) {
				cl, err := c.storageClient()
				if err != nil {
					return nil, err
				}
				return invokeList(cmd, cl.ListDisks, &storagev1.ListDisksRequest{ProjectName: parent, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}, pages.all)
			},
			get: func(cmd *cobra.Command, c *commandContext, name string) (proto.Message, error) {
				cl, err := c.storageClient()
				if err != nil {
					return nil, err
				}
				return invoke(cmd, cl.GetDisk, &storagev1.GetDiskRequest{Name: name})
			},
			remove: func(cmd *cobra.Command, c *commandContext, name string) (proto.Message, error) {
				cl, err := c.storageClient()
				if err != nil {
					return nil, err
				}
				return invoke(cmd, cl.DeleteDisk, &storagev1.DeleteDiskRequest{Name: name})
			},
			names: diskNameCompleter,
		},
		{
			kind: "file-share", aliases: []string{"file-shares", "fileshare", "share", "shares"}, title: "File share",
			parent: projectParent,
			list: func(cmd *cobra.Command, c *commandContext, parent string, pages pageFlags) (proto.Message, error) {
				cl, err := c.storageClient()
				if err != nil {
					return nil, err
				}
				return invokeList(cmd, cl.ListFileShares, &storagev1.ListFileSharesRequest{ProjectName: parent, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}, pages.all)
			},
			remove: func(cmd *cobra.Command, c *commandContext, name string) (proto.Message, error) {
				cl, err := c.storageClient()
				if err != nil {
					return nil, err
				}
				return invoke(cmd, cl.DeleteFileShare, &storagev1.DeleteFileShareRequest{Name: name})
			},
			names: fileShareNameCompleter,
		},
		{
			kind: "network", aliases: []string{"networks", "net", "nets"}, title: "Network",
			parent: projectParent,
			list: func(cmd *cobra.Command, c *commandContext, parent string, pages pageFlags) (proto.Message, error) {
				cl, err := c.networkClient()
				if err != nil {
					return nil, err
				}
				return invokeList(cmd, cl.ListNetworks, &networkv1.ListNetworksRequest{ProjectName: parent, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}, pages.all)
			},
		},
		{
			kind: "baremetal", aliases: []string{"bare-metal", "bm"}, title: "Bare-metal instance",
			parent: projectParent,
			list: func(cmd *cobra.Command, c *commandContext, parent string, pages pageFlags) (proto.Message, error) {
				cl, err := c.bareMetalClient()
				if err != nil {
					return nil, err
				}
				return invokeList(cmd, cl.ListBareMetalInstances, &baremetalv1.ListBareMetalInstancesRequest{ProjectName: parent, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}, pages.all)
			},
			get: func(cmd *cobra.Command, c *commandContext, name string) (proto.Message, error) {
				cl, err := c.bareMetalClient()
				if err != nil {
					return nil, err
				}
				return invoke(cmd, cl.GetBareMetalInstance, &baremetalv1.GetBareMetalInstanceRequest{Name: name})
			},
		},
		{
			kind: "webhook", aliases: []string{"webhooks", "subscription", "subscriptions"}, title: "Webhook subscription",
			parent: projectParent,
			list: func(cmd *cobra.Command, c *commandContext, parent string, pages pageFlags) (proto.Message, error) {
				cl, err := c.webhooksClient()
				if err != nil {
					return nil, err
				}
				return invokeList(cmd, cl.ListSubscriptions, &webhooksv1.ListSubscriptionsRequest{ProjectName: parent, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}, pages.all)
			},
			get: func(cmd *cobra.Command, c *commandContext, name string) (proto.Message, error) {
				cl, err := c.webhooksClient()
				if err != nil {
					return nil, err
				}
				return invoke(cmd, cl.GetSubscription, &webhooksv1.GetSubscriptionRequest{Name: name})
			},
			remove: func(cmd *cobra.Command, c *commandContext, name string) (proto.Message, error) {
				cl, err := c.webhooksClient()
				if err != nil {
					return nil, err
				}
				return invoke(cmd, cl.DeleteSubscription, &webhooksv1.DeleteSubscriptionRequest{Name: name})
			},
		},
		{
			kind: "project", aliases: []string{"projects", "proj"}, title: "Project",
			parent: orgParent,
			list: func(cmd *cobra.Command, c *commandContext, parent string, pages pageFlags) (proto.Message, error) {
				cl, err := c.projectClient()
				if err != nil {
					return nil, err
				}
				return invokeList(cmd, cl.ListProjects, &projectv1.ListProjectsRequest{Parent: parent, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}, pages.all)
			},
			get: func(cmd *cobra.Command, c *commandContext, name string) (proto.Message, error) {
				cl, err := c.projectClient()
				if err != nil {
					return nil, err
				}
				return invoke(cmd, cl.GetProject, &projectv1.GetProjectRequest{Name: name})
			},
			remove: func(cmd *cobra.Command, c *commandContext, name string) (proto.Message, error) {
				cl, err := c.projectClient()
				if err != nil {
					return nil, err
				}
				return invoke(cmd, cl.DeleteProject, &projectv1.DeleteProjectRequest{Name: name})
			},
		},
	}
}

// lookupResource resolves a kind/alias to its handler (case-insensitive).
func lookupResource(kind string) (*resourceHandler, bool) {
	for _, h := range resourceRegistry() {
		if h.matches(kind) {
			return h, true
		}
	}
	return nil, false
}

// resourceKinds returns the canonical kind names for help/completion.
func resourceKinds() []string {
	reg := resourceRegistry()
	out := make([]string, 0, len(reg))
	for _, h := range reg {
		out = append(out, h.kind)
	}
	return out
}