package command

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	opsv1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/ops/v1"
)

// unaryMethod is the shape of every generated connect unary client method, e.g.
// ComputeServiceClient.ListVirtualMachines. Capturing it lets the helpers below
// drive any RPC generically.
type unaryMethod[Req, Resp any] func(context.Context, *connect.Request[Req]) (*connect.Response[Resp], error)

// invoke runs a unary RPC and returns the response message type-erased as a
// proto.Message (every connect response message is one).
func invoke[Req, Resp any](cmd *cobra.Command, method unaryMethod[Req, Resp], req *Req) (proto.Message, error) {
	resp, err := method(cmd.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return any(resp.Msg).(proto.Message), nil
}

// invokeList runs a list RPC, optionally following next_page_token to merge all
// pages into the first response (clearing the token so output looks complete).
func invokeList[Req, Resp any](cmd *cobra.Command, method unaryMethod[Req, Resp], req *Req, all bool) (proto.Message, error) {
	first, err := method(cmd.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	acc := any(first.Msg).(proto.Message)
	if all {
		reqMsg := any(req).(proto.Message)
		for token := nextPageToken(acc); token != ""; {
			if !setPageToken(reqMsg, token) {
				break
			}
			next, err := method(cmd.Context(), connect.NewRequest(req))
			if err != nil {
				return nil, err
			}
			appendListItems(acc, any(next.Msg).(proto.Message))
			token = nextPageToken(any(next.Msg).(proto.Message))
		}
		clearNextPageToken(acc)
	}
	return acc, nil
}

// do runs a unary RPC and writes the response in the active output format. It
// replaces the resp,err / if err / cc.write triplet repeated at every call site
// and routes output through commandContext.write (so --wait/-o/-q all apply).
func do[Req, Resp any](cmd *cobra.Command, cc *commandContext, method unaryMethod[Req, Resp], req *Req) error {
	msg, err := invoke(cmd, method, req)
	if err != nil {
		return err
	}
	return cc.write(msg)
}

// doList runs a list RPC. With --all it transparently follows next_page_token,
// merging every page's items into the first response and clearing the token so
// the rendered output looks like one complete result set.
func doList[Req, Resp any](cmd *cobra.Command, cc *commandContext, method unaryMethod[Req, Resp], req *Req, all bool) error {
	msg, err := invokeList(cmd, method, req, all)
	if err != nil {
		return err
	}
	return cc.write(msg)
}

// ── --wait: poll the returned operation to a terminal state ─────────────────

// maybeWait inspects a response for an operation and, when --wait is set, polls
// the Operations service until the operation reaches a terminal state (or the
// timeout elapses), returning the final operation to render in its place.
func (c *commandContext) maybeWait(msg proto.Message) (proto.Message, error) {
	if !c.root.wait {
		return msg, nil
	}
	name := operationName(msg)
	if name == "" {
		return msg, nil
	}
	client, err := c.opsClient()
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	var deadline time.Time
	if c.root.waitTimeout > 0 {
		deadline = time.Now().Add(c.root.waitTimeout)
	}
	for {
		resp, err := client.GetOperation(ctx, connect.NewRequest(&opsv1.GetOperationRequest{Name: name}))
		if err != nil {
			return nil, err
		}
		switch resp.Msg.GetOperation().GetState() {
		case opsv1.State_STATE_SUCCEEDED, opsv1.State_STATE_FAILED, opsv1.State_STATE_CANCELLED:
			return resp.Msg, nil
		}
		if !deadline.IsZero() && time.Now().After(deadline) {
			return resp.Msg, fmt.Errorf("timed out after %s waiting for %s", c.root.waitTimeout, name)
		}
		time.Sleep(2 * time.Second)
	}
}

// operationName returns the resource name of an operation embedded in a response
// (an `operation` sub-message) or of an Operation message itself, else "".
func operationName(msg proto.Message) string {
	m := msg.ProtoReflect()
	fields := m.Descriptor().Fields()
	if fd := fields.ByName("operation"); fd != nil && fd.Kind() == protoreflect.MessageKind && !fd.IsList() && m.Has(fd) {
		return stringSubField(m.Get(fd).Message(), "name")
	}
	// The message may itself be an Operation: name + a state enum field.
	if fields.ByName("name") != nil && fields.ByName("state") != nil {
		if fields.ByName("state").Kind() == protoreflect.EnumKind {
			return stringSubField(m, "name")
		}
	}
	return ""
}

func stringSubField(m protoreflect.Message, name string) string {
	fd := m.Descriptor().Fields().ByName(protoreflect.Name(name))
	if fd == nil || fd.Kind() != protoreflect.StringKind {
		return ""
	}
	return m.Get(fd).String()
}

// ── pagination reflection helpers ───────────────────────────────────────────

func nextPageToken(msg proto.Message) string {
	return stringSubField(msg.ProtoReflect(), "next_page_token")
}

func clearNextPageToken(msg proto.Message) {
	m := msg.ProtoReflect()
	if fd := m.Descriptor().Fields().ByName("next_page_token"); fd != nil {
		m.Clear(fd)
	}
}

func setPageToken(req proto.Message, token string) bool {
	m := req.ProtoReflect()
	fd := m.Descriptor().Fields().ByName("page_token")
	if fd == nil || fd.Kind() != protoreflect.StringKind {
		return false
	}
	m.Set(fd, protoreflect.ValueOfString(token))
	return true
}

// appendListItems appends the primary repeated-message field's items from src
// onto dst (used to merge paginated results).
func appendListItems(dst, src proto.Message) {
	dm, sm := dst.ProtoReflect(), src.ProtoReflect()
	fd := primaryRepeatedField(dm)
	if fd == nil {
		return
	}
	dstList := dm.Mutable(fd).List()
	srcList := sm.Get(fd).List()
	for i := 0; i < srcList.Len(); i++ {
		dstList.Append(srcList.Get(i))
	}
}

func primaryRepeatedField(m protoreflect.Message) protoreflect.FieldDescriptor {
	fields := m.Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		if fd.IsList() && fd.Kind() == protoreflect.MessageKind {
			return fd
		}
	}
	return nil
}
