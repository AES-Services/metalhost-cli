package output

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode"

	"golang.org/x/term"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// displayWidth returns the number of terminal columns a string occupies. It counts runes
// (so multibyte UTF-8 like "·" counts as one) and skips zero-width combining marks. Wide
// CJK/emoji runes are not double-counted here — that's a rare case for our resource data.
func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		w++
	}
	return w
}

// renderTable turns an API response into a human-readable table. List responses
// (a single repeated message field, e.g. virtual_machines) render as a columnar
// table; single resources render as an aligned key/value block. Everything is
// driven by protoreflect so new resources work without bespoke code.
func renderTable(w io.Writer, value any) error {
	msg, ok := value.(proto.Message)
	if !ok {
		_, err := fmt.Fprintln(w, value)
		return err
	}
	m := msg.ProtoReflect()
	color := colorEnabled(w)

	// A response that bundles a primary resource (singular message field) with one or more
	// sub-collections (repeated message fields) — e.g. GetTicketResponse{ticket, messages} —
	// renders as the record plus titled sub-tables, not as a bare list of the sub-collection.
	if subs := populatedRepeatedMessageFields(m); len(subs) > 0 && hasSingularMessageField(m) {
		return renderComposite(w, m, subs, color)
	}
	if listFD, ok := primaryListField(m); ok {
		return renderList(w, m, listFD, color)
	}
	return renderRecord(w, m, color)
}

// hasSingularMessageField reports whether m has a non-repeated message field (the "primary
// resource" in a get-one response envelope).
func hasSingularMessageField(m protoreflect.Message) bool {
	fields := m.Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		if fd.Kind() == protoreflect.MessageKind && !fd.IsList() && !fd.IsMap() && m.Has(fd) {
			return true
		}
	}
	return false
}

// populatedRepeatedMessageFields returns the non-empty repeated message fields of m, in
// declaration order.
func populatedRepeatedMessageFields(m protoreflect.Message) []protoreflect.FieldDescriptor {
	var out []protoreflect.FieldDescriptor
	fields := m.Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		if fd.IsList() && fd.Kind() == protoreflect.MessageKind && m.Get(fd).List().Len() > 0 {
			out = append(out, fd)
		}
	}
	return out
}

// renderComposite prints the scalar/singular-message fields of m as a record, then each
// repeated-message sub-collection as its own titled table.
func renderComposite(w io.Writer, m protoreflect.Message, subs []protoreflect.FieldDescriptor, color bool) error {
	var pairs [][2]string
	fields := m.Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		if !m.Has(fd) {
			continue
		}
		if fd.IsList() && fd.Kind() == protoreflect.MessageKind {
			continue // rendered as a sub-table below
		}
		pairs = append(pairs, flattenField(m, fd, "")...)
	}
	if err := printPairs(w, pairs, color); err != nil {
		return err
	}
	for _, fd := range subs {
		if _, err := fmt.Fprintf(w, "\n%s:\n", strings.ToUpper(spaceWords(string(fd.Name())))); err != nil {
			return err
		}
		if err := renderItemsTable(w, fd.Message(), m.Get(fd).List(), color); err != nil {
			return err
		}
	}
	return nil
}

// primaryListField finds the repeated message field that represents the page of
// results (list RPCs have exactly one). When several exist we prefer a populated
// one so an empty optional repeated field doesn't win.
func primaryListField(m protoreflect.Message) (protoreflect.FieldDescriptor, bool) {
	fields := m.Descriptor().Fields()
	var candidate protoreflect.FieldDescriptor
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		if fd.IsList() && fd.Kind() == protoreflect.MessageKind {
			if m.Get(fd).List().Len() > 0 {
				return fd, true
			}
			if candidate == nil {
				candidate = fd
			}
		}
	}
	if candidate != nil {
		return candidate, true
	}
	return nil, false
}

func renderList(w io.Writer, m protoreflect.Message, listFD protoreflect.FieldDescriptor, color bool) error {
	list := m.Get(listFD).List()
	if list.Len() == 0 {
		_, err := fmt.Fprintf(w, "No %s found.\n", spaceWords(string(listFD.Name())))
		return err
	}

	if err := renderItemsTable(w, listFD.Message(), list, color); err != nil {
		return err
	}

	if tok := stringField(m, "next_page_token"); tok != "" {
		fmt.Fprintf(w, "\nMore results available — re-run with --page-token %s (or --all).\n", tok)
	}
	return nil
}

// renderItemsTable renders a repeated message field as an aligned table.
func renderItemsTable(w io.Writer, itemDesc protoreflect.MessageDescriptor, list protoreflect.List, color bool) error {
	cols := selectColumns(itemDesc)

	rows := make([][]string, 0, list.Len())
	for i := 0; i < list.Len(); i++ {
		item := list.Get(i).Message()
		row := make([]string, len(cols))
		for c, fd := range cols {
			row[c] = formatValue(item, fd)
		}
		rows = append(rows, row)
	}

	headers := make([]string, len(cols))
	for i, fd := range cols {
		headers[i] = columnHeader(fd)
	}
	return writeColumns(w, headers, rows, stateColumnIndex(cols), color)
}

// renderRecord prints a single resource as aligned key: value lines, recursing
// one level into nested messages with dotted keys.
func renderRecord(w io.Writer, m protoreflect.Message, color bool) error {
	return printPairs(w, flatten(m, ""), color)
}

// printPairs writes aligned "key: value" lines, coloring state-like values.
func printPairs(w io.Writer, pairs [][2]string, color bool) error {
	if len(pairs) == 0 {
		_, err := fmt.Fprintln(w, "(empty)")
		return err
	}
	keyWidth := 0
	for _, p := range pairs {
		if len(p[0]) > keyWidth {
			keyWidth = len(p[0])
		}
	}
	for _, p := range pairs {
		val := p[1]
		if color && isStateKey(p[0]) {
			val = colorize(val)
		}
		if _, err := fmt.Fprintf(w, "%-*s  %s\n", keyWidth, p[0]+":", val); err != nil {
			return err
		}
	}
	return nil
}

func flatten(m protoreflect.Message, prefix string) [][2]string {
	var out [][2]string
	fields := m.Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		if !m.Has(fd) {
			continue
		}
		out = append(out, flattenField(m, fd, prefix)...)
	}
	return out
}

// flattenField flattens a single (set) field into key/value pairs, recursing one level into
// singular messages with dotted keys.
func flattenField(m protoreflect.Message, fd protoreflect.FieldDescriptor, prefix string) [][2]string {
	key := prefix + jsonName(fd)
	switch {
	case fd.IsList():
		return [][2]string{{key, fmt.Sprintf("[%d item(s)]", m.Get(fd).List().Len())}}
	case fd.IsMap():
		return [][2]string{{key, fmt.Sprintf("{%d entr(ies)}", m.Get(fd).Map().Len())}}
	case fd.Kind() == protoreflect.MessageKind && fd.Message().FullName() == "google.protobuf.Timestamp":
		return [][2]string{{key, formatTimestamp(m.Get(fd).Message())}}
	case fd.Kind() == protoreflect.MessageKind:
		return flatten(m.Get(fd).Message(), key+".")
	default:
		return [][2]string{{key, formatDetail(m, fd)}}
	}
}

// selectColumns picks a compact, prioritized set of columns for a list item.
func selectColumns(desc protoreflect.MessageDescriptor) []protoreflect.FieldDescriptor {
	priority := []string{
		"name", "organization_name", "display_name", "organization_display_name", "id", "invite_id",
		"host_name", "spec_label", "instance_type", "email", "subject",
		"availability", "state", "power_state", "status", "phase", "role", "severity", "outcome",
		"action", "summary", "author_type", "author_id", "body",
		"datacenter_name", "region", "zone",
		"vcpus", "ram_gib", "total_cores", "size_gib", "storage_gib", "gpu", "gpu_model", "gpu_count",
		"public_ipv4", "endpoint_url", "meter", "quantity", "unit",
		"billing_mode", "monthly_price_minor", "currency", "amount_minor", "cost_minor",
		"actor", "principal", "resource_name",
		"timestamp_unix", "create_time_unix", "created_at_unix", "expires_at_unix", "bucket_start_unix",
	}
	fields := desc.Fields()
	byName := map[string]protoreflect.FieldDescriptor{}
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		byName[string(fd.Name())] = fd
	}
	var cols []protoreflect.FieldDescriptor
	for _, name := range priority {
		if fd, ok := byName[name]; ok && isScalarish(fd) {
			cols = append(cols, fd)
		}
		if len(cols) >= 7 {
			break
		}
	}
	if len(cols) == 0 {
		// Fallback: first few scalar fields in declaration order.
		for i := 0; i < fields.Len() && len(cols) < 5; i++ {
			fd := fields.Get(i)
			if isScalarish(fd) {
				cols = append(cols, fd)
			}
		}
	}
	return cols
}

func isScalarish(fd protoreflect.FieldDescriptor) bool {
	if fd.IsList() || fd.IsMap() {
		return false
	}
	return fd.Kind() != protoreflect.MessageKind && fd.Kind() != protoreflect.GroupKind
}

func columnHeader(fd protoreflect.FieldDescriptor) string {
	name := string(fd.Name())
	switch {
	case isTimeField(name):
		return timeColumnHeader(name)
	case strings.HasSuffix(name, "_minor"):
		return strings.ToUpper(strings.TrimSuffix(name, "_minor"))
	case strings.HasSuffix(name, "_gib"):
		return strings.ToUpper(strings.TrimSuffix(name, "_gib")) + "(GiB)"
	}
	return strings.ToUpper(name)
}

// isTimeField reports whether a field holds a unix timestamp the table renders as a relative age.
func isTimeField(name string) bool {
	return strings.HasSuffix(name, "_time_unix") || strings.HasSuffix(name, "_at_unix") ||
		strings.HasSuffix(name, "_seen_unix") || strings.HasSuffix(name, "_start_unix") ||
		strings.HasSuffix(name, "_end_unix") || name == "timestamp_unix"
}

// timeColumnHeader derives a distinct header for a unix-time field so messages with several
// timestamps (e.g. ApiKey's create/expires) don't collapse to several identical "AGE" columns.
// The canonical creation time stays "AGE"; everything else uses its semantic prefix.
func timeColumnHeader(name string) string {
	switch name {
	case "create_time_unix", "created_at_unix":
		return "AGE"
	}
	base := strings.TrimSuffix(name, "_unix")
	base = strings.TrimSuffix(base, "_at")
	base = strings.TrimSuffix(base, "_time")
	if base == "" {
		return "AGE"
	}
	return strings.ToUpper(base)
}

func stateColumnIndex(cols []protoreflect.FieldDescriptor) int {
	for i, fd := range cols {
		if isStateKey(string(fd.Name())) {
			return i
		}
	}
	return -1
}

func isStateKey(name string) bool {
	name = strings.ToLower(name)
	name = name[strings.LastIndexByte(name, '.')+1:]
	return name == "state" || name == "status" || name == "phase"
}

// formatValue renders a single list-cell value with human formatting.
func formatValue(m protoreflect.Message, fd protoreflect.FieldDescriptor) string {
	if !m.Has(fd) {
		if strings.HasSuffix(string(fd.Name()), "_unix") {
			return "-"
		}
		return ""
	}
	name := string(fd.Name())
	v := m.Get(fd)
	switch {
	case isTimeField(name):
		return humanAge(v.Int())
	case strings.HasSuffix(name, "_minor"):
		return formatMinor(v.Int(), siblingCurrency(m))
	case name == "name":
		return shortName(v.String())
	}
	return formatScalar(fd, v)
}

// formatDetail formats a scalar for the key:value record view. Like formatValue it humanizes
// timestamps (→ relative age) and money (_minor → currency) so the detail view matches the list
// view, but it keeps resource names full (no shortening) since the record is about one resource.
func formatDetail(m protoreflect.Message, fd protoreflect.FieldDescriptor) string {
	name := string(fd.Name())
	v := m.Get(fd)
	switch {
	case isTimeField(name):
		return humanAge(v.Int())
	case strings.HasSuffix(name, "_minor"):
		return formatMinor(v.Int(), siblingCurrency(m))
	}
	return formatScalar(fd, v)
}

func formatScalar(fd protoreflect.FieldDescriptor, v protoreflect.Value) string {
	switch fd.Kind() {
	case protoreflect.EnumKind:
		if ev := fd.Enum().Values().ByNumber(v.Enum()); ev != nil {
			return trimEnumPrefix(string(fd.Enum().Name()), string(ev.Name()))
		}
		return fmt.Sprintf("%d", v.Enum())
	case protoreflect.BoolKind:
		if v.Bool() {
			return "true"
		}
		return "false"
	default:
		return v.String()
	}
}

func siblingCurrency(m protoreflect.Message) string {
	fd := m.Descriptor().Fields().ByName("currency")
	if fd != nil && m.Has(fd) {
		return m.Get(fd).String()
	}
	return ""
}

// ── helpers ────────────────────────────────────────────────────────────────

func writeColumns(w io.Writer, headers []string, rows [][]string, stateCol int, color bool) error {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = displayWidth(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if w := displayWidth(cell); w > widths[i] {
				widths[i] = w
			}
		}
	}
	var b strings.Builder
	writeRow(&b, headers, widths, -1, false)
	for _, row := range rows {
		writeRow(&b, row, widths, stateCol, color)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func writeRow(b *strings.Builder, cells []string, widths []int, stateCol int, color bool) {
	for i, cell := range cells {
		display := cell
		if color && i == stateCol {
			display = colorize(cell)
		}
		b.WriteString(display)
		if i < len(cells)-1 {
			// Pad based on the raw (uncolored) display width so colors and multibyte
			// runes (e.g. "·") don't break alignment.
			b.WriteString(strings.Repeat(" ", widths[i]-displayWidth(cell)+3))
		}
	}
	b.WriteByte('\n')
}

func stringField(m protoreflect.Message, name string) string {
	fd := m.Descriptor().Fields().ByName(protoreflect.Name(name))
	if fd == nil || fd.Kind() != protoreflect.StringKind {
		return ""
	}
	return m.Get(fd).String()
}

func jsonName(fd protoreflect.FieldDescriptor) string {
	return fd.JSONName()
}

// shortName trims a resource path to its final segment for table density
// (projects/p/disks/d → d), leaving full names visible via -o json/yaml.
func shortName(full string) string {
	if i := strings.LastIndexByte(full, '/'); i >= 0 && i < len(full)-1 {
		return full[i+1:]
	}
	return full
}

// formatTimestamp humanizes a google.protobuf.Timestamp to a relative age (e.g. "3h").
func formatTimestamp(ts protoreflect.Message) string {
	secFD := ts.Descriptor().Fields().ByName("seconds")
	if secFD == nil {
		return "-"
	}
	return humanAge(ts.Get(secFD).Int())
}

func humanAge(unix int64) string {
	if unix <= 0 {
		return "-"
	}
	d := time.Since(time.Unix(unix, 0))
	if d < 0 {
		d = -d
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func formatMinor(minor int64, currency string) string {
	sym := "$"
	if strings.EqualFold(currency, "usdc") {
		sym = "USDC "
	}
	neg := ""
	if minor < 0 {
		neg, minor = "-", -minor
	}
	return fmt.Sprintf("%s%s%d.%02d", neg, sym, minor/100, minor%100)
}

// trimEnumPrefix turns STATE_RUNNING → RUNNING using the enum type's implied
// SCREAMING_SNAKE prefix (State → STATE_). UNSPECIFIED-style values are kept.
func trimEnumPrefix(enumType, value string) string {
	prefix := screamingSnake(enumType) + "_"
	if strings.HasPrefix(value, prefix) && len(value) > len(prefix) {
		return value[len(prefix):]
	}
	return value
}

func screamingSnake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
		}
		if r >= 'a' && r <= 'z' {
			b.WriteRune(r - 32)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func spaceWords(s string) string {
	return strings.ReplaceAll(s, "_", " ")
}

// renderNames implements -o name / --quiet: print just the full resource name(s),
// one per line, so output pipes cleanly into other commands. Falls back to the
// plain value for non-proto payloads.
func renderNames(w io.Writer, value any) error {
	msg, ok := value.(proto.Message)
	if !ok {
		_, err := fmt.Fprintln(w, value)
		return err
	}
	m := msg.ProtoReflect()
	if listFD, ok := primaryListField(m); ok {
		list := m.Get(listFD).List()
		for i := 0; i < list.Len(); i++ {
			if _, err := fmt.Fprintln(w, recordName(list.Get(i).Message())); err != nil {
				return err
			}
		}
		return nil
	}
	if n := recordName(m); n != "" {
		_, err := fmt.Fprintln(w, n)
		return err
	}
	return nil
}

// recordName returns the resource's identifying name, preferring a top-level
// `name` field and falling back to a nested resource's name (e.g. responses that
// wrap a single resource message).
func recordName(m protoreflect.Message) string {
	if n := stringField(m, "name"); n != "" {
		return n
	}
	fields := m.Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		if fd.Kind() == protoreflect.MessageKind && !fd.IsList() && !fd.IsMap() && m.Has(fd) {
			if n := stringField(m.Get(fd).Message(), "name"); n != "" {
				return n
			}
		}
	}
	return ""
}

// ── color ──────────────────────────────────────────────────────────────────

const (
	ansiReset  = "\033[0m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiRed    = "\033[31m"
	ansiDim    = "\033[2m"
)

func colorEnabled(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

func colorize(state string) string {
	c := stateColor(state)
	if c == "" {
		return state
	}
	return c + state + ansiReset
}

func stateColor(state string) string {
	switch up := strings.ToUpper(strings.TrimSpace(state)); {
	case up == "":
		return ""
	case containsAny(up, "RUNNING", "ACTIVE", "READY", "SUCCEEDED", "AVAILABLE", "ATTACHED", "HEALTHY", "OPEN", "PAID"):
		return ansiGreen
	case containsAny(up, "PENDING", "PROVISIONING", "CREATING", "STARTING", "STOPPING", "RESIZING", "UPDATING", "PROCESSING"):
		return ansiYellow
	case containsAny(up, "FAILED", "ERROR", "CANCELLED", "CANCELED", "DELETED", "SUSPENDED", "UNHEALTHY", "DEGRADED"):
		return ansiRed
	case containsAny(up, "STOPPED", "DISABLED", "INACTIVE", "CLOSED", "UNSPECIFIED"):
		return ansiDim
	}
	return ""
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
