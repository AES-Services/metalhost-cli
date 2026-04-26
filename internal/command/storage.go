package command

import (
	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	storagev1 "github.com/AES-Services/foundry-sdk/gen/go/aes/storage/v1"
)

func newStorageCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "storage", Short: "Manage storage resources"}
	cmd.AddCommand(newDiskCommand(opts), newSnapshotCommand(opts), newFileShareCommand(opts))
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
	var createProject, region, class, displayName, fromSnapshot string
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
		resp, err := client.CreateDisk(cmd.Context(), connect.NewRequest(&storagev1.CreateDiskRequest{ProjectName: projectName, DatacenterName: region, SizeGib: size, StorageClass: class, DisplayName: displayName, FromSnapshot: fromSnapshot}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	create.Flags().StringVar(&createProject, "project", "", "project")
	create.Flags().StringVar(&region, "region", "", "datacenter/region")
	create.Flags().Int32Var(&size, "size-gib", 0, "size GiB")
	create.Flags().StringVar(&class, "class", "", "storage class")
	create.Flags().StringVar(&displayName, "display-name", "", "display name")
	create.Flags().StringVar(&fromSnapshot, "from-snapshot", "", "source snapshot")
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
	return cmd
}

func newSnapshotCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "snapshot", Aliases: []string{"snapshots"}, Short: "Manage disk snapshots"}
	var pages pageFlags
	var project string
	list := &cobra.Command{Use: "list", Short: "List snapshots", RunE: func(cmd *cobra.Command, _ []string) error {
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
		resp, err := client.ListSnapshots(cmd.Context(), connect.NewRequest(&storagev1.ListSnapshotsRequest{ProjectName: projectName, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	addPageFlags(list, &pages)
	list.Flags().StringVar(&project, "project", "", "project")
	cmd.AddCommand(list)
	cmd.AddCommand(&cobra.Command{Use: "get NAME", Short: "Get snapshot", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.storageClient()
		if err != nil {
			return err
		}
		resp, err := client.GetSnapshot(cmd.Context(), connect.NewRequest(&storagev1.GetSnapshotRequest{Name: args[0]}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}})
	var sourceDisk string
	create := &cobra.Command{Use: "create", Short: "Create snapshot", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.storageClient()
		if err != nil {
			return err
		}
		resp, err := client.CreateSnapshot(cmd.Context(), connect.NewRequest(&storagev1.CreateSnapshotRequest{SourceDisk: sourceDisk}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	create.Flags().StringVar(&sourceDisk, "disk", "", "source disk")
	cmd.AddCommand(create)
	return cmd
}

func newFileShareCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "file-share", Aliases: []string{"file-shares"}, Short: "Manage file shares"}
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
	cmd.AddCommand(&cobra.Command{Use: "get NAME", Short: "Get file share", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.storageClient()
		if err != nil {
			return err
		}
		resp, err := client.GetFileShare(cmd.Context(), connect.NewRequest(&storagev1.GetFileShareRequest{Name: args[0]}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}})
	return cmd
}
