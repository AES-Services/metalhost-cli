package command

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	outputfmt "github.com/AES-Services/metalhost-cli/internal/output"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/AES-Services/metalhost-sdk/gen/go/aes/monitoring/v1"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

func newAutomationCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "automation", Short: "Scoped keys, service accounts, and GitHub trust policies"}
	cmd.AddCommand(observabilityRPC(opts, "capabilities", "aes.iam.v1.AutomationService", "ListCapabilities", false, false))
	for _, group := range []struct {
		name    string
		methods [][2]string
	}{
		{"accounts", [][2]string{{"list", "ListServiceAccounts"}, {"get", "GetServiceAccount"}, {"create", "CreateServiceAccount"}, {"update", "UpdateServiceAccount"}, {"state", "SetServiceAccountState"}, {"delete", "DeleteServiceAccount"}}},
		{"keys", [][2]string{{"list", "ListCredentials"}, {"create", "CreateCredential"}, {"rotate", "RotateCredential"}, {"revoke", "RevokeCredential"}}},
		{"github", [][2]string{{"list", "ListGitHubTrusts"}, {"create", "CreateGitHubTrust"}, {"update", "UpdateGitHubTrust"}, {"state", "SetGitHubTrustState"}, {"delete", "DeleteGitHubTrust"}}},
	} {
		sub := &cobra.Command{Use: group.name, Short: "Manage " + group.name + " using the public API contract"}
		for _, method := range group.methods {
			secret := group.name == "keys" && (method[0] == "create" || method[0] == "rotate")
			sub.AddCommand(observabilityRPC(opts, method[0], "aes.iam.v1.AutomationService", method[1], method[0] != "list" && method[0] != "get", secret))
		}
		cmd.AddCommand(sub)
	}
	return cmd
}

func newMonitoringCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "monitoring", Short: "Project metric capabilities, samples, rules, incidents, and destinations"}
	cmd.AddCommand(newMonitoringConfigCommand(opts), newPromQLCommand(opts))
	for _, method := range [][2]string{{"catalog", "ListMetricDescriptors"}, {"vms", "ListVMMonitoring"}, {"query", "QueryVMMonitoring"}} {
		cmd.AddCommand(observabilityRPC(opts, method[0], "aes.monitoring.v1.MonitoringService", method[1], false, false))
	}
	guest := &cobra.Command{Use: "guest", Short: "Optional guest monitoring enrollment (never automatically stops or starts a VM)", Long: "Inspect status first. Existing VMs without the identity device require an explicitly approved stop/start: stop using the compute command, wait for completion, enable with confirm_device_attachment=true, then explicitly start the VM. Prepared new VMs need no restart. Enrollment returns public installation configuration, not an API key. Revocation blocks ingestion but does not uninstall guest software."}
	for _, method := range [][2]string{{"status", "GetEnhancedMonitoring"}, {"enable", "EnableEnhancedMonitoring"}, {"set-paused", "SetEnhancedMonitoringPaused"}, {"revoke", "RevokeEnhancedMonitoring"}} {
		guest.AddCommand(observabilityRPC(opts, method[0], "aes.monitoring.v1.MonitoringService", method[1], method[0] != "status", false))
	}
	cmd.AddCommand(guest)
	for _, group := range []struct {
		name    string
		methods [][2]string
	}{
		{"rules", [][2]string{{"list", "ListAlertRules"}, {"save", "SaveAlertRule"}, {"delete", "DeleteAlertRule"}, {"preview", "PreviewAlertRule"}, {"instances", "ListAlertInstances"}}},
		{"incidents", [][2]string{{"list", "ListIncidents"}, {"summary", "GetIncidentSummary"}, {"get", "GetIncident"}, {"update", "UpdateIncident"}}},
		{"destinations", [][2]string{{"list", "ListAlertDestinations"}, {"candidates", "ListAlertDestinationCandidates"}, {"save", "SaveAlertDestination"}, {"delete", "DeleteAlertDestination"}, {"request-verification", "RequestDestinationVerification"}, {"verify", "VerifyAlertDestination"}, {"test", "TestAlertDestination"}, {"test-status", "GetDestinationTest"}}},
	} {
		sub := &cobra.Command{Use: group.name, Short: "Manage monitoring " + group.name}
		for _, method := range group.methods {
			write := !strings.HasPrefix(method[1], "List") && !strings.HasPrefix(method[1], "Get") && method[0] != "preview"
			sub.AddCommand(observabilityRPC(opts, method[0], "aes.monitoring.v1.AlertService", method[1], write, false))
		}
		cmd.AddCommand(sub)
	}
	return cmd
}

func observabilityRPC(opts *rootOptions, use, service, method string, write, secret bool) *cobra.Command {
	var file, secretFile string
	cmd := &cobra.Command{Use: use, Short: method + " (public JSON request)", Long: method + " accepts the SDK protobuf JSON request via --file PATH or --file - (stdin). Existing resources use names from list responses. --project fills a missing top-level project_name. Preserve request_id, resource ID and expected_version on retries. Unknown fields are rejected. No mutation is retried automatically.", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if write && file == "" {
			return errors.New("--file is required for a mutation; use a reviewed JSON request with stable retry IDs")
		}
		if secret && secretFile == "" {
			return errors.New("--secret-file is required; secrets are never printed to the terminal")
		}
		cc, err := loadCommandContext(opts)
		if err != nil {
			return err
		}
		md, err := findMethod(service, method)
		if err != nil {
			return err
		}
		input := dynamicpb.NewMessage(md.Input())
		if file != "" {
			var r io.Reader = cmd.InOrStdin()
			if file != "-" {
				f, err := os.Open(file)
				if err != nil {
					return err
				}
				defer f.Close()
				r = f
			}
			raw, err := io.ReadAll(io.LimitReader(r, (1<<20)+1))
			if err != nil {
				return err
			}
			if len(raw) > 1<<20 {
				return errors.New("request exceeds 1 MiB")
			}
			if err := protojson.Unmarshal(raw, input); err != nil {
				return errors.New("invalid request JSON; check field names and types against the SDK contract")
			}
		}
		if field := md.Input().Fields().ByName("project_name"); field != nil && input.Get(field).String() == "" {
			input.Set(field, protoreflect.ValueOfString(cc.profile.Project))
		}
		var outFile *os.File
		if secret {
			outFile, err = os.OpenFile(secretFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if err != nil {
				return fmt.Errorf("create secret file without overwriting: %w", err)
			}
			defer outFile.Close()
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
		defer cancel()
		output, err := callObservability(ctx, cc, md, input)
		if err != nil {
			if outFile != nil {
				_ = outFile.Close()
				if removeErr := os.Remove(secretFile); removeErr != nil {
					fmt.Fprintln(cmd.ErrOrStderr(), "Empty secret file could not be removed; inspect it before retrying.")
				}
			}
			return err
		}
		if outFile != nil {
			field := output.Descriptor().Fields().ByName("secret")
			value := output.Get(field).String()
			output.Clear(field)
			if value == "" {
				_ = outFile.Close()
				if err := os.Remove(secretFile); err != nil {
					return err
				}
				if err := outputfmt.Write(cmd.OutOrStdout(), cc.profile.Format, output); err != nil {
					return err
				}
				return errors.New("the request already completed but its one-time secret is unavailable; inspect the returned resource and revoke the credential or rotate the webhook with zero overlap before creating another")
			}
			if _, err := io.WriteString(outFile, value+"\n"); err != nil {
				_ = outputfmt.Write(cmd.OutOrStdout(), cc.profile.Format, output)
				return fmt.Errorf("credential created but saving its secret failed; revoke the credential: %w", err)
			}
			if err := outFile.Sync(); err != nil {
				_ = outputfmt.Write(cmd.OutOrStdout(), cc.profile.Format, output)
				return fmt.Errorf("credential created but secret file sync failed: %w", err)
			}
			fmt.Fprintln(cmd.ErrOrStderr(), "Secret saved to the requested private file. Keep it out of source control and CI artifacts.")
		}
		return outputfmt.Write(cmd.OutOrStdout(), cc.profile.Format, output)
	}}
	cmd.Flags().StringVarP(&file, "file", "f", "", "protobuf JSON request file, or - for stdin")
	cmd.Annotations = map[string]string{"rpc-service": service, "rpc-method": method}
	if secret {
		cmd.Flags().StringVar(&secretFile, "secret-file", "", "new private output file for the one-time secret (never overwritten)")
	}
	return cmd
}

func callObservability(ctx context.Context, cc *commandContext, method protoreflect.MethodDescriptor, input *dynamicpb.Message) (*dynamicpb.Message, error) {
	cfg, err := cc.sdkConfig()
	if err != nil {
		return nil, err
	}
	raw, err := protojson.Marshal(input)
	if err != nil {
		return nil, err
	}
	service := method.Parent().(protoreflect.ServiceDescriptor)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.BaseURL()+"/"+string(service.FullName())+"/"+string(method.Name()), bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connect-Protocol-Version", "1")
	client := *cfg.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.New("request failed; for a mutation, retry the exact request ID before starting another")
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, (4<<20)+1))
	if err != nil {
		return nil, errors.New("response interrupted; mutation status is unconfirmed")
	}
	if len(body) > 4<<20 {
		return nil, errors.New("response exceeds 4 MiB; narrow the query or page size")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, connectErrorFromJSON(resp.StatusCode, body)
	}
	out := dynamicpb.NewMessage(method.Output())
	if err := protojson.Unmarshal(body, out); err != nil {
		return nil, errors.New("invalid response; mutation status is unconfirmed")
	}
	return out, nil
}
