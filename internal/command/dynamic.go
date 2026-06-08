package command

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
)

// newDynamicDebugCommand is a hidden command used to validate the dynamic
// Connect wire path against a live API:
//
//	metalhost x-dyn ComputeService ListVirtualMachines project_name=projects/p
//
// It builds the request from key=value string fields, calls the method purely
// from its descriptor, and renders the response with the normal output path.
func newDynamicDebugCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:    "x-dyn SERVICE METHOD [field=value ...]",
		Short:  "(debug) call any RPC dynamically from its descriptor",
		Hidden: true,
		Args:   cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cc, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			md, err := findMethod(args[0], args[1])
			if err != nil {
				return err
			}
			fields := map[string]string{}
			for _, kv := range args[2:] {
				if k, v, ok := strings.Cut(kv, "="); ok {
					fields[strings.TrimSpace(k)] = v
				}
			}
			req := newRequest(md, fields)
			msg, err := cc.dynamicCall(cmd.Context(), md, req)
			if err != nil {
				return err
			}
			return cc.write(msg)
		},
	}
}

// The dynamic engine calls any Metalhost RPC purely from its proto descriptor —
// no per-service Go. It speaks the Connect unary protocol over the SDK's
// authenticated HTTP client, marshaling requests/responses as proto-JSON into
// dynamicpb messages. This is what lets the resource registry be discovered
// from descriptors instead of hand-written.

// dynamicCall invokes the unary method described by md with the given request
// message, returning the response as a dynamicpb message (a proto.Message that
// the existing renderers already understand).
func (c *commandContext) dynamicCall(ctx context.Context, md protoreflect.MethodDescriptor, req proto.Message) (proto.Message, error) {
	cfg, err := c.sdkConfig()
	if err != nil {
		return nil, err
	}
	body, err := protojson.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	svc, ok := md.Parent().(protoreflect.ServiceDescriptor)
	if !ok {
		return nil, fmt.Errorf("method %s has no parent service", md.FullName())
	}
	url := cfg.BaseURL() + "/" + string(svc.FullName()) + "/" + string(md.Name())

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Connect-Protocol-Version", "1")

	httpResp, err := cfg.Client().Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, connectErrorFromJSON(httpResp.StatusCode, respBody)
	}

	out := dynamicpb.NewMessage(md.Output())
	if err := protojson.Unmarshal(respBody, out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return out, nil
}

// connectErrorFromJSON turns a Connect error body ({"code","message"}) into a
// *connect.Error so the central HandleError mapping (exit codes + hints) works.
func connectErrorFromJSON(status int, body []byte) error {
	var payload struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(body, &payload)
	msg := payload.Message
	if msg == "" {
		msg = strings.TrimSpace(string(body))
	}
	if msg == "" {
		msg = http.StatusText(status)
	}
	return connect.NewError(connectCodeFromString(payload.Code), fmt.Errorf("%s", msg))
}

func connectCodeFromString(s string) connect.Code {
	switch s {
	case "canceled":
		return connect.CodeCanceled
	case "invalid_argument":
		return connect.CodeInvalidArgument
	case "deadline_exceeded":
		return connect.CodeDeadlineExceeded
	case "not_found":
		return connect.CodeNotFound
	case "already_exists":
		return connect.CodeAlreadyExists
	case "permission_denied":
		return connect.CodePermissionDenied
	case "resource_exhausted":
		return connect.CodeResourceExhausted
	case "failed_precondition":
		return connect.CodeFailedPrecondition
	case "aborted":
		return connect.CodeAborted
	case "out_of_range":
		return connect.CodeOutOfRange
	case "unimplemented":
		return connect.CodeUnimplemented
	case "internal":
		return connect.CodeInternal
	case "unavailable":
		return connect.CodeUnavailable
	case "data_loss":
		return connect.CodeDataLoss
	case "unauthenticated":
		return connect.CodeUnauthenticated
	default:
		return connect.CodeUnknown
	}
}

// ── descriptor discovery ────────────────────────────────────────────────────

// findMethod locates a unary method descriptor by service+method name across
// all registered proto files (the SDK gen packages register themselves on
// import). serviceSuffix matches the end of the service full name, e.g.
// "ComputeService".
func findMethod(serviceSuffix, method string) (protoreflect.MethodDescriptor, error) {
	var found protoreflect.MethodDescriptor
	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		svcs := fd.Services()
		for i := 0; i < svcs.Len(); i++ {
			svc := svcs.Get(i)
			if !strings.HasSuffix(string(svc.FullName()), serviceSuffix) {
				continue
			}
			if m := svc.Methods().ByName(protoreflect.Name(method)); m != nil {
				found = m
				return false
			}
		}
		return true
	})
	if found == nil {
		return nil, fmt.Errorf("method %s/%s not found in registered descriptors", serviceSuffix, method)
	}
	return found, nil
}

// newRequest builds a dynamicpb request message for md's input, setting the
// named string fields (skipping any the input doesn't declare).
func newRequest(md protoreflect.MethodDescriptor, stringFields map[string]string) proto.Message {
	msg := dynamicpb.NewMessage(md.Input())
	fields := md.Input().Fields()
	for name, val := range stringFields {
		if val == "" {
			continue
		}
		fd := fields.ByName(protoreflect.Name(name))
		if fd == nil || fd.Kind() != protoreflect.StringKind {
			continue
		}
		msg.Set(fd, protoreflect.ValueOfString(val))
	}
	return msg
}
