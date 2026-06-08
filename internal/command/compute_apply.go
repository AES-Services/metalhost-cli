package command

import (
	"cmp"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"gopkg.in/yaml.v3"

	computev1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/compute/v1"
)

// vmApplyCommand implements `metalhost vm apply -f vm.yaml` — create a VM from a declarative
// VirtualMachineManifest file (the same schema the dashboard sends). The manifest is the public
// contract; this is just yaml/json → the proto message → CreateVirtualMachine.
func vmApplyCommand(opts *rootOptions) *cobra.Command {
	var file, project, region string
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Create a VM from a declarative manifest file (-f vm.yaml)",
		Long: "Create a VM from a VirtualMachineManifest (compute.metalhost.io/v1) in YAML or JSON.\n" +
			"metadata.project and spec.region fall back to the active profile (or --project/--region) when omitted.\n\n" +
			"Example:\n  metalhost vm apply -f vm.yaml",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(file) == "" {
				return fmt.Errorf("-f/--file is required")
			}
			data, err := readManifestInput(file)
			if err != nil {
				return err
			}
			manifest, err := parseVMManifest(data)
			if err != nil {
				return err
			}
			ctx, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			if manifest.Metadata == nil {
				manifest.Metadata = &computev1.VirtualMachineMetadata{}
			}
			if manifest.Spec == nil {
				manifest.Spec = &computev1.VirtualMachineSpec{}
			}
			// Fill project/region from --flag, then the active profile, when the manifest omits them.
			if strings.TrimSpace(manifest.Metadata.GetProject()) == "" {
				if p := cmp.Or(strings.TrimSpace(project), strings.TrimSpace(ctx.profile.Project)); p != "" {
					manifest.Metadata.Project = p
				}
			}
			if strings.TrimSpace(manifest.Spec.GetRegion()) == "" {
				if r := cmp.Or(strings.TrimSpace(region), strings.TrimSpace(ctx.profile.Region)); r != "" {
					manifest.Spec.Region = r
				}
			}
			client, err := ctx.computeClient()
			if err != nil {
				return err
			}
			resp, err := client.CreateVirtualMachine(cmd.Context(),
				connect.NewRequest(&computev1.CreateVirtualMachineRequest{Manifest: manifest}))
			if err != nil {
				return err
			}
			return ctx.write(resp.Msg)
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "manifest file (YAML or JSON; '-' for stdin)")
	cmd.Flags().StringVar(&project, "project", "", "override/fill metadata.project")
	cmd.Flags().StringVar(&region, "region", "", "override/fill spec.region")
	return cmd
}

// readManifestInput reads the manifest from a path, or stdin when path is "-".
func readManifestInput(path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(os.Stdin)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest %q: %w", path, err)
	}
	return data, nil
}

// parseVMManifest decodes a YAML or JSON VirtualMachineManifest. YAML is converted to JSON and fed
// to protojson so proto field semantics (enum names, camelCase json names) apply uniformly.
func parseVMManifest(data []byte) (*computev1.VirtualMachineManifest, error) {
	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	j, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	var m computev1.VirtualMachineManifest
	if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(j, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	return &m, nil
}
