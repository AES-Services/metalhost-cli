package command

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protowire"
)

func errorInfoDetail(t *testing.T, reason, requestID string) []byte {
	t.Helper()
	var b []byte
	b = protowire.AppendTag(b, 1, protowire.BytesType)
	b = protowire.AppendString(b, reason)
	b = protowire.AppendTag(b, 2, protowire.BytesType)
	b = protowire.AppendString(b, errorDomain)
	var entry []byte
	entry = protowire.AppendTag(entry, 1, protowire.BytesType)
	entry = protowire.AppendString(entry, "request_id")
	entry = protowire.AppendTag(entry, 2, protowire.BytesType)
	entry = protowire.AppendString(entry, requestID)
	b = protowire.AppendTag(b, 3, protowire.BytesType)
	b = protowire.AppendBytes(b, entry)
	return b
}

func TestDecodeErrorInfo(t *testing.T) {
	info, ok := decodeErrorInfo(errorInfoDetail(t, "QUOTA_EXCEEDED", "req-1"))
	if !ok || info.Reason != "QUOTA_EXCEEDED" || info.RequestID != "req-1" {
		t.Fatalf("info=%+v ok=%v", info, ok)
	}
	if _, ok := decodeErrorInfo([]byte{0xff, 0xff}); ok {
		t.Fatal("garbage accepted")
	}
}

func TestHandleErrorShowsReasonAndRequestID(t *testing.T) {
	body := []byte(`{"code":"resource_exhausted","message":"You've reached a quota limit.","details":[{"type":"google.rpc.ErrorInfo","value":"x","debug":{"reason":"QUOTA_EXCEEDED","domain":"metalhost.net","metadata":{"request_id":"7f6c2a1e"}}}]}`)
	var buf bytes.Buffer
	code := HandleError(&buf, connectErrorFromJSON(429, body))
	out := buf.String()
	for _, want := range []string{"(QUOTA_EXCEEDED)", "metalhost quota request", "Request ID: 7f6c2a1e"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output %q missing %q", out, want)
		}
	}
	if code != exitCodeFor(connect.CodeResourceExhausted) {
		t.Fatalf("exit = %d", code)
	}
	// A foreign domain is ignored; the code label stays.
	buf.Reset()
	foreign := []byte(`{"code":"internal","message":"boom","details":[{"type":"google.rpc.ErrorInfo","debug":{"reason":"X","domain":"example.com"}}]}`)
	HandleError(&buf, connectErrorFromJSON(500, foreign))
	if strings.Contains(buf.String(), "Request ID") || !strings.Contains(buf.String(), "(internal)") {
		t.Fatalf("foreign domain: %q", buf.String())
	}
}

func TestParsePortMappingsForwarding(t *testing.T) {
	got, err := parsePortMappings([]string{"2222:22/tcp", "80", "30000-32767/udp"})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].GetPort() != 2222 || got[0].GetTargetPort() != 22 || got[0].GetProtocol() != "tcp" {
		t.Fatalf("forward = %v", got[0])
	}
	if got[1].GetTargetPort() != 0 || got[2].GetEndPort() != 32767 {
		t.Fatalf("plain = %v %v", got[1], got[2])
	}
	if _, err := parsePortMappings([]string{"2000-2010:22"}); err == nil {
		t.Fatal("forwarded range accepted")
	}
}

func TestNewRequestIDIsUUIDv4(t *testing.T) {
	re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if id := newRequestID(); !re.MatchString(id) {
		t.Fatalf("id %q", id)
	}
}

// Every October command resolves to a registered RPC descriptor.
func TestOctoberCommandsResolve(t *testing.T) {
	root := NewRootCommand()
	for _, path := range [][]string{
		{"vpc", "subnet", "attach"}, {"public-ip", "reserve"}, {"load-balancer", "create"},
		{"security-group", "add-vm"}, {"vm", "network", "internet-policy", "set"},
		{"vm", "network", "alias", "add"}, {"vm", "quote-resize"}, {"disk", "quote-resize"},
		{"file-share", "get"}, {"quota", "request"}, {"project", "members", "set"},
		{"wallet", "spend-by-project"}, {"webhook", "rotate-secret"},
		{"iam", "mfa", "regenerate-recovery-codes"}, {"automation", "github", "setup-begin"},
		{"monitoring", "guest", "install-collector"}, {"org", "check-slug"},
	} {
		cmd := findCommand(root, path...)
		if cmd == nil {
			t.Fatalf("missing command %v", path)
		}
		if _, err := findMethod(cmd.Annotations["rpc-service"], cmd.Annotations["rpc-method"]); err != nil {
			t.Fatalf("%v: %v", path, err)
		}
	}
	if f := findCommand(root, "network", "create").Flags(); f.Lookup("subnet") == nil || f.Lookup("subnet-cidr-v4") != nil || f.Lookup("id") != nil {
		t.Fatal("network create still has legacy flags")
	}
}
