package command

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/types/dynamicpb"
)

// runPublicIPCommand runs `public-ip <args>` against a server that answers
// every request with response.
func runPublicIPCommand(t *testing.T, response string, args ...string) (string, error) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()
	c := NewRootCommand()
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetErr(&out)
	c.SetIn(strings.NewReader(`{"request_id":"11111111-1111-4111-8111-111111111111"}`))
	c.SetArgs(append(append(isolatedMonitoringCLI(t), "--endpoint", server.URL, "public-ip"), args...))
	err := c.Execute()
	return out.String(), err
}

// A server newer than this build's SDK adds fields; reads must still decode.
func TestResponseWithUnknownFieldsDecodes(t *testing.T) {
	response := `{
		"publicIps": [{
			"name": "public-ips/a",
			"ipAddress": "203.0.113.10",
			"reserved": true,
			"futureField": {"nested": [1, 2]},
			"binding": {"targetVm": "virtual-machines/vm", "futureMode": "FUTURE_MODE_SHARED"}
		}],
		"futureTopLevel": "x"
	}`
	out, err := runPublicIPCommand(t, response, "list")
	if err != nil {
		t.Fatalf("list failed on a response with unknown fields: %v", err)
	}
	if !strings.Contains(out, "public-ips/a") || !strings.Contains(out, "203.0.113.10") {
		t.Fatalf("known fields missing from output: %s", out)
	}
	if strings.Contains(out, "futureField") || strings.Contains(out, "futureTopLevel") {
		t.Fatalf("unknown fields should be dropped, got: %s", out)
	}
}

func TestDynamicCallIgnoresUnknownResponseFields(t *testing.T) {
	md, err := findMethod(networkService, "ListPublicIps")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"publicIps":[{"name":"public-ips/a","futureField":1}],"futureTopLevel":"x"}`))
	}))
	defer server.Close()
	isolatedMonitoringCLI(t)
	cc, err := loadCommandContext(&rootOptions{configPath: filepath.Join(t.TempDir(), "missing.yaml"), endpoint: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cc.dynamicCall(t.Context(), md, dynamicpb.NewMessage(md.Input())); err != nil {
		t.Fatalf("dynamicCall failed on a response with unknown fields: %v", err)
	}
}

// An undecodable response to a read is a decode failure, not an unconfirmed
// mutation; for a mutation it still says the outcome is unconfirmed.
func TestUndecodableResponseMessage(t *testing.T) {
	const bad = `{"publicIps":"not-a-list","publicIp":7}`
	_, err := runPublicIPCommand(t, bad, "list")
	if err == nil || !strings.Contains(err.Error(), "could not decode the server response") || strings.Contains(err.Error(), "mutation") {
		t.Fatalf("read: want a neutral decode error, got %v", err)
	}
	_, err = runPublicIPCommand(t, bad, "reserve", "--file", "-")
	if err == nil || !strings.Contains(err.Error(), "could not decode the server response") || !strings.Contains(err.Error(), "mutation status is unconfirmed") {
		t.Fatalf("mutation: want an unconfirmed-mutation decode error, got %v", err)
	}
}
