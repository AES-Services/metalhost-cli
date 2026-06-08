package command

import (
	"testing"

	computev1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/compute/v1"
	opsv1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/ops/v1"
)

func TestPageTokenReflectionHelpers(t *testing.T) {
	req := &computev1.ListVirtualMachinesRequest{}
	if !setPageToken(req, "tok-123") {
		t.Fatal("setPageToken returned false for a message with page_token")
	}
	if req.GetPageToken() != "tok-123" {
		t.Fatalf("page_token = %q, want tok-123", req.GetPageToken())
	}

	resp := &computev1.ListVirtualMachinesResponse{NextPageToken: "next-abc"}
	if got := nextPageToken(resp); got != "next-abc" {
		t.Fatalf("nextPageToken = %q, want next-abc", got)
	}
	clearNextPageToken(resp)
	if resp.GetNextPageToken() != "" {
		t.Fatalf("next_page_token not cleared: %q", resp.GetNextPageToken())
	}
}

func TestAppendListItemsMergesRepeatedField(t *testing.T) {
	dst := &computev1.ListVirtualMachinesResponse{
		VirtualMachines: []*computev1.VirtualMachine{{Name: "a"}},
	}
	src := &computev1.ListVirtualMachinesResponse{
		VirtualMachines: []*computev1.VirtualMachine{{Name: "b"}, {Name: "c"}},
	}
	appendListItems(dst, src)
	if len(dst.GetVirtualMachines()) != 3 {
		t.Fatalf("merged length = %d, want 3", len(dst.GetVirtualMachines()))
	}
	got := []string{
		dst.GetVirtualMachines()[0].GetName(),
		dst.GetVirtualMachines()[1].GetName(),
		dst.GetVirtualMachines()[2].GetName(),
	}
	want := []string{"a", "b", "c"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("merged[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestOperationNameFromEmbeddedOperation(t *testing.T) {
	resp := &computev1.CreateVirtualMachineResponse{
		Operation: &opsv1.Operation{Name: "operations/op-1"},
	}
	if got := operationName(resp); got != "operations/op-1" {
		t.Fatalf("operationName(embedded) = %q, want operations/op-1", got)
	}
}

func TestOperationNameFromOperationItself(t *testing.T) {
	op := &opsv1.Operation{Name: "operations/op-2", State: opsv1.State_STATE_RUNNING}
	if got := operationName(op); got != "operations/op-2" {
		t.Fatalf("operationName(operation) = %q, want operations/op-2", got)
	}
}

func TestOperationNameNoneForPlainList(t *testing.T) {
	resp := &computev1.ListVirtualMachinesResponse{NextPageToken: "x"}
	if got := operationName(resp); got != "" {
		t.Fatalf("operationName(list) = %q, want empty", got)
	}
}
