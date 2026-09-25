package command

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestObservabilityCommandsHaveRealPublicMethods(t *testing.T) {
	var inspect func(*cobra.Command)
	inspect = func(c *cobra.Command) {
		if service := c.Annotations["rpc-service"]; service != "" {
			if _, err := findMethod(service, c.Annotations["rpc-method"]); err != nil {
				t.Fatal(err)
			}
		}
		for _, child := range c.Commands() {
			inspect(child)
		}
	}
	inspect(NewRootCommand())
}

func TestScopedKeySecretUsesPrivateFileNotConsole(t *testing.T) {
	for _, env := range []string{"METALHOST_PROFILE", "METALHOST_API_KEY", "METALHOST_ENDPOINT", "METALHOST_PROJECT", "METALHOST_FORMAT"} {
		t.Setenv(env, "")
	}
	secret := "mha_one-time-secret"
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req["projectName"] != "projects/a" || req["requestId"] != "11111111-1111-4111-8111-111111111111" {
			t.Fatalf("lost scope or retry identity: %v", req)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"secret":"` + secret + `","credential":{"name":"automation-credentials/test"}}`))
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "key")
	args := []string{"--config", filepath.Join(t.TempDir(), "missing.yaml"), "--endpoint", server.URL, "--project", "projects/a", "--format", "json", "automation", "keys", "create", "--file", "-", "--secret-file", path}
	run := func() (string, error) {
		c := NewRootCommand()
		var out bytes.Buffer
		c.SetOut(&out)
		c.SetErr(&out)
		c.SetIn(strings.NewReader(`{"request_id":"11111111-1111-4111-8111-111111111111","display_name":"CLI","permissions":["monitoring.read"]}`))
		c.SetArgs(args)
		err := c.Execute()
		return out.String(), err
	}
	out, err := run()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, secret) || !strings.Contains(out, "automation-credentials/test") {
		t.Fatal("secret leaked or metadata missing")
	}
	body, err := os.ReadFile(path)
	if err != nil || string(body) != secret+"\n" {
		t.Fatal("secret not saved")
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("secret file is not private")
	}
	if _, err := run(); err == nil {
		t.Fatal("existing file overwritten")
	}
	if calls != 1 {
		t.Fatal("created an extra credential before rejecting existing secret file")
	}
}

func TestMonitoringMutationRequiresExplicitRequest(t *testing.T) {
	c := NewRootCommand()
	c.SetArgs([]string{"monitoring", "rules", "save"})
	if err := c.Execute(); err == nil || !strings.Contains(err.Error(), "--file is required") {
		t.Fatalf("unsafe empty mutation: %v", err)
	}
}

func TestFollowupObservabilityRequests(t *testing.T) {
	for _, tc := range []struct {
		name   string
		args   []string
		body   string
		method string
		field  string
		want   any
	}{
		{"pause", []string{"monitoring", "guest", "set-paused"}, `{"name":"virtual-machines/test","installation_id":"installation","paused":true,"request_id":"11111111-1111-4111-8111-111111111111"}`, "/aes.monitoring.v1.MonitoringService/SetEnhancedMonitoringPaused", "paused", true},
		{"resume", []string{"monitoring", "guest", "set-paused"}, `{"name":"virtual-machines/test","installation_id":"installation","paused":false,"request_id":"22222222-2222-4222-8222-222222222222"}`, "/aes.monitoring.v1.MonitoringService/SetEnhancedMonitoringPaused", "installationId", "installation"},
		{"project credentials", []string{"automation", "keys", "list"}, `{"all_project_credentials":true}`, "/aes.iam.v1.AutomationService/ListCredentials", "allProjectCredentials", true},
		{"chat destination", []string{"monitoring", "destinations", "save"}, `{"webhook_url":"https://example.invalid/private-test","request_id":"11111111-1111-4111-8111-111111111111"}`, "/aes.monitoring.v1.AlertService/SaveAlertDestination", "webhookUrl", "https://example.invalid/private-test"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := isolatedMonitoringCLI(t)
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path != tc.method {
					t.Errorf("unexpected RPC: %s", r.URL.Path)
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if body[tc.field] != tc.want {
					t.Errorf("lost %s", tc.field)
				}
				if tc.name == "pause" || tc.name == "resume" {
					var original map[string]any
					if err := json.Unmarshal([]byte(tc.body), &original); err != nil {
						t.Error(err)
					}
					if body["name"] != original["name"] || body["installationId"] != original["installation_id"] || body["requestId"] != original["request_id"] {
						t.Error("lost mutation identity")
					}
					if (body["paused"] == true) != (tc.name == "pause") {
						t.Error("wrong pause/resume state")
					}
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{}`))
			}))
			defer server.Close()
			c := NewRootCommand()
			var out bytes.Buffer
			c.SetOut(&out)
			c.SetErr(&out)
			c.SetIn(strings.NewReader(tc.body))
			args = append(args, "--endpoint", server.URL, "--project", "projects/a", "--format", "json")
			args = append(args, tc.args...)
			c.SetArgs(append(args, "--file", "-"))
			if err := c.Execute(); err != nil {
				t.Fatal(err)
			}
			if calls != 1 {
				t.Fatalf("expected one RPC, got %d", calls)
			}
			if strings.Contains(out.String(), "private-test") {
				t.Fatal("webhook URL leaked to console")
			}
		})
	}
}

func TestGuestEnrollmentDoesNotPerformPowerOperations(t *testing.T) {
	for _, env := range []string{"METALHOST_PROFILE", "METALHOST_API_KEY", "METALHOST_ENDPOINT", "METALHOST_PROJECT", "METALHOST_FORMAT"} {
		t.Setenv(env, "")
	}
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.Path)
		if r.URL.Path != "/aes.monitoring.v1.MonitoringService/EnableEnhancedMonitoring" {
			t.Errorf("unexpected power or other RPC: %s", r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["requestId"] != "11111111-1111-4111-8111-111111111111" || body["confirmDeviceAttachment"] != true || body["name"] != "virtual-machines/test" {
			t.Errorf("lost consent/identity: %v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"monitoring":{"name":"virtual-machines/test","status":"AWAITING_INSTALLATION"}}`))
	}))
	defer server.Close()
	c := NewRootCommand()
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetErr(&out)
	c.SetIn(strings.NewReader(`{"name":"virtual-machines/test","request_id":"11111111-1111-4111-8111-111111111111","confirm_device_attachment":true}`))
	c.SetArgs([]string{"--config", filepath.Join(t.TempDir(), "missing.yaml"), "--endpoint", server.URL, "--format", "json", "monitoring", "guest", "enable", "--file", "-"})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 {
		t.Fatalf("unexpected calls: %v", calls)
	}
}
