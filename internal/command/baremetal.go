package command

import (
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	baremetalv1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/baremetal/v1"
)

// newBareMetalCommand exposes the full bare-metal lifecycle: browse the fleet, quote + order a
// host lease, run BMC operations (power / reinstall / console / rescue / boot device), and manage
// the org's bring-your-own-OS ISO library. Mirrors the per-service VM command tree.
func newBareMetalCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "baremetal",
		Aliases: []string{"bare-metal", "bm"},
		Short:   "Browse, lease, and operate bare-metal hosts",
		Example: examples(`
  metalhost baremetal offers
  metalhost baremetal quote --host hosts/dal-r740-01 --billing-mode monthly-1
  metalhost baremetal order my-box --host hosts/dal-r740-01 --os-image ubuntu-2404
  metalhost baremetal power my-box --action reboot
  metalhost baremetal iso list`),
	}
	cmd.AddCommand(
		bmOffersCommand(opts),
		bmInventoryCommand(opts),
		bmQuoteCommand(opts),
		bmOrderCommand(opts),
		bmListCommand(opts),
		bmGetCommand(opts),
		bmReleaseCommand(opts),
		bmRenewCommand(opts),
		bmPowerCommand(opts),
		bmReinstallCommand(opts),
		bmConsoleCommand(opts),
		bmRescueCommand(opts),
		bmBootDeviceCommand(opts),
		bmISOCommand(opts),
	)
	return cmd
}

// --- Browse / quote / order ---------------------------------------------------------------

func bmOffersCommand(opts *rootOptions) *cobra.Command {
	var pages pageFlags
	var dc string
	cmd := &cobra.Command{
		Use:     "offers",
		Aliases: []string{"available"},
		Short:   "List leasable bare-metal hosts (with specs + monthly price)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.bareMetalClient()
			if err != nil {
				return err
			}
			return doList(cmd, ctx, client.ListAvailableBareMetal, &baremetalv1.ListAvailableBareMetalRequest{
				DatacenterName: qualifyName(dc, "datacenters/"),
				PageSize:       effectivePageSize(pages),
				PageToken:      pages.pageToken,
			}, pages.all)
		},
	}
	addPageFlags(cmd, &pages)
	cmd.Flags().StringVar(&dc, "datacenter", "", "filter to one datacenter, e.g. us-dal-1")
	return cmd
}

func bmInventoryCommand(opts *rootOptions) *cobra.Command {
	var pages pageFlags
	var dc string
	cmd := &cobra.Command{
		Use:   "inventory",
		Short: "List the full published bare-metal fleet (available + leased)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.bareMetalClient()
			if err != nil {
				return err
			}
			return doList(cmd, ctx, client.ListBareMetalInventory, &baremetalv1.ListBareMetalInventoryRequest{
				DatacenterName: qualifyName(dc, "datacenters/"),
				PageSize:       effectivePageSize(pages),
				PageToken:      pages.pageToken,
			}, pages.all)
		},
	}
	addPageFlags(cmd, &pages)
	cmd.Flags().StringVar(&dc, "datacenter", "", "filter to one datacenter, e.g. us-dal-1")
	return cmd
}

func bmQuoteCommand(opts *rootOptions) *cobra.Command {
	var host, billingModeRaw, currency string
	var ipv4PrefixLen int32
	cmd := &cobra.Command{
		Use:   "quote",
		Short: "Quote a host lease (hourly rate + monthly prepay)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			mode, err := parseBillingMode(billingModeRaw)
			if err != nil {
				return err
			}
			client, err := ctx.bareMetalClient()
			if err != nil {
				return err
			}
			resp, err := client.QuoteBareMetal(cmd.Context(), connect.NewRequest(&baremetalv1.QuoteBareMetalRequest{
				HostName:            qualifyName(host, "hosts/"),
				BillingMode:         mode,
				Currency:            currency,
				PublicIpv4PrefixLen: ipv4PrefixLen,
			}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}
	cmd.Flags().StringVar(&host, "host", "", "host to quote, e.g. hosts/dal-r740-01 (required)")
	cmd.Flags().StringVar(&billingModeRaw, "billing-mode", "", "BILLING_MODE_HOURLY (default), monthly-1, monthly-3, monthly-6, monthly-12")
	cmd.Flags().StringVar(&currency, "currency", "", "USD (default) or USDC")
	cmd.Flags().Int32Var(&ipv4PrefixLen, "ipv4-prefix-len", 0, "public IPv4 block size: 32 (default) / 31 / 30 / 29 / 28")
	_ = cmd.MarkFlagRequired("host")
	return cmd
}

func bmOrderCommand(opts *rootOptions) *cobra.Command {
	var (
		project, host, billingModeRaw                string
		osImage, customImageURL, hostname, linuxUser string
		userData                                     string
		sshKeyNames, labelPairs, annotationPairs     []string
		autorenew                                    bool
		ipv4PrefixLen                                int32
	)
	cmd := &cobra.Command{
		Use:     "order NAME",
		Aliases: []string{"create", "lease"},
		Short:   "Lease a bare-metal host (NAME is a bare slug or bare-metal-instances/<slug>)",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			if strings.TrimSpace(host) == "" {
				return fmt.Errorf("--host is required (see `metalhost baremetal offers`)")
			}
			projectName, err := requireProject(ctx, project)
			if err != nil {
				return err
			}
			mode, err := parseBillingMode(billingModeRaw)
			if err != nil {
				return err
			}
			client, err := ctx.bareMetalClient()
			if err != nil {
				return err
			}
			resp, err := client.CreateBareMetalInstance(cmd.Context(), connect.NewRequest(&baremetalv1.CreateBareMetalInstanceRequest{
				Name:                qualifyName(args[0], "bare-metal-instances/"),
				ProjectName:         projectName,
				HostName:            qualifyName(host, "hosts/"),
				BillingMode:         mode,
				Autorenew:           autorenew,
				OsImage:             osImage,
				CustomImageUrl:      customImageURL,
				Hostname:            hostname,
				LinuxUsername:       linuxUser,
				SshKeyNames:         sshKeyNames,
				UserData:            userData,
				PublicIpv4PrefixLen: ipv4PrefixLen,
				Labels:              stringMapFromPairs(labelPairs),
				Annotations:         stringMapFromPairs(annotationPairs),
			}))
			if err != nil {
				return err
			}
			return ctx.write(unwrapSingleResource(resp.Msg))
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "project (defaults to active profile project)")
	cmd.Flags().StringVar(&host, "host", "", "specific host to lease, e.g. hosts/dal-r740-01 (required)")
	cmd.Flags().StringVar(&billingModeRaw, "billing-mode", "", "BILLING_MODE_HOURLY (default), monthly-1, monthly-3, monthly-6, monthly-12")
	cmd.Flags().BoolVar(&autorenew, "autorenew", false, "auto-renew a prepaid monthly term (ignored for hourly)")
	cmd.Flags().StringVar(&osImage, "os-image", "", "catalog image id for a managed install, e.g. ubuntu-2404")
	cmd.Flags().StringVar(&customImageURL, "custom-image-url", "", "direct image URL (overrides --os-image)")
	cmd.Flags().StringVar(&hostname, "hostname", "", "cloud-init hostname")
	cmd.Flags().StringVar(&linuxUser, "user", "", "cloud-init login username")
	cmd.Flags().StringSliceVar(&sshKeyNames, "ssh-key-name", nil, "registered SSH key resource name(s) (repeatable)")
	cmd.Flags().StringVar(&userData, "user-data", "", "cloud-init user-data (#cloud-config YAML)")
	cmd.Flags().Int32Var(&ipv4PrefixLen, "ipv4-prefix-len", 0, "public IPv4 block size: 32 (default) / 31 / 30 / 29 / 28")
	cmd.Flags().StringSliceVar(&labelPairs, "label", nil, "labels as key=value (repeatable)")
	cmd.Flags().StringSliceVar(&annotationPairs, "annotation", nil, "annotations as key=value (repeatable)")
	_ = cmd.MarkFlagRequired("host")
	return cmd
}

// --- Instances ----------------------------------------------------------------------------

func bmListCommand(opts *rootOptions) *cobra.Command {
	var pages pageFlags
	var project string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List your bare-metal instances",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			projectName, err := requireProject(ctx, project)
			if err != nil {
				return err
			}
			client, err := ctx.bareMetalClient()
			if err != nil {
				return err
			}
			return doList(cmd, ctx, client.ListBareMetalInstances, &baremetalv1.ListBareMetalInstancesRequest{
				ProjectName: projectName,
				PageSize:    effectivePageSize(pages),
				PageToken:   pages.pageToken,
			}, pages.all)
		},
	}
	addPageFlags(cmd, &pages)
	cmd.Flags().StringVar(&project, "project", "", "project (defaults to active profile project)")
	return cmd
}

func bmGetCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "get NAME",
		Short: "Get a bare-metal instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.bareMetalClient()
			if err != nil {
				return err
			}
			resp, err := client.GetBareMetalInstance(cmd.Context(), connect.NewRequest(&baremetalv1.GetBareMetalInstanceRequest{
				Name: qualifyName(args[0], "bare-metal-instances/"),
			}))
			if err != nil {
				return err
			}
			return ctx.write(unwrapSingleResource(resp.Msg))
		},
	}
}

func bmReleaseCommand(opts *rootOptions) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "release NAME",
		Short: "Release a bare-metal lease (refused while a monthly term is active)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			name := qualifyName(args[0], "bare-metal-instances/")
			if err := confirmDestructive(cmd, yes, "Release bare-metal lease", name); err != nil {
				return err
			}
			client, err := ctx.bareMetalClient()
			if err != nil {
				return err
			}
			if _, err := client.ReleaseBareMetalInstance(cmd.Context(), connect.NewRequest(&baremetalv1.ReleaseBareMetalInstanceRequest{
				Name: name,
			})); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Released %s\n", name)
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the interactive confirmation prompt")
	return cmd
}

func bmRenewCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "renew NAME",
		Short: "Renew a prepaid monthly lease immediately",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.bareMetalClient()
			if err != nil {
				return err
			}
			resp, err := client.RenewBareMetalNow(cmd.Context(), connect.NewRequest(&baremetalv1.RenewBareMetalNowRequest{
				Name: qualifyName(args[0], "bare-metal-instances/"),
			}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}
}

// --- BMC operations -----------------------------------------------------------------------

func bmPowerCommand(opts *rootOptions) *cobra.Command {
	var action string
	cmd := &cobra.Command{
		Use:   "power NAME",
		Short: "BMC power action (on | off | reboot)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			act, err := parseBareMetalPowerAction(action)
			if err != nil {
				return err
			}
			client, err := ctx.bareMetalClient()
			if err != nil {
				return err
			}
			resp, err := client.SetBareMetalPower(cmd.Context(), connect.NewRequest(&baremetalv1.SetBareMetalPowerRequest{
				Name:   qualifyName(args[0], "bare-metal-instances/"),
				Action: act,
			}))
			if err != nil {
				return err
			}
			return ctx.write(unwrapSingleResource(resp.Msg))
		},
	}
	cmd.Flags().StringVar(&action, "action", "", "on | off | reboot (required)")
	_ = cmd.MarkFlagRequired("action")
	return cmd
}

func bmReinstallCommand(opts *rootOptions) *cobra.Command {
	var (
		osImage, customImageURL, hostname, linuxUser, userData string
		sshKeyNames                                            []string
		yes                                                    bool
	)
	cmd := &cobra.Command{
		Use:   "reinstall NAME",
		Short: "Reinstall the OS on a lease (wipes the OS disk)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			name := qualifyName(args[0], "bare-metal-instances/")
			if err := confirmDestructive(cmd, yes, "Reinstall (wipes the OS disk on)", name); err != nil {
				return err
			}
			client, err := ctx.bareMetalClient()
			if err != nil {
				return err
			}
			resp, err := client.ReinstallBareMetal(cmd.Context(), connect.NewRequest(&baremetalv1.ReinstallBareMetalRequest{
				Name:           name,
				OsImage:        osImage,
				CustomImageUrl: customImageURL,
				Hostname:       hostname,
				LinuxUsername:  linuxUser,
				SshKeyNames:    sshKeyNames,
				UserData:       userData,
			}))
			if err != nil {
				return err
			}
			return ctx.write(unwrapSingleResource(resp.Msg))
		},
	}
	cmd.Flags().StringVar(&osImage, "os-image", "", "new catalog image id (empty keeps the current image)")
	cmd.Flags().StringVar(&customImageURL, "custom-image-url", "", "direct image URL (overrides --os-image)")
	cmd.Flags().StringVar(&hostname, "hostname", "", "cloud-init hostname override")
	cmd.Flags().StringVar(&linuxUser, "user", "", "cloud-init login username override")
	cmd.Flags().StringSliceVar(&sshKeyNames, "ssh-key-name", nil, "registered SSH key resource name(s) (repeatable)")
	cmd.Flags().StringVar(&userData, "user-data", "", "cloud-init user-data override (#cloud-config YAML)")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the interactive confirmation prompt")
	return cmd
}

func bmConsoleCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "console NAME",
		Short: "Get a short-lived BMC KVM console URL (+ VNC password)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.bareMetalClient()
			if err != nil {
				return err
			}
			resp, err := client.GetBareMetalConsoleURL(cmd.Context(), connect.NewRequest(&baremetalv1.GetBareMetalConsoleURLRequest{
				Name: qualifyName(args[0], "bare-metal-instances/"),
			}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}
}

func bmRescueCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "rescue", Short: "Enter or exit BMC rescue mode"}

	var rescueImageURL string
	enter := &cobra.Command{
		Use:   "enter NAME",
		Short: "Boot the host into rescue mode (one-time boot override + power cycle)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.bareMetalClient()
			if err != nil {
				return err
			}
			resp, err := client.EnterBareMetalRescueMode(cmd.Context(), connect.NewRequest(&baremetalv1.EnterBareMetalRescueModeRequest{
				Name:           qualifyName(args[0], "bare-metal-instances/"),
				RescueImageUrl: rescueImageURL,
			}))
			if err != nil {
				return err
			}
			return ctx.write(unwrapSingleResource(resp.Msg))
		},
	}
	enter.Flags().StringVar(&rescueImageURL, "rescue-image-url", "", "override the rescue image URL (defaults to the DC's configured image)")

	exit := &cobra.Command{
		Use:   "exit NAME",
		Short: "Exit rescue mode (clear the override + power cycle back to disk)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.bareMetalClient()
			if err != nil {
				return err
			}
			resp, err := client.ExitBareMetalRescueMode(cmd.Context(), connect.NewRequest(&baremetalv1.ExitBareMetalRescueModeRequest{
				Name: qualifyName(args[0], "bare-metal-instances/"),
			}))
			if err != nil {
				return err
			}
			return ctx.write(unwrapSingleResource(resp.Msg))
		},
	}
	cmd.AddCommand(enter, exit)
	return cmd
}

func bmBootDeviceCommand(opts *rootOptions) *cobra.Command {
	var device string
	cmd := &cobra.Command{
		Use:   "boot-device NAME",
		Short: "Set the one-time boot device (disk | cdrom | pxe) and power-cycle",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			dev, err := parseBareMetalBootDevice(device)
			if err != nil {
				return err
			}
			client, err := ctx.bareMetalClient()
			if err != nil {
				return err
			}
			resp, err := client.SetBareMetalBootDevice(cmd.Context(), connect.NewRequest(&baremetalv1.SetBareMetalBootDeviceRequest{
				Name:   qualifyName(args[0], "bare-metal-instances/"),
				Device: dev,
			}))
			if err != nil {
				return err
			}
			return ctx.write(unwrapSingleResource(resp.Msg))
		},
	}
	cmd.Flags().StringVar(&device, "device", "", "disk | cdrom | pxe (required)")
	_ = cmd.MarkFlagRequired("device")
	return cmd
}

// --- ISO library (bring-your-own-OS) ------------------------------------------------------

func bmISOCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "iso", Short: "Manage the org's install-ISO library + virtual media"}
	cmd.AddCommand(
		bmISOListCommand(opts),
		bmISOUploadURLCommand(opts),
		bmISOFromURLCommand(opts),
		bmISODeleteCommand(opts),
		bmISOAttachCommand(opts),
		bmISODetachCommand(opts),
		bmISOStatusCommand(opts),
	)
	return cmd
}

func bmISOListCommand(opts *rootOptions) *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the org's uploaded install ISOs",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			projectName, err := requireProject(ctx, project)
			if err != nil {
				return err
			}
			client, err := ctx.bareMetalClient()
			if err != nil {
				return err
			}
			resp, err := client.ListBareMetalISOs(cmd.Context(), connect.NewRequest(&baremetalv1.ListBareMetalISOsRequest{
				ProjectName: projectName,
			}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "project whose org owns the library (defaults to active profile project)")
	return cmd
}

func bmISOUploadURLCommand(opts *rootOptions) *cobra.Command {
	var project, filename string
	cmd := &cobra.Command{
		Use:   "upload-url",
		Short: "Get a presigned PUT URL to upload an install ISO",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			projectName, err := requireProject(ctx, project)
			if err != nil {
				return err
			}
			client, err := ctx.bareMetalClient()
			if err != nil {
				return err
			}
			resp, err := client.CreateBareMetalISOUploadURL(cmd.Context(), connect.NewRequest(&baremetalv1.CreateBareMetalISOUploadURLRequest{
				ProjectName: projectName,
				Filename:    filename,
			}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "project whose org owns the library (defaults to active profile project)")
	cmd.Flags().StringVar(&filename, "filename", "", "ISO filename, e.g. proxmox-ve-8.iso (required)")
	_ = cmd.MarkFlagRequired("filename")
	return cmd
}

func bmISOFromURLCommand(opts *rootOptions) *cobra.Command {
	var project, url, filename string
	cmd := &cobra.Command{
		Use:   "from-url",
		Short: "Import an ISO into the library by URL (fetched server-side)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			projectName, err := requireProject(ctx, project)
			if err != nil {
				return err
			}
			client, err := ctx.bareMetalClient()
			if err != nil {
				return err
			}
			resp, err := client.CreateBareMetalISOFromURL(cmd.Context(), connect.NewRequest(&baremetalv1.CreateBareMetalISOFromURLRequest{
				ProjectName: projectName,
				Url:         url,
				Filename:    filename,
			}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "project whose org owns the library (defaults to active profile project)")
	cmd.Flags().StringVar(&url, "url", "", "public HTTPS URL to the ISO (required)")
	cmd.Flags().StringVar(&filename, "filename", "", "filename override (defaults to the URL basename)")
	_ = cmd.MarkFlagRequired("url")
	return cmd
}

func bmISODeleteCommand(opts *rootOptions) *cobra.Command {
	var project, objectKey string
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete an ISO from the library",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			projectName, err := requireProject(ctx, project)
			if err != nil {
				return err
			}
			client, err := ctx.bareMetalClient()
			if err != nil {
				return err
			}
			if _, err := client.DeleteBareMetalISO(cmd.Context(), connect.NewRequest(&baremetalv1.DeleteBareMetalISORequest{
				ProjectName: projectName,
				ObjectKey:   objectKey,
			})); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted ISO %s\n", objectKey)
			return nil
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "project whose org owns the library (defaults to active profile project)")
	cmd.Flags().StringVar(&objectKey, "object-key", "", "ISO object key (from `iso list`) (required)")
	_ = cmd.MarkFlagRequired("object-key")
	return cmd
}

func bmISOAttachCommand(opts *rootOptions) *cobra.Command {
	var objectKey string
	cmd := &cobra.Command{
		Use:   "attach NAME",
		Short: "Mount a library ISO as the host's BMC virtual CD",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.bareMetalClient()
			if err != nil {
				return err
			}
			resp, err := client.AttachBareMetalISO(cmd.Context(), connect.NewRequest(&baremetalv1.AttachBareMetalISORequest{
				Name:      qualifyName(args[0], "bare-metal-instances/"),
				ObjectKey: objectKey,
			}))
			if err != nil {
				return err
			}
			return ctx.write(unwrapSingleResource(resp.Msg))
		},
	}
	cmd.Flags().StringVar(&objectKey, "object-key", "", "ISO object key (from `iso list`) (required)")
	_ = cmd.MarkFlagRequired("object-key")
	return cmd
}

func bmISODetachCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "detach NAME",
		Short: "Eject any mounted virtual media",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.bareMetalClient()
			if err != nil {
				return err
			}
			resp, err := client.DetachBareMetalISO(cmd.Context(), connect.NewRequest(&baremetalv1.DetachBareMetalISORequest{
				Name: qualifyName(args[0], "bare-metal-instances/"),
			}))
			if err != nil {
				return err
			}
			return ctx.write(unwrapSingleResource(resp.Msg))
		},
	}
}

func bmISOStatusCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "status NAME",
		Short: "Show the host's current virtual-media mount state",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			client, err := ctx.bareMetalClient()
			if err != nil {
				return err
			}
			resp, err := client.GetBareMetalVirtualMedia(cmd.Context(), connect.NewRequest(&baremetalv1.GetBareMetalVirtualMediaRequest{
				Name: qualifyName(args[0], "bare-metal-instances/"),
			}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}
}

// --- enum parsers -------------------------------------------------------------------------

func parseBareMetalPowerAction(raw string) (baremetalv1.BareMetalPowerAction, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "on":
		return baremetalv1.BareMetalPowerAction_BARE_METAL_POWER_ACTION_ON, nil
	case "off":
		return baremetalv1.BareMetalPowerAction_BARE_METAL_POWER_ACTION_OFF, nil
	case "reboot", "restart", "cycle":
		return baremetalv1.BareMetalPowerAction_BARE_METAL_POWER_ACTION_REBOOT, nil
	default:
		return baremetalv1.BareMetalPowerAction_BARE_METAL_POWER_ACTION_UNSPECIFIED,
			fmt.Errorf("invalid --action %q: want on, off, or reboot", raw)
	}
}

func parseBareMetalBootDevice(raw string) (baremetalv1.BareMetalBootDevice, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "disk":
		return baremetalv1.BareMetalBootDevice_BARE_METAL_BOOT_DEVICE_DISK, nil
	case "cdrom", "cd", "iso":
		return baremetalv1.BareMetalBootDevice_BARE_METAL_BOOT_DEVICE_CDROM, nil
	case "pxe", "net", "network":
		return baremetalv1.BareMetalBootDevice_BARE_METAL_BOOT_DEVICE_PXE, nil
	default:
		return baremetalv1.BareMetalBootDevice_BARE_METAL_BOOT_DEVICE_UNSPECIFIED,
			fmt.Errorf("invalid --device %q: want disk, cdrom, or pxe", raw)
	}
}
