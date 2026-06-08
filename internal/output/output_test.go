package output

import (
	"bytes"
	"strings"
	"testing"

	computev1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/compute/v1"
	opsv1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/ops/v1"
)

// TestJSONRendersProtoEnumsAsStrings guards against regressing to encoding/json,
// which serialises proto enums as their integer value (e.g. 2) instead of the
// canonical proto-JSON name (STATE_RUNNING). The dashboard and `vm apply` both
// speak proto-JSON, so the CLI must too.
func TestJSONRendersProtoEnumsAsStrings(t *testing.T) {
	msg := &opsv1.Operation{Name: "operations/abc", State: opsv1.State_STATE_RUNNING}
	var buf bytes.Buffer
	if err := Write(&buf, "json", msg); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "STATE_RUNNING") {
		t.Fatalf("json output should contain enum name STATE_RUNNING, got:\n%s", got)
	}
	if strings.Contains(got, `"state":2`) || strings.Contains(got, `"state": 2`) {
		t.Fatalf("json output leaked the raw enum integer:\n%s", got)
	}
}

// TestYAMLRendersProtoEnumsAsStrings is the YAML counterpart.
func TestYAMLRendersProtoEnumsAsStrings(t *testing.T) {
	msg := &opsv1.Operation{Name: "operations/abc", State: opsv1.State_STATE_SUCCEEDED}
	var buf bytes.Buffer
	if err := Write(&buf, "yaml", msg); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "STATE_SUCCEEDED") {
		t.Fatalf("yaml output should contain enum name STATE_SUCCEEDED, got:\n%s", got)
	}
}

func TestTableListRendersColumns(t *testing.T) {
	resp := &computev1.ListVirtualMachinesResponse{
		VirtualMachines: []*computev1.VirtualMachine{
			{Name: "projects/p/virtual-machines/web-1", State: "RUNNING", DatacenterName: "datacenters/us-dal-1", Vcpus: 4, RamGib: 16},
		},
	}
	var buf bytes.Buffer
	if err := Write(&buf, "table", resp); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	for _, want := range []string{"NAME", "STATE", "RUNNING", "web-1"} {
		if !strings.Contains(got, want) {
			t.Fatalf("table missing %q, got:\n%s", want, got)
		}
	}
	// Resource paths are shortened to their final segment in tables.
	if strings.Contains(got, "projects/p/virtual-machines/web-1") {
		t.Fatalf("table should shorten resource names, got:\n%s", got)
	}
}

func TestTableEmptyListIsFriendly(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, "table", &computev1.ListVirtualMachinesResponse{}); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); !strings.Contains(strings.ToLower(got), "no ") {
		t.Fatalf("empty list should be friendly, got: %q", got)
	}
}

func TestQuietPrintsFullNamesOnly(t *testing.T) {
	resp := &computev1.ListVirtualMachinesResponse{
		VirtualMachines: []*computev1.VirtualMachine{
			{Name: "projects/p/virtual-machines/web-1", State: "RUNNING"},
			{Name: "projects/p/virtual-machines/web-2", State: "STOPPED"},
		},
	}
	var buf bytes.Buffer
	if err := Write(&buf, "name", resp); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(buf.String())
	want := "projects/p/virtual-machines/web-1\nprojects/p/virtual-machines/web-2"
	if got != want {
		t.Fatalf("quiet output = %q, want %q", got, want)
	}
}

func TestTableSingleRecordKeyValue(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, "table", &computev1.VirtualMachine{Name: "projects/p/virtual-machines/web-1", State: "RUNNING"}); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "name:") || !strings.Contains(got, "state:") {
		t.Fatalf("record view should print key: value lines, got:\n%s", got)
	}
}
