package command

import (
	"time"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	objectv1 "github.com/AES-Services/foundry-sdk/gen/go/aes/objectstore/v1"
)

func newObjectStoreCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "bucket", Aliases: []string{"buckets", "object"}, Short: "Manage object storage"}
	var pages pageFlags
	var project string
	list := &cobra.Command{Use: "list", Short: "List buckets", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		projectName, err := requireProject(ctx, project)
		if err != nil {
			return err
		}
		client, err := ctx.objectStoreClient()
		if err != nil {
			return err
		}
		resp, err := client.ListBuckets(cmd.Context(), connect.NewRequest(&objectv1.ListBucketsRequest{ProjectName: projectName, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	addPageFlags(list, &pages)
	list.Flags().StringVar(&project, "project", "", "project")
	cmd.AddCommand(list)
	cmd.AddCommand(&cobra.Command{Use: "get NAME", Short: "Get bucket", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.objectStoreClient()
		if err != nil {
			return err
		}
		resp, err := client.GetBucket(cmd.Context(), connect.NewRequest(&objectv1.GetBucketRequest{Name: args[0]}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}})
	var createProject string
	create := &cobra.Command{Use: "create NAME", Short: "Create bucket", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		projectName, err := requireProject(ctx, createProject)
		if err != nil {
			return err
		}
		client, err := ctx.objectStoreClient()
		if err != nil {
			return err
		}
		resp, err := client.CreateBucket(cmd.Context(), connect.NewRequest(&objectv1.CreateBucketRequest{Name: args[0], ProjectName: projectName}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	create.Flags().StringVar(&createProject, "project", "", "project")
	cmd.AddCommand(create)
	cmd.AddCommand(&cobra.Command{Use: "delete NAME", Short: "Delete bucket", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.objectStoreClient()
		if err != nil {
			return err
		}
		resp, err := client.DeleteBucket(cmd.Context(), connect.NewRequest(&objectv1.DeleteBucketRequest{Name: args[0]}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}})
	cmd.AddCommand(newObjectCommand(opts))
	return cmd
}

func newObjectCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "object", Short: "Manage objects with presigned URLs"}
	var prefix, token string
	var pageSize int32
	list := &cobra.Command{Use: "list BUCKET", Short: "List objects", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.objectStoreClient()
		if err != nil {
			return err
		}
		resp, err := client.ListObjects(cmd.Context(), connect.NewRequest(&objectv1.ListObjectsRequest{BucketName: args[0], Prefix: prefix, PageSize: pageSize, PageToken: token}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	list.Flags().StringVar(&prefix, "prefix", "", "object prefix")
	list.Flags().Int32Var(&pageSize, "page-size", 50, "page size")
	list.Flags().StringVar(&token, "page-token", "", "page token")
	cmd.AddCommand(list)
	addPresign := func(use, short string, upload bool) {
		var object string
		var ttl time.Duration
		c := &cobra.Command{Use: use + " BUCKET", Short: short, Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.objectStoreClient()
			if err != nil {
				return err
			}
			if upload {
				resp, err := client.CreatePresignedUploadURL(cmd.Context(), connect.NewRequest(&objectv1.CreatePresignedUploadURLRequest{BucketName: args[0], ObjectKey: object, ExpiresSeconds: int32(ttl.Seconds())}))
				if err != nil {
					return err
				}
				return ctx.write(resp.Msg)
			}
			resp, err := client.CreatePresignedDownloadURL(cmd.Context(), connect.NewRequest(&objectv1.CreatePresignedDownloadURLRequest{BucketName: args[0], ObjectKey: object, ExpiresSeconds: int32(ttl.Seconds())}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		}}
		c.Flags().StringVar(&object, "object", "", "object key")
		c.Flags().DurationVar(&ttl, "ttl", time.Hour, "URL TTL")
		cmd.AddCommand(c)
	}
	addPresign("presign-upload", "Create presigned upload URL", true)
	addPresign("presign-download", "Create presigned download URL", false)
	return cmd
}
