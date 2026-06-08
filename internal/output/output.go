package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v3"
)

// Write renders value to w in the requested format. API responses are protobuf
// messages, so json/yaml route through protojson to preserve proto semantics
// (enum names instead of integers, lowerCamelCase field names, oneofs,
// well-known types). Falling back to encoding/json or yaml.v3 directly would
// emit raw enum integers and mangled Go field names — diverging from the
// proto-JSON the dashboard and `vm apply` speak.
func Write(w io.Writer, format string, value any) error {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "table":
		return renderTable(w, value)
	case "json":
		return writeJSON(w, value)
	case "yaml":
		return writeYAML(w, value)
	case "name", "quiet":
		return renderNames(w, value)
	default:
		return fmt.Errorf("unknown output format %q", format)
	}
}

func writeJSON(w io.Writer, value any) error {
	if msg, ok := value.(proto.Message); ok {
		b, err := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(msg)
		if err != nil {
			return err
		}
		_, err = w.Write(append(b, '\n'))
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func writeYAML(w io.Writer, value any) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	defer enc.Close()
	if msg, ok := value.(proto.Message); ok {
		// Marshal to proto-JSON first, then re-decode into a generic structure so
		// the YAML keeps proto field names and enum strings. yaml.v3 parses the
		// JSON bytes (JSON is valid YAML) into the intermediate value.
		b, err := protojson.Marshal(msg)
		if err != nil {
			return err
		}
		var intermediate any
		if err := yaml.Unmarshal(b, &intermediate); err != nil {
			return err
		}
		return enc.Encode(intermediate)
	}
	return enc.Encode(value)
}
