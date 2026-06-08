package command

import (
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// resolveOne fetches a single resource by name. It prefers a dedicated Get RPC;
// for kinds that only expose List (network, ssh-key, file-share) it transparently
// lists every page and filters by name, so `get KIND NAME` / `describe` work
// uniformly across all kinds.
func resolveOne(cmd *cobra.Command, cc *commandContext, h *resourceHandler, name string) (proto.Message, error) {
	if h.get != nil {
		msg, err := h.get(cmd, cc, name)
		if err != nil {
			return nil, err
		}
		return unwrapSingleResource(msg), nil
	}
	if h.list == nil {
		return nil, fmt.Errorf("`%s` cannot be fetched by name", h.kind)
	}
	return getViaList(cmd, cc, h, name)
}

// getViaList implements get-by-name for list-only kinds: page through the
// collection and return the element whose `name` matches.
func getViaList(cmd *cobra.Command, cc *commandContext, h *resourceHandler, name string) (proto.Message, error) {
	parent := ""
	if h.parent != nil {
		var err error
		if parent, err = h.parent(cc); err != nil {
			return nil, err
		}
	}
	msg, err := h.list(cmd, cc, parent, pageFlags{all: true})
	if err != nil {
		return nil, err
	}
	m := msg.ProtoReflect()
	fd := primaryRepeatedField(m)
	if fd == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("%s %q not found", h.kind, name))
	}
	items := m.Get(fd).List()
	for i := 0; i < items.Len(); i++ {
		item := items.Get(i).Message()
		if nf := item.Descriptor().Fields().ByName("name"); nf != nil && item.Get(nf).String() == name {
			return item.Interface(), nil
		}
	}
	return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("%s %q not found", h.kind, name))
}

// unwrapSingleResource turns a single-resource response envelope (e.g.
// GetDiskResponse{disk}) into the bare resource message it wraps, so
// `get KIND NAME -o yaml` prints the resource itself — which `apply` can then
// consume verbatim under `spec:`. Responses that aren't a single singular
// message field are returned unchanged.
func unwrapSingleResource(msg proto.Message) proto.Message {
	m := msg.ProtoReflect()
	fields := m.Descriptor().Fields()
	if fields.Len() != 1 {
		return msg
	}
	f := fields.Get(0)
	if f.Kind() != protoreflect.MessageKind || f.IsList() || f.IsMap() {
		return msg
	}
	return m.Get(f).Message().Interface()
}

// The unified, kubectl-style verbs (get/describe/delete) operate over the
// resource registry so the same grammar works for every kind:
//
//	metalhost get vm                      # list
//	metalhost get vm projects/p/.../web   # get one
//	metalhost describe disk projects/p/...# full detail
//	metalhost delete disk projects/p/...  # delete with confirm
//
// They sit alongside the per-service subtrees (`metalhost vm list`, …), which
// remain for command-specific flags and actions.

func unknownKindErr(kind string) error {
	return fmt.Errorf("unknown kind %q; known kinds: %s", kind, strings.Join(resourceKinds(), ", "))
}

func newGetCommand(opts *rootOptions) *cobra.Command {
	var pages pageFlags
	cmd := &cobra.Command{
		Use:               "get KIND [NAME]",
		Short:             "List resources of a kind, or get one by name",
		Long:              "Unified read verb. KIND is a resource type (vm, disk, network, project, …). With no NAME it lists; with a NAME it fetches that one resource.",
		Args:              cobra.RangeArgs(1, 2),
		ValidArgsFunction: verbCompletion(opts),
		Example: examples(`
  metalhost get vm
  metalhost get vm projects/p/virtual-machines/web-1
  metalhost get disk --all -o json
  metalhost get project --org organizations/acme`),
		RunE: func(cmd *cobra.Command, args []string) error {
			h, ok := lookupResource(args[0])
			if !ok {
				return unknownKindErr(args[0])
			}
			cc, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			if len(args) == 2 {
				msg, err := resolveOne(cmd, cc, h, args[1])
				if err != nil {
					return err
				}
				return cc.write(msg)
			}
			parent := ""
			if h.parent != nil {
				if parent, err = h.parent(cc); err != nil {
					return err
				}
			}
			msg, err := h.list(cmd, cc, parent, pages)
			if err != nil {
				return err
			}
			return cc.write(msg)
		},
	}
	addPageFlags(cmd, &pages)
	return cmd
}

func newDescribeCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:               "describe KIND NAME",
		Short:             "Show the full details of a single resource",
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: verbCompletion(opts),
		Example: examples(`
  metalhost describe vm projects/p/virtual-machines/web-1
  metalhost describe disk projects/p/disks/data -o yaml`),
		RunE: func(cmd *cobra.Command, args []string) error {
			h, ok := lookupResource(args[0])
			if !ok {
				return unknownKindErr(args[0])
			}
			cc, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			msg, err := resolveOne(cmd, cc, h, args[1])
			if err != nil {
				return err
			}
			return cc.write(msg)
		},
	}
}

func newDeleteCommand(opts *rootOptions) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:               "delete KIND NAME",
		Short:             "Delete a resource by name",
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: verbCompletion(opts),
		Example: examples(`
  metalhost delete vm projects/p/virtual-machines/web-1 --yes
  metalhost delete disk projects/p/disks/data-1`),
		RunE: func(cmd *cobra.Command, args []string) error {
			h, ok := lookupResource(args[0])
			if !ok {
				return unknownKindErr(args[0])
			}
			if h.remove == nil {
				return fmt.Errorf("`delete %s` is not supported (no Delete RPC)", h.kind)
			}
			cc, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			if err := confirmDestructive(cmd, yes, "delete "+h.title, args[1]); err != nil {
				return err
			}
			msg, err := h.remove(cmd, cc, args[1])
			if err != nil {
				return err
			}
			return writeDeleted(cmd, cc, h.kind, args[1], msg)
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the confirmation prompt")
	return cmd
}

// writeDeleted renders the result of a delete-style RPC consistently: a friendly confirmation
// for empty responses (DeleteXResponse{}), or the message itself when it carries an async
// Operation. Used by both the unified `delete` verb and the per-service `X delete` commands.
func writeDeleted(cmd *cobra.Command, cc *commandContext, what, name string, msg proto.Message) error {
	if isEmptyResponse(msg) {
		fmt.Fprintf(cmd.OutOrStdout(), "Deleted %s %s\n", what, name)
		return nil
	}
	return cc.write(msg)
}

// isEmptyResponse reports whether a proto message has no populated fields (e.g. DeleteXResponse{}).
func isEmptyResponse(msg proto.Message) bool {
	if msg == nil {
		return true
	}
	empty := true
	msg.ProtoReflect().Range(func(protoreflect.FieldDescriptor, protoreflect.Value) bool {
		empty = false
		return false
	})
	return empty
}

// verbCompletion completes the KIND for arg 0 and, where a name completer is
// registered, resource NAMEs for arg 1.
func verbCompletion(opts *rootOptions) completionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		switch len(args) {
		case 0:
			out := make([]string, 0)
			for _, k := range resourceKinds() {
				if toComplete == "" || strings.HasPrefix(k, toComplete) {
					out = append(out, k)
				}
			}
			return out, cobra.ShellCompDirectiveNoFileComp
		case 1:
			if h, ok := lookupResource(args[0]); ok && h.names != nil {
				return h.names(opts)(cmd, nil, toComplete)
			}
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}
