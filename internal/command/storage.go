package command

import (
	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	storagev1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/storage/v1"
)

func newStorageCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "storage", Short: "Manage storage resources"}
	cmd.AddCommand(newDiskCommand(opts), newFileShareCommand(opts))
	return cmd
}

func newDiskCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "disk", Aliases: []string{"disks"}, Short: "Manage disks"}
	var pages pageFlags
	var project string
	list := &cobra.Command{Use: "list", Short: "List disks", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		projectName, err := requireProject(ctx, project)
		if err != nil {
			return err
		}
		client, err := ctx.storageClient()
		if err != nil {
			return err
		}
		resp, err := client.ListDisks(cmd.Context(), connect.NewRequest(&storagev1.ListDisksRequest{ProjectName: projectName, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	addPageFlags(list, &pages)
	list.Flags().StringVar(&project, "project", "", "project")
	cmd.AddCommand(list)

	cmd.AddCommand(&cobra.Command{Use: "get NAME", Short: "Get disk", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.storageClient()
		if err != nil {
			return err
		}
		resp, err := client.GetDisk(cmd.Context(), connect.NewRequest(&storagev1.GetDiskRequest{Name: args[0]}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}})

	var createProject, region, class, displayName string
	var size int32
	create := &cobra.Command{Use: "create", Short: "Create disk", RunE: func(cmd *cobra.Command, _ []string) error {
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
		client, err := ctx.storageClient()
		if err != nil {
			return err
		}
		resp, err := client.CreateDisk(cmd.Context(), connect.NewRequest(&storagev1.CreateDiskRequest{ProjectName: projectName, DatacenterName: region, SizeGib: size, StorageClass: class, DisplayName: displayName}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	create.Flags().StringVar(&createProject, "project", "", "project")
	create.Flags().StringVar(&region, "region", "", "datacenter/region")
	create.Flags().Int32Var(&size, "size-gib", 0, "size GiB (1-1024)")
	create.Flags().StringVar(&class, "class", "nvme", "storage class (currently only 'nvme')")
	create.Flags().StringVar(&displayName, "display-name", "", "display name")
	cmd.AddCommand(create)

	cmd.AddCommand(&cobra.Command{Use: "delete NAME", Short: "Delete disk", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.storageClient()
		if err != nil {
			return err
		}
		resp, err := client.DeleteDisk(cmd.Context(), connect.NewRequest(&storagev1.DeleteDiskRequest{Name: args[0]}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}})

	var resizeSize int32
	resize := &cobra.Command{Use: "resize NAME", Short: "Resize a disk", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.storageClient()
		if err != nil {
			return err
		}
		resp, err := client.ResizeDisk(cmd.Context(), connect.NewRequest(&storagev1.ResizeDiskRequest{DiskName: args[0], NewSizeGib: resizeSize}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	resize.Flags().Int32Var(&resizeSize, "size-gib", 0, "new size in GiB (must be > current size)")
	cmd.AddCommand(resize)

	var attachVM string
	attach := &cobra.Command{Use: "attach NAME", Short: "Attach a disk to a VM", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.storageClient()
		if err != nil {
			return err
		}
		resp, err := client.AttachDisk(cmd.Context(), connect.NewRequest(&storagev1.AttachDiskRequest{DiskName: args[0], VmName: attachVM}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	attach.Flags().StringVar(&attachVM, "vm", "", "target VM resource name (required)")
	cmd.AddCommand(attach)

	cmd.AddCommand(&cobra.Command{Use: "detach NAME", Short: "Detach a disk from its VM", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.storageClient()
		if err != nil {
			return err
		}
		resp, err := client.DetachDisk(cmd.Context(), connect.NewRequest(&storagev1.DetachDiskRequest{DiskName: args[0]}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}})

	var updateDisplay string
	var clearLabels, clearAnnotations bool
	update := &cobra.Command{Use: "update NAME", Short: "Update a disk's display name / labels / annotations", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.storageClient()
		if err != nil {
			return err
		}
		req := &storagev1.UpdateDiskRequest{Name: args[0], ClearLabels: clearLabels, ClearAnnotations: clearAnnotations}
		if updateDisplay != "" {
			d := updateDisplay
			req.DisplayName = &d
		}
		resp, err := client.UpdateDisk(cmd.Context(), connect.NewRequest(req))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	update.Flags().StringVar(&updateDisplay, "display-name", "", "new display name")
	update.Flags().BoolVar(&clearLabels, "clear-labels", false, "clear all labels")
	update.Flags().BoolVar(&clearAnnotations, "clear-annotations", false, "clear all annotations")
	cmd.AddCommand(update)

	return cmd
}

func newFileShareCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "file-share", Aliases: []string{"file-shares"}, Short: "Manage file shares (CephFS + NFSv4)"}
	var pages pageFlags
	var project string
	list := &cobra.Command{Use: "list", Short: "List file shares", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		projectName, err := requireProject(ctx, project)
		if err != nil {
			return err
		}
		client, err := ctx.storageClient()
		if err != nil {
			return err
		}
		resp, err := client.ListFileShares(cmd.Context(), connect.NewRequest(&storagev1.ListFileSharesRequest{ProjectName: projectName, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	addPageFlags(list, &pages)
	list.Flags().StringVar(&project, "project", "", "project")
	cmd.AddCommand(list)

	var createProject, createDC, createClass, createDisplay string
	var createSize int32
	create := &cobra.Command{Use: "create", Short: "Create a CephFS-backed file share", RunE: func(cmd *cobra.Command, _ []string) error {
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
		client, err := ctx.storageClient()
		if err != nil {
			return err
		}
		resp, err := client.CreateFileShare(cmd.Context(), connect.NewRequest(&storagev1.CreateFileShareRequest{
			ProjectName:    projectName,
			DatacenterName: createDC,
			SizeGib:        createSize,
			StorageClass:   createClass,
			DisplayName:    createDisplay,
		}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	create.Flags().StringVar(&createProject, "project", "", "project (defaults to active project)")
	create.Flags().StringVar(&createDC, "datacenter", "", "datacenter (defaults to profile region)")
	create.Flags().Int32Var(&createSize, "size-gib", 0, "size GiB (required)")
	create.Flags().StringVar(&createClass, "class", "cephfs-rwx", "storage tier alias")
	create.Flags().StringVar(&createDisplay, "display-name", "", "display name")
	cmd.AddCommand(create)

	cmd.AddCommand(&cobra.Command{Use: "delete NAME", Short: "Delete a file share", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.storageClient()
		if err != nil {
			return err
		}
		resp, err := client.DeleteFileShare(cmd.Context(), connect.NewRequest(&storagev1.DeleteFileShareRequest{Name: args[0]}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}})

	return cmd
}
