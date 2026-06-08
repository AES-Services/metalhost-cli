package command

import (
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	computev1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/compute/v1"
)

func newComputeCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "vm",
		Aliases: []string{"vms", "compute"},
		Short:   "Manage virtual machines",
		Example: examples(`
  metalhost vm list
  metalhost vm get projects/p/virtual-machines/web-1
  metalhost vm create --vcpus 4 --ram-gib 16 --cpu-class cascadelake \
      --image ubuntu-24-04 --disk-size-gib 40 --ssh-key-name projects/p/ssh-keys/laptop
  metalhost vm apply -f vm.yaml
  metalhost vm delete projects/p/virtual-machines/web-1 --yes`),
	}
	cmd.AddCommand(newVMCommands(opts)...)
	attachNameCompleter(cmd, vmNameCompleter(opts))
	cmd.AddCommand(newSSHKeyCommands(opts))
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
			return doList(cmd, ctx, client.ListVirtualMachines, &computev1.ListVirtualMachinesRequest{ProjectName: projectName, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}, pages.all)
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

	var (
		createProject, region                    string
		image, bootURL, bootDiskName             string
		gpuModel                                 string
		sshKey, userData, hostname               string
		linuxUser, password                      string
		network, billingModeRaw, cpuClass        string
		sshKeyNames, labelPairs, annotationPairs []string
		diskSize, vcpus, ramGib, gpuCount        int32
		autorenew, assignPubIPv4, sudo           bool
	)
	create := &cobra.Command{Use: "create", Short: "Create a VM", Long: "Create a VM from flags. For a full declarative spec (multiple users, etc.) use `vm apply -f`.", RunE: func(cmd *cobra.Command, _ []string) error {
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
		billingMode, err := parseBillingMode(billingModeRaw)
		if err != nil {
			return err
		}
		if vcpus <= 0 || ramGib <= 0 || cpuClass == "" {
			return fmt.Errorf("--vcpus, --ram-gib, and --cpu-class are required")
		}

		// Boot source: at most one of --image (catalog id) / --boot-url (raw URL) / --boot-disk-name
		// (pre-existing Disk). image/url need a --disk-size-gib.
		boot := &computev1.VMBootSpec{}
		bootSources := 0
		if image != "" {
			boot.Image, boot.DiskGib, bootSources = image, diskSize, bootSources+1
		}
		if bootURL != "" {
			boot.ImageUrl, boot.DiskGib, bootSources = bootURL, diskSize, bootSources+1
		}
		if bootDiskName != "" {
			boot.DiskName, bootSources = bootDiskName, bootSources+1
		}
		if bootSources > 1 {
			return fmt.Errorf("specify at most one of --image, --boot-url, or --boot-disk-name")
		}
		if (image != "" || bootURL != "") && diskSize < 1 {
			return fmt.Errorf("--disk-size-gib must be at least 1 when using --image/--boot-url")
		}

		computeSpec := &computev1.VMComputeSpec{Vcpus: vcpus, RamGib: ramGib, CpuClass: cpuClass}
		if gpuModel != "" || gpuCount > 0 {
			computeSpec.Gpu = &computev1.GPUSpec{Model: gpuModel, Count: gpuCount}
		}

		// One login user from the flags; `vm apply -f` covers the multi-user case. A user is added
		// only when there's something to log in with (name/keys/password).
		var users []*computev1.UserSpec
		if linuxUser != "" || password != "" || sshKey != "" || len(sshKeyNames) > 0 {
			name := linuxUser
			if name == "" {
				name = "ubuntu"
			}
			users = append(users, &computev1.UserSpec{
				Name: name, Password: password, Sudo: sudo, SshKeys: sshKeyNames, SshPubkey: sshKey,
			})
		}

		manifest := &computev1.VirtualMachineManifest{
			ApiVersion: "compute.metalhost.io/v1",
			Kind:       "VirtualMachine",
			Metadata: &computev1.VirtualMachineMetadata{
				Name:        hostname,
				Project:     projectName,
				Labels:      stringMapFromPairs(labelPairs),
				Annotations: stringMapFromPairs(annotationPairs),
			},
			Spec: &computev1.VirtualMachineSpec{
				Region:    region,
				Compute:   computeSpec,
				Boot:      boot,
				Network:   &computev1.VMNetworkSpec{Network: network, PublicIpv4: assignPubIPv4},
				Users:     users,
				CloudInit: userData,
				Billing:   &computev1.VMBillingSpec{Mode: billingMode, Autorenew: autorenew},
			},
		}

		client, err := ctx.computeClient()
		if err != nil {
			return err
		}
		resp, err := client.CreateVirtualMachine(cmd.Context(), connect.NewRequest(&computev1.CreateVirtualMachineRequest{Manifest: manifest}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	create.Flags().StringVar(&createProject, "project", "", "project (defaults to active profile project)")
	create.Flags().StringVar(&region, "region", "", "datacenter, e.g. datacenters/us-dal-1 (defaults to profile region)")
	create.Flags().Int32Var(&vcpus, "vcpus", 0, "vCPU count (required)")
	create.Flags().Int32Var(&ramGib, "ram-gib", 0, "RAM GiB (required)")
	create.Flags().StringVar(&cpuClass, "cpu-class", "", "CPU class, e.g. cascadelake (required)")
	create.Flags().StringVar(&gpuModel, "gpu-model", "", "GPU model for whole-GPU passthrough, e.g. rtx4090")
	create.Flags().Int32Var(&gpuCount, "gpu-count", 0, "number of GPUs to attach (with --gpu-model)")
	create.Flags().StringVar(&image, "image", "", "catalog image id, e.g. ubuntu-24-04 (resolved server-side; alternative to --boot-url)")
	create.Flags().StringVar(&bootURL, "boot-url", "", "streamable raw boot image URL (CDI http source)")
	create.Flags().StringVar(&bootDiskName, "boot-disk-name", "", "use a pre-existing Disk as boot, e.g. projects/p/disks/my-disk")
	create.Flags().Int32Var(&diskSize, "disk-size-gib", 0, "boot disk size GiB (required with --image/--boot-url)")
	create.Flags().StringVar(&hostname, "hostname", "", "VM hostname (DNS-1123 label, ≤63 chars; defaults to a UUID slug)")
	create.Flags().BoolVar(&assignPubIPv4, "assign-public-ipv4", false, "allocate a public IPv4 from the DC pool")
	create.Flags().StringVar(&network, "network", "", "tenant network resource name, e.g. projects/p/networks/default (defaults to project's default)")
	create.Flags().StringVar(&linuxUser, "user", "", "login username to create (default ubuntu when keys/password given)")
	create.Flags().StringVar(&password, "password", "", "login password for --user (sent over TLS, hashed server-side)")
	create.Flags().BoolVar(&sudo, "sudo", true, "grant the user passwordless sudo")
	create.Flags().StringVar(&sshKey, "ssh-key", "", "OpenSSH public key line (inline; alternative to --ssh-key-name)")
	create.Flags().StringSliceVar(&sshKeyNames, "ssh-key-name", nil, "registered SSH key resource name(s), e.g. projects/p/ssh-keys/laptop (repeatable)")
	create.Flags().StringVar(&userData, "user-data", "", "cloud-init user-data (#cloud-config YAML); when set, owns user creation")
	create.Flags().StringVar(&billingModeRaw, "billing-mode", "", "BILLING_MODE_HOURLY (default), BILLING_MODE_MONTHLY_1, monthly-3, monthly-6, monthly-12")
	create.Flags().BoolVar(&autorenew, "autorenew", false, "auto-renew prepaid monthly term (ignored for hourly billing)")
	create.Flags().StringSliceVar(&labelPairs, "label", nil, "labels as key=value (repeatable)")
	create.Flags().StringSliceVar(&annotationPairs, "annotation", nil, "annotations as key=value (repeatable)")

	return []*cobra.Command{
		list, get, create, vmApplyCommand(opts),
		vmDeleteCommand(opts),
		vmStartCommand(opts),
		vmStopCommand(opts),
		vmRestartCommand(opts),
		vmResizeCommand(opts),
		vmReimageCommand(opts),
		vmCloneCommand(opts),
		vmConsoleCommand(opts),
		vmMetricsCommand(opts),
		vmAutorenewCommand(opts),
		vmRenewNowCommand(opts),
		vmSnapshotCommand(opts),
		vmFromBackupCommand(opts),
	}
}

func vmResizeCommand(opts *rootOptions) *cobra.Command {
	var (
		vcpus, ramGib int32
		cpuClass      string
	)
	cmd := &cobra.Command{Use: "resize NAME", Short: "Resize a VM (change vCPU/RAM/CPU class)", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		if vcpus <= 0 || ramGib <= 0 || cpuClass == "" {
			return fmt.Errorf("--vcpus, --ram-gib, and --cpu-class are required")
		}
		client, err := ctx.computeClient()
		if err != nil {
			return err
		}
		resp, err := client.ResizeVirtualMachine(cmd.Context(), connect.NewRequest(&computev1.ResizeVirtualMachineRequest{
			Name:     args[0],
			Vcpus:    vcpus,
			RamGib:   ramGib,
			CpuClass: cpuClass,
		}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	cmd.Flags().Int32Var(&vcpus, "vcpus", 0, "new vCPU count (required)")
	cmd.Flags().Int32Var(&ramGib, "ram-gib", 0, "new RAM GiB (required)")
	cmd.Flags().StringVar(&cpuClass, "cpu-class", "", "new CPU class, e.g. cascadelake (required)")
	return cmd
}

func vmReimageCommand(opts *rootOptions) *cobra.Command {
	var bootURL, token string
	cmd := &cobra.Command{Use: "reimage NAME", Short: "Reimage a VM (destroys data — requires confirmation token)", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.computeClient()
		if err != nil {
			return err
		}
		resp, err := client.ReimageVirtualMachine(cmd.Context(), connect.NewRequest(&computev1.ReimageVirtualMachineRequest{
			Name:              args[0],
			BootImageUrl:      bootURL,
			ConfirmationToken: token,
		}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	cmd.Flags().StringVar(&bootURL, "boot-url", "", "streamable raw boot image URL (empty reuses original boot source)")
	cmd.Flags().StringVar(&token, "confirm", "", "confirmation token — must be REIMAGE/<vm-resource-name>")
	return cmd
}

func vmCloneCommand(opts *rootOptions) *cobra.Command {
	var src, displayName string
	var labelPairs, annotationPairs []string
	cmd := &cobra.Command{Use: "clone", Short: "Clone a VM (disk-level copy)", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.computeClient()
		if err != nil {
			return err
		}
		resp, err := client.CloneVirtualMachine(cmd.Context(), connect.NewRequest(&computev1.CloneVirtualMachineRequest{
			SourceVmName:      src,
			TargetDisplayName: displayName,
			Labels:            stringMapFromPairs(labelPairs),
			Annotations:       stringMapFromPairs(annotationPairs),
		}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	cmd.Flags().StringVar(&src, "source", "", "source VM resource name (required)")
	cmd.Flags().StringVar(&displayName, "display-name", "", "display name for the clone (required)")
	cmd.Flags().StringSliceVar(&labelPairs, "label", nil, "labels for the new VM as key=value (repeatable; NOT inherited from source)")
	cmd.Flags().StringSliceVar(&annotationPairs, "annotation", nil, "annotations as key=value (repeatable; NOT inherited from source)")
	return cmd
}

func vmConsoleCommand(opts *rootOptions) *cobra.Command {
	var kind string
	cmd := &cobra.Command{Use: "console NAME", Short: "Open a console (serial or VNC) — returns a short-lived WebSocket URL", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.computeClient()
		if err != nil {
			return err
		}
		consoleType := computev1.ConsoleType_CONSOLE_TYPE_SERIAL
		switch strings.ToLower(strings.TrimSpace(kind)) {
		case "vnc":
			consoleType = computev1.ConsoleType_CONSOLE_TYPE_VNC
		case "", "serial":
			consoleType = computev1.ConsoleType_CONSOLE_TYPE_SERIAL
		}
		resp, err := client.OpenConsole(cmd.Context(), connect.NewRequest(&computev1.OpenConsoleRequest{Name: args[0], Type: consoleType}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	cmd.Flags().StringVar(&kind, "type", "serial", "serial | vnc")
	return cmd
}

func vmMetricsCommand(opts *rootOptions) *cobra.Command {
	var lookback time.Duration
	var step int32
	cmd := &cobra.Command{Use: "metrics NAME", Short: "Fetch VM metrics (CPU/memory/network)", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.computeClient()
		if err != nil {
			return err
		}
		end := time.Now().Unix()
		start := time.Now().Add(-lookback).Unix()
		resp, err := client.GetVMMetrics(cmd.Context(), connect.NewRequest(&computev1.GetVMMetricsRequest{
			Name:          args[0],
			StartTimeUnix: start,
			EndTimeUnix:   end,
			StepSeconds:   step,
		}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	cmd.Flags().DurationVar(&lookback, "since", time.Hour, "lookback window")
	cmd.Flags().Int32Var(&step, "step-seconds", 60, "sample step in seconds (max 3600)")
	return cmd
}

func vmAutorenewCommand(opts *rootOptions) *cobra.Command {
	var enabled bool
	cmd := &cobra.Command{Use: "autorenew NAME", Short: "Toggle auto-renew on a prepaid monthly VM", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.computeClient()
		if err != nil {
			return err
		}
		resp, err := client.SetVMAutorenew(cmd.Context(), connect.NewRequest(&computev1.SetVMAutorenewRequest{Name: args[0], Autorenew: enabled}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	cmd.Flags().BoolVar(&enabled, "enabled", false, "true to enable auto-renew, false to disable")
	return cmd
}

func vmRenewNowCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "renew NAME", Short: "Renew a prepaid monthly VM immediately", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.computeClient()
		if err != nil {
			return err
		}
		resp, err := client.RenewVMNow(cmd.Context(), connect.NewRequest(&computev1.RenewVMNowRequest{Name: args[0]}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
}

func vmSnapshotCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "snapshot", Aliases: []string{"snapshots", "backup", "backups"}, Short: "Manage VM snapshots / backups"}

	var snapVM, snapDisplay string
	var snapLabelPairs, snapAnnotationPairs []string
	create := &cobra.Command{Use: "create", Short: "Create a snapshot of a VM", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.computeClient()
		if err != nil {
			return err
		}
		resp, err := client.SnapshotVirtualMachine(cmd.Context(), connect.NewRequest(&computev1.SnapshotVirtualMachineRequest{
			VmName:      snapVM,
			DisplayName: snapDisplay,
			Labels:      stringMapFromPairs(snapLabelPairs),
			Annotations: stringMapFromPairs(snapAnnotationPairs),
		}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	create.Flags().StringVar(&snapVM, "vm", "", "source VM resource name (required)")
	create.Flags().StringVar(&snapDisplay, "display-name", "", "display name for the snapshot (required)")
	create.Flags().StringSliceVar(&snapLabelPairs, "label", nil, "labels as key=value (repeatable)")
	create.Flags().StringSliceVar(&snapAnnotationPairs, "annotation", nil, "annotations as key=value (repeatable)")
	cmd.AddCommand(create)

	var pages pageFlags
	var listProject, listSrc string
	list := &cobra.Command{Use: "list", Short: "List VM snapshots in a project", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		projectName, err := requireProject(ctx, listProject)
		if err != nil {
			return err
		}
		client, err := ctx.computeClient()
		if err != nil {
			return err
		}
		return doList(cmd, ctx, client.ListVmSnapshots, &computev1.ListVmSnapshotsRequest{ProjectName: projectName, SourceVm: listSrc, PageSize: effectivePageSize(pages), PageToken: pages.pageToken}, pages.all)
	}}
	addPageFlags(list, &pages)
	list.Flags().StringVar(&listProject, "project", "", "project (defaults to active project)")
	list.Flags().StringVar(&listSrc, "vm", "", "filter to snapshots from this source VM")
	cmd.AddCommand(list)

	cmd.AddCommand(&cobra.Command{Use: "get NAME", Short: "Get a VM snapshot", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		client, err := ctx.computeClient()
		if err != nil {
			return err
		}
		resp, err := client.GetVmSnapshot(cmd.Context(), connect.NewRequest(&computev1.GetVmSnapshotRequest{Name: args[0]}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}})

	var snapDelYes bool
	snapDelete := &cobra.Command{Use: "delete NAME", Short: "Delete a VM snapshot", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		if err := confirmDestructive(cmd, snapDelYes, "Delete VM snapshot", args[0]); err != nil {
			return err
		}
		client, err := ctx.computeClient()
		if err != nil {
			return err
		}
		resp, err := client.DeleteVmSnapshot(cmd.Context(), connect.NewRequest(&computev1.DeleteVmSnapshotRequest{Name: args[0]}))
		if err != nil {
			return err
		}
		return writeDeleted(cmd, ctx, "vm-snapshot", args[0], resp.Msg)
	}}
	snapDelete.Flags().BoolVar(&snapDelYes, "yes", false, "skip the interactive confirmation prompt")
	cmd.AddCommand(snapDelete)

	return cmd
}

func vmFromBackupCommand(opts *rootOptions) *cobra.Command {
	var snapshot, displayName, hostname, billingModeRaw, userData string
	var sshKeyNames []string
	var assignPubIPv4, autorenew bool
	var vcpus, ramGib int32
	var cpuClass string
	cmd := &cobra.Command{Use: "from-backup", Short: "Create a new VM from a VmSnapshot", RunE: func(cmd *cobra.Command, _ []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		billingMode, err := parseBillingMode(billingModeRaw)
		if err != nil {
			return err
		}
		client, err := ctx.computeClient()
		if err != nil {
			return err
		}
		resp, err := client.CreateVirtualMachineFromBackup(cmd.Context(), connect.NewRequest(&computev1.CreateVirtualMachineFromBackupRequest{
			VmSnapshotName:    snapshot,
			TargetDisplayName: displayName,
			Hostname:          hostname,
			Vcpus:             vcpus,
			RamGib:            ramGib,
			CpuClass:          cpuClass,
			BillingMode:       billingMode,
			Autorenew:         autorenew,
			AssignPublicIpv4:  assignPubIPv4,
			SshKeyNames:       sshKeyNames,
			UserData:          userData,
		}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	cmd.Flags().StringVar(&snapshot, "snapshot", "", "source VmSnapshot resource name (required)")
	cmd.Flags().StringVar(&displayName, "display-name", "", "display name for the new VM (required)")
	cmd.Flags().StringVar(&hostname, "hostname", "", "hostname (defaults to sanitised display name)")
	cmd.Flags().Int32Var(&vcpus, "vcpus", 0, "configurator override: vCPUs")
	cmd.Flags().Int32Var(&ramGib, "ram-gib", 0, "configurator override: RAM GiB")
	cmd.Flags().StringVar(&cpuClass, "cpu-class", "", "configurator override: CPU class")
	cmd.Flags().StringVar(&billingModeRaw, "billing-mode", "", "BILLING_MODE_HOURLY (default), BILLING_MODE_MONTHLY_1, …")
	cmd.Flags().BoolVar(&autorenew, "autorenew", false, "auto-renew prepaid monthly term")
	cmd.Flags().BoolVar(&assignPubIPv4, "assign-public-ipv4", false, "attach a public IPv4")
	cmd.Flags().StringSliceVar(&sshKeyNames, "ssh-key-name", nil, "registered SSH key resource name(s)")
	cmd.Flags().StringVar(&userData, "user-data", "", "cloud-init user data override")
	return cmd
}

func vmDeleteCommand(opts *rootOptions) *cobra.Command {
	var yes, deleteDisks bool
	cmd := &cobra.Command{Use: "delete NAME", Short: "Delete VM", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		if err := confirmDestructive(cmd, yes, "Delete VM", args[0]); err != nil {
			return err
		}
		client, err := ctx.computeClient()
		if err != nil {
			return err
		}
		resp, err := client.DeleteVirtualMachine(cmd.Context(), connect.NewRequest(&computev1.DeleteVirtualMachineRequest{
			Name:                args[0],
			DeleteAttachedDisks: deleteDisks,
		}))
		if err != nil {
			return err
		}
		return ctx.write(resp.Msg)
	}}
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the interactive confirmation prompt")
	cmd.Flags().BoolVar(&deleteDisks, "delete-disks", false, "also delete the attached disks (boot + data) instead of detaching them back to AVAILABLE")
	return cmd
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

func parseBillingMode(raw string) (computev1.BillingMode, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return computev1.BillingMode_BILLING_MODE_UNSPECIFIED, nil
	}
	u := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(s, "-", "_"), " ", "_"))
	if v, ok := computev1.BillingMode_value[u]; ok {
		return computev1.BillingMode(v), nil
	}
	switch u {
	case "HOURLY":
		return computev1.BillingMode_BILLING_MODE_HOURLY, nil
	case "MONTHLY_1", "MONTHLY1":
		return computev1.BillingMode_BILLING_MODE_MONTHLY_1, nil
	case "MONTHLY_3", "MONTHLY3":
		return computev1.BillingMode_BILLING_MODE_MONTHLY_3, nil
	case "MONTHLY_6", "MONTHLY6":
		return computev1.BillingMode_BILLING_MODE_MONTHLY_6, nil
	case "MONTHLY_12", "MONTHLY12":
		return computev1.BillingMode_BILLING_MODE_MONTHLY_12, nil
	default:
		return 0, fmt.Errorf("unknown billing mode %q (examples: BILLING_MODE_HOURLY, monthly-3)", raw)
	}
}
