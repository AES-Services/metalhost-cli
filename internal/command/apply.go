package command

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
	"gopkg.in/yaml.v3"
)

// apply is the declarative, kubectl-style write verb. It works for any
// resource-oriented kind — Create<Kind>/Update<Kind> RPCs that embed the
// resource message — discovered purely from proto descriptors via the dynamic
// engine. No per-kind Go.
//
// Manifest (YAML or JSON):
//
//	kind: Disk
//	parent: projects/myproj      # required for create; ignored on update
//	spec:                        # the resource message (proto-JSON / camelCase)
//	  displayName: data
//	  datacenterName: datacenters/us-dal-1
//	  sizeGib: 50
//	  storageClass: nvme
//
// create vs update is chosen by whether spec.name is set: a named resource is
// updated (mask = the spec fields you provided), an unnamed one is created
// under `parent`. Round-trips with `get <kind> <name> -o yaml`.
type applyManifest struct {
	Kind   string         `json:"kind" yaml:"kind"`
	Parent string         `json:"parent" yaml:"parent"`
	Spec   map[string]any `json:"spec" yaml:"spec"`
}

func newApplyCommand(opts *rootOptions) *cobra.Command {
	var file, kind, parent string
	cmd := &cobra.Command{
		Use:   "apply -f FILE",
		Short: "Create or update a resource from a declarative manifest",
		Long: "Declarative write verb. Two input shapes:\n\n" +
			"  • Envelope manifest — a doc with `kind`, optional `parent`, and a `spec` (the resource fields).\n" +
			"  • Bare resource — the resource fields at the top level, with `--kind` (and `--parent` for create) " +
			"supplied as flags. This is what `get <kind> <name> -o yaml` emits, so it round-trips directly.\n\n" +
			"If the resource has `name` set it is updated (only the fields present); otherwise it is created " +
			"under `parent` (falls back to --parent / --project / profile). Pass `-f -` to read from stdin.",
		Example: examples(`
  metalhost apply -f disk.yaml
  metalhost get disk projects/p/disks/data -o yaml | metalhost apply --kind disk -f -
  metalhost apply --kind disk --parent projects/p -f new-disk.yaml`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(file) == "" {
				return errors.New("a manifest is required: pass -f FILE (or -f - for stdin)")
			}
			raw, err := readManifest(cmd, file)
			if err != nil {
				return err
			}
			var man *applyManifest
			if strings.TrimSpace(kind) != "" {
				man, err = parseBareManifest(raw, kind, parent)
			} else {
				man, err = parseManifest(raw)
			}
			if err != nil {
				return err
			}
			cc, err := loadCommandContext(opts)
			if err != nil {
				return err
			}
			return applyResource(cmd, cc, man)
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "manifest file (YAML or JSON); use - for stdin")
	cmd.Flags().StringVar(&kind, "kind", "", "resource kind for a bare resource doc (e.g. disk); makes the whole file the spec")
	cmd.Flags().StringVar(&parent, "parent", "", "parent for create when using --kind (e.g. projects/p); ignored on update")
	_ = cmd.MarkFlagRequired("file")
	return cmd
}

func readManifest(cmd *cobra.Command, file string) ([]byte, error) {
	if file == "-" {
		return io.ReadAll(cmd.InOrStdin())
	}
	return os.ReadFile(file)
}

// parseBareManifest treats the whole document as the resource spec (no envelope), pairing it with
// a kind/parent supplied via flags. This is the shape `get <kind> <name> -o yaml` emits, so a
// `get ... | apply --kind <kind> -f -` pipeline round-trips.
func parseBareManifest(raw []byte, kind, parent string) (*applyManifest, error) {
	var spec map[string]any
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		return nil, fmt.Errorf("parse resource: %w", err)
	}
	if len(spec) == 0 {
		return nil, errors.New("resource document is empty")
	}
	// Tolerate an envelope accidentally passed with --kind: unwrap its spec.
	if inner, ok := spec["spec"].(map[string]any); ok && spec["kind"] != nil {
		spec = inner
	}
	return &applyManifest{Kind: kind, Parent: parent, Spec: spec}, nil
}

// parseManifest decodes YAML or JSON (YAML is a superset) into the envelope.
func parseManifest(raw []byte) (*applyManifest, error) {
	var man applyManifest
	if err := yaml.Unmarshal(raw, &man); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if strings.TrimSpace(man.Kind) == "" {
		return nil, errors.New("manifest is missing `kind`")
	}
	if man.Spec == nil {
		return nil, errors.New("manifest is missing `spec`")
	}
	return &man, nil
}

func applyResource(cmd *cobra.Command, cc *commandContext, man *applyManifest) error {
	resDesc, err := findMessageByName(man.Kind)
	if err != nil {
		return err
	}

	// Build the resource message from spec via proto-JSON so field names, enums,
	// and well-known types decode exactly like the wire format.
	specJSON, err := json.Marshal(man.Spec)
	if err != nil {
		return fmt.Errorf("encode spec: %w", err)
	}
	res := dynamicpb.NewMessage(resDesc)
	if err := protojson.Unmarshal(specJSON, res); err != nil {
		return fmt.Errorf("decode spec into %s: %w", resDesc.Name(), err)
	}

	nameFD := resDesc.Fields().ByName("name")
	hasName := nameFD != nil && strings.TrimSpace(res.Get(nameFD).String()) != ""

	var md protoreflect.MethodDescriptor
	var req proto.Message
	if hasName {
		md, req, err = buildUpdateRequest(resDesc, res)
	} else {
		parent := strings.TrimSpace(man.Parent)
		if parent == "" {
			if p, perr := requireProject(cc, ""); perr == nil {
				parent = p
			}
		}
		md, req, err = buildCreateRequest(resDesc, res, parent)
	}
	if err != nil {
		return err
	}

	out, err := cc.dynamicCall(cmd.Context(), md, req)
	if err != nil {
		return err
	}
	return cc.write(out)
}

// buildCreateRequest assembles a Create<Kind> request: the resource embedded in
// its field, plus parent. Pure (no network) so it is unit-testable.
func buildCreateRequest(resDesc protoreflect.MessageDescriptor, res proto.Message, parent string) (protoreflect.MethodDescriptor, proto.Message, error) {
	md, err := findMethodByName("Create" + string(resDesc.Name()))
	if err != nil {
		return nil, nil, fmt.Errorf("%s is not creatable: %w", resDesc.Name(), err)
	}
	in := md.Input()
	req := dynamicpb.NewMessage(in)

	resField := fieldOfMessageType(in, resDesc.FullName())
	if resField == nil {
		return nil, nil, fmt.Errorf("Create%s does not embed a %s field (not resource-oriented)", resDesc.Name(), resDesc.Name())
	}
	req.Set(resField, protoreflect.ValueOfMessage(res.ProtoReflect()))

	if pf := in.Fields().ByName("parent"); pf != nil && pf.Kind() == protoreflect.StringKind {
		if strings.TrimSpace(parent) == "" {
			return nil, nil, errors.New("create requires `parent` (set it in the manifest or pass --project / a profile default)")
		}
		req.Set(pf, protoreflect.ValueOfString(parent))
	}
	return md, req, nil
}

// buildUpdateRequest assembles an Update<Kind> request: the resource embedded in
// its field, plus an update mask listing exactly the fields the manifest set
// (minus the identifier). Mask paths are proto field names (snake_case), read
// off the descriptor so manifest JSON key casing is irrelevant; the server
// ignores any immutable/output-only paths. Pure (no network) for testability.
func buildUpdateRequest(resDesc protoreflect.MessageDescriptor, res proto.Message) (protoreflect.MethodDescriptor, proto.Message, error) {
	md, err := findMethodByName("Update" + string(resDesc.Name()))
	if err != nil {
		return nil, nil, fmt.Errorf("%s is not updatable: %w", resDesc.Name(), err)
	}
	in := md.Input()
	req := dynamicpb.NewMessage(in)

	resField := fieldOfMessageType(in, resDesc.FullName())
	if resField == nil {
		return nil, nil, fmt.Errorf("Update%s does not embed a %s field (not resource-oriented)", resDesc.Name(), resDesc.Name())
	}
	req.Set(resField, protoreflect.ValueOfMessage(res.ProtoReflect()))

	if mf := fieldMaskField(in); mf != nil {
		paths := setFieldNames(res, "name")
		mask := dynamicpb.NewMessage(mf.Message())
		pathsFD := mf.Message().Fields().ByName("paths")
		list := mask.NewField(pathsFD).List()
		for _, p := range paths {
			list.Append(protoreflect.ValueOfString(p))
		}
		mask.Set(pathsFD, protoreflect.ValueOfList(list))
		req.Set(mf, protoreflect.ValueOfMessage(mask))
	}
	return md, req, nil
}

// ── descriptor helpers ──────────────────────────────────────────────────────

// findMessageByName resolves a kind to a top-level message descriptor by simple
// name, case-insensitively ("disk" or "Disk" → message Disk).
func findMessageByName(kind string) (protoreflect.MessageDescriptor, error) {
	var found protoreflect.MessageDescriptor
	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		msgs := fd.Messages()
		for i := 0; i < msgs.Len(); i++ {
			m := msgs.Get(i)
			if strings.EqualFold(string(m.Name()), kind) {
				found = m
				return false
			}
		}
		return true
	})
	if found == nil {
		return nil, fmt.Errorf("unknown kind %q: no proto message with that name is registered", kind)
	}
	return found, nil
}

// findMethodByName locates a unary method by its simple name across all services.
func findMethodByName(method string) (protoreflect.MethodDescriptor, error) {
	var found protoreflect.MethodDescriptor
	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		svcs := fd.Services()
		for i := 0; i < svcs.Len(); i++ {
			if m := svcs.Get(i).Methods().ByName(protoreflect.Name(method)); m != nil {
				found = m
				return false
			}
		}
		return true
	})
	if found == nil {
		return nil, fmt.Errorf("no %s RPC is registered", method)
	}
	return found, nil
}

// fieldOfMessageType returns the first field in desc whose message type matches
// full (e.g. the `disk` field of CreateDiskRequest).
func fieldOfMessageType(desc protoreflect.MessageDescriptor, full protoreflect.FullName) protoreflect.FieldDescriptor {
	fields := desc.Fields()
	for i := 0; i < fields.Len(); i++ {
		f := fields.Get(i)
		if f.Kind() == protoreflect.MessageKind && !f.IsList() && !f.IsMap() && f.Message().FullName() == full {
			return f
		}
	}
	return nil
}

// fieldMaskField returns the google.protobuf.FieldMask field of desc, if any.
func fieldMaskField(desc protoreflect.MessageDescriptor) protoreflect.FieldDescriptor {
	fields := desc.Fields()
	for i := 0; i < fields.Len(); i++ {
		f := fields.Get(i)
		if f.Kind() == protoreflect.MessageKind && f.Message().FullName() == "google.protobuf.FieldMask" {
			return f
		}
	}
	return nil
}

// setFieldNames returns the proto field names that are populated on msg,
// excluding the given names, sorted for stable output.
func setFieldNames(msg proto.Message, exclude ...string) []string {
	skip := map[string]bool{}
	for _, e := range exclude {
		skip[e] = true
	}
	var out []string
	msg.ProtoReflect().Range(func(fd protoreflect.FieldDescriptor, _ protoreflect.Value) bool {
		if name := string(fd.Name()); !skip[name] {
			out = append(out, name)
		}
		return true
	})
	sort.Strings(out)
	return out
}
