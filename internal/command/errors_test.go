package command

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"connectrpc.com/connect"
)

func TestHandleErrorMapsConnectCodes(t *testing.T) {
	cases := []struct {
		code     connect.Code
		wantExit int
		wantText string
	}{
		{connect.CodeUnauthenticated, 4, "auth login"},
		{connect.CodePermissionDenied, 3, "permission"},
		{connect.CodeNotFound, 5, "not found"},
		{connect.CodeUnavailable, 6, "unreachable"},
		{connect.CodeAlreadyExists, 9, "already exists"},
		{connect.CodeInvalidArgument, 2, "rejected"},
	}
	for _, tc := range cases {
		var buf bytes.Buffer
		err := connect.NewError(tc.code, fmt.Errorf("boom"))
		got := HandleError(&buf, err)
		if got != tc.wantExit {
			t.Fatalf("%s: exit = %d, want %d", tc.code, got, tc.wantExit)
		}
		if !strings.Contains(strings.ToLower(buf.String()), tc.wantText) {
			t.Fatalf("%s: output %q missing %q", tc.code, buf.String(), tc.wantText)
		}
	}
}

func TestHandleErrorPlainError(t *testing.T) {
	var buf bytes.Buffer
	if got := HandleError(&buf, errors.New("nope")); got != 1 {
		t.Fatalf("exit = %d, want 1", got)
	}
	if !strings.Contains(buf.String(), "nope") {
		t.Fatalf("output = %q", buf.String())
	}
}

func TestHandleErrorNil(t *testing.T) {
	var buf bytes.Buffer
	if got := HandleError(&buf, nil); got != 0 {
		t.Fatalf("exit = %d, want 0", got)
	}
}
