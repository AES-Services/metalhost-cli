package command

import (
	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	computev1 "github.com/AES-Services/foundry-sdk/gen/go/aes/compute/v1"
)

func newComputeCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "vm", Aliases: []string{"vms", "compute"}, Short: "Manage virtual machines"}
	cmd.AddCommand(newVMCommands(opts)...)
	cmd.AddCommand(newImageCommand(opts))
	return cmd
}

func newVMCommands(opts *rootOptions) []*cobra.Command {
	var pages pageFlags
	var project string
	list := &cobra.Command{
		Use: "list", Short: "List VMs",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			projectName, err := requireProject(ctx, project)
			if err != nil {
				return err
			}
			client, err := ctx.computeClient()
			if err != nil {
				return err
			}
			resp, err := client.ListVirtualMachines(cmd.Context(), connect.NewRequest(&computev1.ListVirtualMachinesRequest{ProjectName: projectName, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}
	addPageFlags(list, &pages)
	list.Flags().StringVar(&project, "project", "", "project")

	get := &cobra.Command{Use: "get NAME", Short: "Get VM", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.computeClient()
		if err != nil {
			return err
		}
		resp, err := client.GetVirtualMachine(cmd.Context(), connect.NewRequest(&computev1.GetVirtualMachineRequest{Name: args[0]}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}

	var createProject, region, sku, image, bootURL, sshKey, userData string
	var diskSize int32
	var networks []string
	create := &cobra.Command{Use: "create", Short: "Create VM", RunE: func(cmd *cobra.Command, _ []string) error {
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
		client, err := ctx.computeClient()
		if err != nil {
			return err
		}
		resp, err := client.CreateVirtualMachine(cmd.Context(), connect.NewRequest(&computev1.CreateVirtualMachineRequest{ProjectName: projectName, DatacenterName: region, InstanceType: sku, ImageName: image, BootImageUrl: bootURL, DiskSizeGib: diskSize, NetworkNames: networks, SshPubkey: sshKey, UserData: userData}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	create.Flags().StringVar(&createProject, "project", "", "project")
	create.Flags().StringVar(&region, "region", "", "datacenter/region")
	create.Flags().StringVar(&sku, "sku", "", "instance SKU")
	create.Flags().StringVar(&image, "image", "", "machine image name")
	create.Flags().StringVar(&bootURL, "boot-url", "", "boot image URL")
	create.Flags().Int32Var(&diskSize, "disk-size-gib", 0, "boot disk size GiB")
	create.Flags().StringSliceVar(&networks, "network", nil, "network to attach")
	create.Flags().StringVar(&sshKey, "ssh-key", "", "SSH public key")
	create.Flags().StringVar(&userData, "user-data", "", "cloud-init user data")

	return []*cobra.Command{
		list, get, create,
		vmDeleteCommand(opts),
		vmStartCommand(opts),
		vmStopCommand(opts),
		vmRestartCommand(opts),
	}
}

func vmDeleteCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "delete NAME", Short: "Delete VM", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.computeClient()
		if err != nil {
			return err
		}
		resp, err := client.DeleteVirtualMachine(cmd.Context(), connect.NewRequest(&computev1.DeleteVirtualMachineRequest{Name: args[0]}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
}

func vmStartCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "start NAME", Short: "Start VM", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.computeClient()
		if err != nil {
			return err
		}
		resp, err := client.StartVirtualMachine(cmd.Context(), connect.NewRequest(&computev1.StartVirtualMachineRequest{Name: args[0]}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
}

func vmStopCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "stop NAME", Short: "Stop VM", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.computeClient()
		if err != nil {
			return err
		}
		resp, err := client.StopVirtualMachine(cmd.Context(), connect.NewRequest(&computev1.StopVirtualMachineRequest{Name: args[0]}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
}

func vmRestartCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "restart NAME", Short: "Restart VM", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.computeClient()
		if err != nil {
			return err
		}
		resp, err := client.RestartVirtualMachine(cmd.Context(), connect.NewRequest(&computev1.RestartVirtualMachineRequest{Name: args[0]}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
}

func newImageCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "image", Aliases: []string{"images"}, Short: "Manage machine images"}
	var pages pageFlags
	var project string
	list := &cobra.Command{Use: "list", Short: "List images", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		projectName, err := requireProject(ctx, project)
		if err != nil {
			return err
		}
		client, err := ctx.computeClient()
		if err != nil {
			return err
		}
		resp, err := client.ListMachineImages(cmd.Context(), connect.NewRequest(&computev1.ListMachineImagesRequest{ProjectId: projectName, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	addPageFlags(list, &pages)
	list.Flags().StringVar(&project, "project", "", "project")
	cmd.AddCommand(list)
	cmd.AddCommand(&cobra.Command{Use: "get NAME", Short: "Get image", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.computeClient()
		if err != nil {
			return err
		}
		resp, err := client.GetMachineImage(cmd.Context(), connect.NewRequest(&computev1.GetMachineImageRequest{Name: args[0]}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}})
	var createProject, imageID, displayName, region, sourceURL string
	create := &cobra.Command{Use: "create", Short: "Create image", RunE: func(cmd *cobra.Command, _ []string) error {
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
		client, err := ctx.computeClient()
		if err != nil {
			return err
		}
		resp, err := client.CreateMachineImage(cmd.Context(), connect.NewRequest(&computev1.CreateMachineImageRequest{ProjectId: projectName, ImageId: imageID, DisplayName: displayName, DatacenterName: region, SourceUrl: sourceURL}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	create.Flags().StringVar(&createProject, "project", "", "project")
	create.Flags().StringVar(&imageID, "id", "", "image id")
	create.Flags().StringVar(&displayName, "display-name", "", "display name")
	create.Flags().StringVar(&region, "region", "", "datacenter/region")
	create.Flags().StringVar(&sourceURL, "source-url", "", "source URL")
	cmd.AddCommand(create)
	cmd.AddCommand(&cobra.Command{Use: "delete NAME", Short: "Delete image", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.computeClient()
		if err != nil {
			return err
		}
		resp, err := client.DeleteMachineImage(cmd.Context(), connect.NewRequest(&computev1.DeleteMachineImageRequest{Name: args[0]}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}})
	return cmd
}
