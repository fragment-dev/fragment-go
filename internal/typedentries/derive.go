// Package typedentries derives typed batch Ledger Entry payloads from GraphQL
// operations.
//
// The Fragment CLI generates one addLedgerEntry mutation per entry type in a
// Schema. Those operations carry two facts that the addLedgerEntries batch
// mutation cannot express on its own: the entry type, as a string literal, and
// each parameter, bound to a typed operation variable. This package reads them
// back out so the generator can emit a typed struct per entry type.
package typedentries

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
)

// Param is one caller-supplied parameter of a typed payload.
type Param struct {
	// WireName is the parameter name exactly as it appears in the Schema. It is
	// what goes on the wire, never a normalized form of it.
	WireName string
	// FieldName is the Go field name, which may have been escaped to be a legal
	// and unique identifier. Escaping never affects WireName.
	FieldName string
	// GoType is the Go type of the field, a pointer when the parameter is
	// optional.
	GoType string
	// Required reports whether the operation declared the variable non-null.
	Required bool
}

// Payload is one typed entry model, identified by (Type, TypeVersion).
type Payload struct {
	Type        string
	TypeVersion int
	// GoName is the generated struct name. It always carries the version, so it
	// depends only on this payload's own identity and never on which other
	// operations were in the input.
	GoName string
	// Params are in the order they appear in the source operation.
	Params []Param
	// Untyped is set when the operation gave no inline parameters object, in
	// which case callers fall back to an untyped map.
	Untyped bool
	// SourceOp is the operation the payload was derived from, for diagnostics.
	SourceOp string
}

// commonFields are the fields every payload carries regardless of entry type.
// They are fixed by LedgerEntryInput and deliberately not derived from the
// source operation: an operation binds only the fields the CLI chose to expose,
// and that choice has already changed between CLI versions. Deriving the set
// would invent a restriction the API does not have.
//
// lines is absent on purpose. It cannot be combined with an entry that has a
// type. type, typeVersion and parameters are derived rather than supplied.
var commonFields = []string{
	"Ik",
	"LedgerIk",
	"Posted",
	"Description",
	"Tags",
	"Groups",
	"Conditions",
}

// Derive reads every operation in the given sources and returns one payload per
// distinct (type, typeVersion), in first-seen order.
//
// Operations that are not typed entry operations are skipped silently; that is
// not an error. Warnings describe things the caller should know about but that
// do not stop generation.
func Derive(sources []*ast.Source, scalars map[string]string) ([]Payload, []string, error) {
	var (
		payloads []Payload
		warnings []string
		index    = map[string]int{}
	)

	for _, src := range sources {
		doc, err := parser.ParseQuery(src)
		if err != nil {
			return nil, nil, fmt.Errorf("parsing %s: %w", src.Name, err)
		}

		for _, op := range doc.Operations {
			entry, ok := recognize(op)
			if !ok {
				continue
			}

			version, ok := typeVersionOf(entry)
			if !ok {
				// Identity is (type, typeVersion), and a non-literal version
				// leaves it undefined. Treating it as absent would pin the
				// payload to version 1 even though the operation accepts any
				// version, so skip rather than guess.
				warnings = append(warnings, fmt.Sprintf(
					"operation %q has a non-literal typeVersion; skipping, as its (type, typeVersion) identity cannot be determined",
					op.Name))
				continue
			}

			entryType := entry.Children.ForName("type").Raw
			key := entryType + "\x00" + strconv.Itoa(version)

			params, untyped := paramsOf(op, entry, scalars)

			if prev, seen := index[key]; seen {
				// The CLI and API guarantee no two entries share a
				// (type, typeVersion), so duplicates necessarily describe the
				// same model and the first wins. Differing parameters mean the
				// .graphql is stale rather than that the input is invalid, so
				// warn rather than fail the customer's build.
				if !sameParams(payloads[prev].Params, params) {
					warnings = append(warnings, fmt.Sprintf(
						"operations %q and %q both describe %s v%d but declare different parameters; keeping %q. Regenerate your .graphql from the current Schema",
						payloads[prev].SourceOp, op.Name, entryType, version, payloads[prev].SourceOp))
				}
				continue
			}

			nameWarnings := escapeFields(params)
			for _, w := range nameWarnings {
				warnings = append(warnings, fmt.Sprintf("%s v%d: %s", entryType, version, w))
			}

			index[key] = len(payloads)
			payloads = append(payloads, Payload{
				Type:        entryType,
				TypeVersion: version,
				GoName:      payloadName(entryType, version),
				Params:      params,
				Untyped:     untyped,
				SourceOp:    op.Name,
			})
		}
	}

	if dupes := duplicateGoNames(payloads); len(dupes) > 0 {
		return nil, warnings, fmt.Errorf(
			"entry types %s produce the same Go identifier; rename one in your Schema", strings.Join(dupes, ", "))
	}

	return payloads, warnings, nil
}

// recognize reports whether an operation is a typed entry operation and, if so,
// returns its inline entry object.
//
// Every condition must hold. Failing any of them is not an error; it is how the
// SDK's own addLedgerEntry operations, whose type is a variable rather than a
// literal, are excluded.
func recognize(op *ast.OperationDefinition) (*ast.Value, bool) {
	if op.Operation != ast.Mutation || op.Name == "" {
		return nil, false
	}
	if len(op.SelectionSet) != 1 {
		return nil, false
	}
	field, ok := op.SelectionSet[0].(*ast.Field)
	if !ok || field.Name != "addLedgerEntry" {
		return nil, false
	}
	arg := field.Arguments.ForName("entry")
	if arg == nil || arg.Value == nil || arg.Value.Kind != ast.ObjectValue {
		return nil, false
	}
	entryType := arg.Value.Children.ForName("type")
	if entryType == nil || entryType.Kind != ast.StringValue {
		return nil, false
	}
	return arg.Value, true
}

// typeVersionOf returns the version the entry pins. An operation that pins no
// version is normalised to 1, because an entry with no typeVersion resolves to
// version 1 server-side rather than to the latest. Normalising here keeps the
// generated name and the wire value from disagreeing.
//
// The second result is false when a version is present but is not an integer
// literal, leaving the payload's identity undefined.
func typeVersionOf(entry *ast.Value) (int, bool) {
	v := entry.Children.ForName("typeVersion")
	if v == nil {
		return 1, true
	}
	if v.Kind != ast.IntValue {
		return 0, false
	}
	n, err := strconv.Atoi(v.Raw)
	if err != nil {
		return 0, false
	}
	return n, true
}

// paramsOf extracts the caller-supplied parameters, in source order.
func paramsOf(op *ast.OperationDefinition, entry *ast.Value, scalars map[string]string) ([]Param, bool) {
	obj := entry.Children.ForName("parameters")
	if obj == nil || obj.Kind != ast.ObjectValue {
		// Either no parameters at all, or bound to a variable so their
		// individual types are not visible here. Either way there is nothing to
		// type, and the payload falls back to an untyped map.
		return nil, true
	}

	var params []Param
	for _, child := range obj.Children {
		if child.Value.Kind != ast.Variable {
			// The operation fixes this value, so it is not something the caller
			// supplies.
			continue
		}
		def := op.VariableDefinitions.ForName(child.Value.Raw)
		if def == nil || def.Type == nil {
			continue
		}
		// Type and required-ness come from the variable definition, never from
		// the field name.
		params = append(params, Param{
			WireName: child.Name,
			GoType:   goType(def.Type, scalars),
			Required: def.Type.NonNull,
		})
	}
	return params, false
}

// builtinScalars are the scalars every GraphQL schema has. They are not in the
// generator's binding table, which only lists Fragment's own scalars, so
// resolving them here is what keeps a plain String! parameter from being
// mistaken for a Schema type.
var builtinScalars = map[string]string{
	"String":  "string",
	"Int":     "int",
	"Float":   "float64",
	"Boolean": "bool",
	"ID":      "string",
}

// goType maps a GraphQL type to its Go equivalent, matching what genqlient
// generates for the same type: optional values are pointers, and lists are
// slices whether or not they are nullable.
func goType(t *ast.Type, scalars map[string]string) string {
	if t.Elem != nil {
		// A slice already has a nil value, so genqlient does not add a pointer
		// for a nullable list and neither do we.
		return "[]" + goType(t.Elem, scalars)
	}

	base, ok := scalars[t.NamedType]
	if !ok {
		base, ok = builtinScalars[t.NamedType]
	}
	if !ok {
		// Not a scalar, so it is an enum or input object, which genqlient emits
		// into the SDK's queries package under its Schema name.
		base = "queries." + t.NamedType
	}
	if t.NonNull {
		return base
	}
	return "*" + base
}

// escapeFields assigns each parameter a legal, unique Go field name.
//
// Names must not collide with each other or with the common fields every
// payload carries, and the first occurrence in source order keeps the plain
// name. Escaping is purely local: WireName is untouched, so each parameter
// still carries its own value to the API.
func escapeFields(params []Param) []string {
	var warnings []string
	taken := map[string]bool{}
	for _, f := range commonFields {
		taken[f] = true
	}

	for i := range params {
		want := exportedIdent(params[i].WireName)
		name := want
		for n := 2; taken[name]; n++ {
			name = want + strconv.Itoa(n)
		}
		taken[name] = true
		params[i].FieldName = name

		if name != want {
			warnings = append(warnings, fmt.Sprintf(
				"parameter %q maps to field %s because %s is already taken", params[i].WireName, name, want))
		}
	}
	return warnings
}

// exportedIdent converts a Schema name to an exported Go identifier, splitting
// on the separators Fragment entry types and parameters use.
func exportedIdent(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-' || r == '.' || r == ' ' || r == '/'
	})
	var b strings.Builder
	for _, p := range parts {
		r := []rune(p)
		b.WriteString(strings.ToUpper(string(r[0])))
		if len(r) > 1 {
			b.WriteString(string(r[1:]))
		}
	}
	out := b.String()
	if out == "" || !isLetter(rune(out[0])) {
		out = "F" + out
	}
	return out
}

func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// payloadName builds a payload's struct name. The version is always present, so
// adding a v2 later never renames v1 and existing call sites keep compiling.
func payloadName(entryType string, version int) string {
	return exportedIdent(entryType) + "V" + strconv.Itoa(version) + "Entry"
}

func sameParams(a, b []Param) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].WireName != b[i].WireName || a[i].GoType != b[i].GoType || a[i].Required != b[i].Required {
			return false
		}
	}
	return true
}

// duplicateGoNames reports entry types that collapse onto one identifier, for
// example "user-funds" and "user_funds". Unlike a parameter collision this
// cannot be resolved by suffixing, because a name must depend only on its own
// identity; suffixing here would rename a payload whenever an unrelated one was
// added.
func duplicateGoNames(payloads []Payload) []string {
	byName := map[string][]string{}
	for _, p := range payloads {
		byName[p.GoName] = append(byName[p.GoName], fmt.Sprintf("%s v%d", p.Type, p.TypeVersion))
	}
	var out []string
	for _, types := range byName {
		if len(types) > 1 {
			sort.Strings(types)
			out = append(out, strings.Join(types, " and "))
		}
	}
	sort.Strings(out)
	return out
}
