package command

import (
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	storagev1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/storage/v1"
)

func backupRequest[T any](msg *T, key string) *connect.Request[T] {
	req := connect.NewRequest(msg)
	if key != "" {
		req.Header().Set("Idempotency-Key", key)
	}
	return req
}

func newDiskBackupCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "backup", Aliases: []string{"backups", "snapshot", "snapshots"}, Short: "Manage single-disk backups"}
	var display, key string
	create := &cobra.Command{Use: "create DISK", Short: "Capture a disk without replacing it", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cc, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		cl, err := cc.storageClient()
		if err != nil {
			return err
		}
		resp, err := cl.CreateSnapshot(cmd.Context(), backupRequest(&storagev1.CreateSnapshotRequest{SourceDisk: args[0], DisplayName: display}, key))
		if err != nil {
			return err
		}
		return cc.write(resp.Msg)
	}}
	create.Flags().StringVar(&display, "display-name", "", "backup display name")
	create.Flags().StringVar(&key, "idempotency-key", "", "reuse this key for retries of the same capture")
	cmd.AddCommand(create)

	var source string
	var pages pageFlags
	list := &cobra.Command{Use: "list", Short: "List single-disk backups in the selected project", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		cc, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		parent, err := requireProject(cc, "")
		if err != nil {
			return err
		}
		cl, err := cc.storageClient()
		if err != nil {
			return err
		}
		return doList(cmd, cc, cl.ListSnapshots, &storagev1.ListSnapshotsRequest{ProjectName: parent, SourceDisk: source, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}, pages.all)
	}}
	list.Flags().StringVar(&source, "disk", "", "filter by source disk")
	addPageFlags(list, &pages)
	cmd.AddCommand(list)
	cmd.AddCommand(&cobra.Command{Use: "get NAME", Short: "Get a single-disk backup and its readiness", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cc, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		cl, err := cc.storageClient()
		if err != nil {
			return err
		}
		return do(cmd, cc, cl.GetSnapshot, &storagev1.GetSnapshotRequest{Name: args[0]})
	}})
	var yes bool
	del := &cobra.Command{Use: "delete NAME", Short: "Permanently delete a single-disk backup", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cc, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		if err := confirmDestructive(cmd, yes, "Delete single-disk backup", args[0]); err != nil {
			return err
		}
		cl, err := cc.storageClient()
		if err != nil {
			return err
		}
		resp, err := cl.DeleteSnapshot(cmd.Context(), connect.NewRequest(&storagev1.DeleteSnapshotRequest{Name: args[0]}))
		if err != nil {
			return err
		}
		return writeDeleted(cmd, cc, "disk-backup", args[0], resp.Msg)
	}}
	del.Flags().BoolVar(&yes, "yes", false, "skip confirmation")
	cmd.AddCommand(del)
	return cmd
}

func newBackupScheduleCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "backup-schedule", Aliases: []string{"backup-schedules"}, Short: "Manage automatic backups and keep-N retention (fixed UTC times)"}
	cmd.AddCommand(backupScheduleWriteCommand(opts, false), backupScheduleWriteCommand(opts, true))
	var target string
	var pages pageFlags
	list := &cobra.Command{Use: "list", Short: "List schedules in the selected project", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		cc, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		parent, err := requireProject(cc, "")
		if err != nil {
			return err
		}
		cl, err := cc.storageClient()
		if err != nil {
			return err
		}
		return doList(cmd, cc, cl.ListSnapshotSchedules, &storagev1.ListSnapshotSchedulesRequest{ProjectName: parent, Target: target, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}, pages.all)
	}}
	list.Flags().StringVar(&target, "target", "", "filter by project, VM, or disk resource name")
	addPageFlags(list, &pages)
	cmd.AddCommand(list)
	var yes bool
	del := &cobra.Command{Use: "delete NAME", Short: "Remove a schedule while keeping existing backups", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cc, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		if err := confirmDestructive(cmd, yes, "Delete backup schedule (existing backups are kept)", args[0]); err != nil {
			return err
		}
		cl, err := cc.storageClient()
		if err != nil {
			return err
		}
		resp, err := cl.DeleteSnapshotSchedule(cmd.Context(), connect.NewRequest(&storagev1.DeleteSnapshotScheduleRequest{Name: args[0]}))
		if err != nil {
			return err
		}
		return writeDeleted(cmd, cc, "backup-schedule", args[0], resp.Msg)
	}}
	del.Flags().BoolVar(&yes, "yes", false, "skip confirmation")
	cmd.AddCommand(del)
	return cmd
}

func backupScheduleWriteCommand(opts *rootOptions, update bool) *cobra.Command {
	schedule := &storagev1.SnapshotSchedule{Cadence: "DAILY", RetentionCount: 7, Enabled: true}
	var key string
	use, short := "create", "Create an automatic backup schedule"
	args := cobra.NoArgs
	if update {
		use, short, args = "update NAME", "Update timing, retention, or enabled status", cobra.ExactArgs(1)
	}
	cmd := &cobra.Command{Use: use, Short: short, Args: args, RunE: func(cmd *cobra.Command, args []string) error {
		cc, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		cl, err := cc.storageClient()
		if err != nil {
			return err
		}
		schedule.Cadence = strings.ToUpper(schedule.Cadence)
		if !update {
			parent, err := requireProject(cc, "")
			if err != nil {
				return err
			}
			schedule.ProjectName = parent
			if schedule.Target == "" {
				schedule.Target = parent
			}
		}
		if !update || cmd.Flags().Changed("cadence") {
			switch schedule.Cadence {
			case "HOURLY", "DAILY", "WEEKLY":
			default:
				return fmt.Errorf("cadence must be hourly, daily, or weekly")
			}
		}
		if schedule.HourUtc < 0 || schedule.HourUtc > 23 || schedule.MinuteUtc < 0 || schedule.MinuteUtc > 59 || schedule.WeekdayUtc < 0 || schedule.WeekdayUtc > 6 {
			return fmt.Errorf("UTC time must use hour 0–23, minute 0–59, and weekday 0–6")
		}
		if schedule.RetentionCount < 1 || schedule.RetentionCount > 100 {
			return fmt.Errorf("keep must be between 1 and 100")
		}
		if update {
			schedule.Name = args[0]
			var paths []string
			for _, flag := range []string{"cadence", "hour-utc", "minute-utc", "weekday-utc", "keep", "enabled", "display-name"} {
				if cmd.Flags().Changed(flag) {
					path := strings.ReplaceAll(flag, "-", "_")
					if flag == "keep" {
						path = "retention_count"
					}
					paths = append(paths, path)
				}
			}
			if len(paths) == 0 {
				return fmt.Errorf("specify at least one field to update")
			}
			return do(cmd, cc, cl.UpdateSnapshotSchedule, &storagev1.UpdateSnapshotScheduleRequest{Schedule: schedule, UpdateMask: &fieldmaskpb.FieldMask{Paths: paths}})
		}
		resp, err := cl.CreateSnapshotSchedule(cmd.Context(), backupRequest(&storagev1.CreateSnapshotScheduleRequest{Schedule: schedule}, key))
		if err != nil {
			return err
		}
		return cc.write(resp.Msg)
	}}
	if !update {
		cmd.Flags().StringVar(&schedule.Target, "target", "", "project, VM, or disk resource name (defaults to the selected project)")
		cmd.Flags().StringVar(&key, "idempotency-key", "", "reuse this key for retries of the same schedule creation")
	}
	cmd.Flags().StringVar(&schedule.Cadence, "cadence", "daily", "hourly, daily, or weekly")
	cmd.Flags().Int32Var(&schedule.HourUtc, "hour-utc", 0, "UTC hour (0–23), for daily and weekly")
	cmd.Flags().Int32Var(&schedule.MinuteUtc, "minute-utc", 0, "UTC minute (0–59)")
	cmd.Flags().Int32Var(&schedule.WeekdayUtc, "weekday-utc", 0, "UTC weekday for weekly (Sunday=0, Saturday=6)")
	cmd.Flags().Int32Var(&schedule.RetentionCount, "keep", 7, "successful copies to retain per source (1–100); manual backups are never pruned")
	cmd.Flags().BoolVar(&schedule.Enabled, "enabled", true, "enable new captures; set false to pause")
	cmd.Flags().StringVar(&schedule.DisplayName, "display-name", "", "schedule display name")
	return cmd
}
