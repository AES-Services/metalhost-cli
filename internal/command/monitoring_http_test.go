package command

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func isolatedMonitoringCLI(t *testing.T) []string {
	t.Helper()
	for _, key := range []string{"METALHOST_PROFILE", "METALHOST_API_KEY", "METALHOST_ENDPOINT", "METALHOST_PROJECT", "METALHOST_FORMAT"} {
		t.Setenv(key, "")
	}
	return []string{"--config", filepath.Join(t.TempDir(), "missing.yaml"), "--project", "projects/fixture", "--format", "json"}
}

func TestMonitoringConfigurationNeverIncludesCredential(t *testing.T) {
	args := isolatedMonitoringCLI(t)
	t.Setenv("METALHOST_API_KEY", "mha_do-not-include-in-config")
	for _, target := range []string{"grafana", "prometheus"} {
		c := NewRootCommand()
		var out bytes.Buffer
		c.SetOut(&out)
		c.SetErr(&out)
		c.SetArgs(append(args, "--endpoint", "https://api.example.invalid", "monitoring", "config", "--target", target))
		if err := c.Execute(); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out.String(), "mha_do-not-include-in-config") || !strings.Contains(out.String(), "https://api.example.invalid/v1/monitoring/projects/fixture/") {
			t.Fatal("config leaked key or lost scope")
		}
		if target == "prometheus" && (!strings.Contains(out.String(), "honor_timestamps: true") || strings.Count(out.String(), "credentials_file:") != 2) {
			t.Fatal("scrape/discovery auth or source timestamps missing")
		}
		if target == "grafana" && !strings.Contains(out.String(), "Bearer $METALHOST_METRICS_KEY") {
			t.Fatal("Grafana credential placeholder missing")
		}
	}
}

func TestHostedPromQLAuthenticatesAndRejectsRedirect(t *testing.T) {
	args := isolatedMonitoringCLI(t)
	t.Setenv("METALHOST_API_KEY", "mha_fixture-test-key")
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer mha_fixture-test-key" || r.Header.Get("X-Scope-OrgID") != "" {
			t.Error("wrong credential or caller-controlled tenant")
		}
		if r.URL.Query().Get("query") != "sum(metalhost_vm_observed_running)" {
			t.Error("query changed")
		}
		if calls == 1 {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
			return
		}
		http.Redirect(w, r, "https://example.invalid/no-token", http.StatusFound)
	}))
	defer server.Close()
	for run := 0; run < 2; run++ {
		c := NewRootCommand()
		var out bytes.Buffer
		c.SetOut(&out)
		c.SetErr(&out)
		c.SetArgs(append(args, "--endpoint", server.URL, "monitoring", "promql", "sum(metalhost_vm_observed_running)"))
		err := c.Execute()
		if run == 0 && (err != nil || !strings.Contains(out.String(), "success")) {
			t.Fatalf("query: %s %v", out.String(), err)
		}
		if run == 1 && (err == nil || !strings.Contains(err.Error(), "302")) {
			t.Fatalf("redirect not rejected: %v", err)
		}
	}
	if calls != 2 {
		t.Fatalf("requests %d", calls)
	}
}

func TestHostedPromQLRejectsPartialRangeBeforeNetwork(t *testing.T) {
	args := isolatedMonitoringCLI(t)
	c := NewRootCommand()
	c.SetArgs(append(args, "--endpoint", "https://example.invalid", "monitoring", "promql", "up", "--start", "2026-09-20T00:00:00Z"))
	if err := c.Execute(); err == nil || !strings.Contains(err.Error(), "range requires") {
		t.Fatalf("invalid range accepted: %v", err)
	}
}
