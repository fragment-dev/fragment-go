package typedentries

import (
	"bytes"
	"go/format"
	"strings"
	"testing"
)

// The tests here assert the generator's output verbatim. That is the point: the
// output is source code a customer reads and writes against, so the useful
// question is not "does it contain something" but "is it exactly this".
//
// They complement rather than duplicate the two coarser checks elsewhere:
// snapshot_test.go pins whole files for two realistic operation documents, and
// internal/generated/compile_test.go proves the output builds. Neither can say
// where a change originated. These can.

// assertSource compares generated source verbatim and, on failure, reports the
// first line that differs rather than dumping two blocks for the reader to diff by
// eye.
func assertSource(t *testing.T, got, want string) {
	t.Helper()

	if got == want {
		return
	}

	gotLines, wantLines := strings.Split(got, "\n"), strings.Split(want, "\n")
	for i := 0; i < len(gotLines) || i < len(wantLines); i++ {
		g, w := "", ""
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if g != w {
			t.Errorf("generated source differs at line %d\n  want: %q\n   got: %q\n\nfull output:\n%s",
				i+1, w, g, got)
			return
		}
	}
}

func TestComment(t *testing.T) {
	cases := map[string]struct {
		indent, text, want string
	}{
		"short line fits": {
			text: "one short line",
			want: "// one short line\n",
		},
		"empty text is a paragraph break": {
			text: "",
			want: "//\n",
		},
		"indent is applied to every line": {
			indent: "\t",
			text:   "a word-wrapping case with enough words in it that it has to break across more than one line to fit",
			want: "\t// a word-wrapping case with enough words in it that it has to break across\n" +
				"\t// more than one line to fit\n",
		},
		"runs of whitespace collapse": {
			text: "words    separated \n by  odd   whitespace",
			want: "// words separated by odd whitespace\n",
		},
		"a word longer than the limit overflows rather than breaking": {
			// Hard-breaking would split identifiers and URLs, which is worse than
			// one long line. Pinned so that changing the wrapping cannot start
			// mangling them by accident.
			text: "supercalifragilisticexpialidociousandthensomemoreletterstopushitwellpastthelimit tail",
			want: "// supercalifragilisticexpialidociousandthensomemoreletterstopushitwellpastthelimit\n" +
				"// tail\n",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var b bytes.Buffer
			comment(&b, tc.indent, tc.text)
			assertSource(t, b.String(), tc.want)
		})
	}
}

func TestParamDoc(t *testing.T) {
	cases := map[string]struct {
		param Param
		want  string
	}{
		// The wire name is named even when the mapping is obvious, because it is
		// the fact a reader cannot recover from the field name.
		"required parameter": {
			param: Param{WireName: "amount", FieldName: "Amount", Kind: KindValue},
			want:  `Amount is the "amount" parameter.`,
		},
		"optional parameter says so": {
			param: Param{WireName: "memo", FieldName: "Memo", Kind: KindPointer},
			want:  `Memo is the "memo" parameter. Optional; leave nil to omit it.`,
		},
		"a slice is optional too": {
			param: Param{WireName: "tag_list", FieldName: "TagList", Kind: KindSlice},
			want:  `TagList is the "tag_list" parameter. Optional; leave nil to omit it.`,
		},
		// Two collisions that call for different reactions from the reader, so the
		// wording distinguishes them.
		"renamed off a common field": {
			param: Param{WireName: "posted", FieldName: "Posted2", Kind: KindPointer},
			want: `Posted2 is the "posted" parameter. It is not Posted, which is a field every ` +
				`entry carries; the parameter is renamed because that name is taken. ` +
				`Optional; leave nil to omit it.`,
		},
		"renamed off another parameter": {
			param: Param{WireName: "userId", FieldName: "UserId2", Kind: KindValue},
			want: `UserId2 is the "userId" parameter. It is renamed from UserId, which ` +
				`another parameter of this entry type already uses.`,
		},
		// A field cannot share a name with a method the payload declares, so those
		// names are reserved too. The wording is inaccurate for them: MarshalJSON is
		// a method, not "a field every entry carries". Pinned as-is so that
		// correcting it registers as a deliberate change.
		"renamed off a generated method": {
			param: Param{WireName: "marshalJSON", FieldName: "MarshalJSON2", Kind: KindValue},
			want: `MarshalJSON2 is the "marshalJSON" parameter. It is not MarshalJSON, which ` +
				`is a field every entry carries; the parameter is renamed because that ` +
				`name is taken.`,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := paramDoc(tc.param); got != tc.want {
				t.Errorf("paramDoc\n  want: %q\n   got: %q", tc.want, got)
			}
		})
	}
}

// TestSetField covers the choice of helper, which follows the shape of the Go type
// rather than whether the Schema marked the field required.
//
// The distinction matters because a nullable list is a slice, not a pointer.
// Dispatching on required-ness hands it to the pointer helper, which gofmt accepts
// and the compiler rejects.
func TestSetField(t *testing.T) {
	cases := map[string]struct {
		kind ParamKind
		want string
	}{
		"a value is always written": {
			kind: KindValue,
			want: "\tparams.Set(\"wire_name\", e.Field)\n",
		},
		"a pointer is written only when non-nil": {
			kind: KindPointer,
			want: "\tbatch.SetOpt(params, \"wire_name\", e.Field)\n",
		},
		"a slice is written only when non-nil": {
			kind: KindSlice,
			want: "\tbatch.SetSlice(params, \"wire_name\", e.Field)\n",
		},
		// Raw JSON cannot go through the generic slice helper: json.RawMessage is a
		// []byte, so that would encode it as base64 instead of as JSON.
		"raw JSON is guarded inline": {
			kind: KindRaw,
			want: "\tif e.Field != nil {\n\t\tparams.Set(\"wire_name\", e.Field)\n\t}\n",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var b bytes.Buffer
			setField(&b, "params", "wire_name", "e.Field", tc.kind)
			assertSource(t, b.String(), tc.want)
		})
	}
}

// TestNeedsJSON covers the one conditional import. A missing import is not something
// gofmt reports, so getting this wrong produces a file that formats cleanly and does
// not build.
func TestNeedsJSON(t *testing.T) {
	cases := map[string]struct {
		payloads []Payload
		want     bool
	}{
		"no json anywhere": {
			payloads: []Payload{{Params: []Param{{GoType: "string"}, {GoType: "[]string"}}}},
			want:     false,
		},
		"an untyped payload carries raw parameters": {
			payloads: []Payload{{Untyped: true}},
			want:     true,
		},
		"a parameter bound to a JSON scalar": {
			payloads: []Payload{{Params: []Param{{GoType: "json.RawMessage"}}}},
			want:     true,
		},
		"a pointer to raw JSON counts too": {
			payloads: []Payload{{Params: []Param{{GoType: "*json.RawMessage"}}}},
			want:     true,
		},
		"only one payload of several needs it": {
			payloads: []Payload{
				{Params: []Param{{GoType: "string"}}},
				{Params: []Param{{GoType: "json.RawMessage"}}},
			},
			want: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := needsJSON(tc.payloads); got != tc.want {
				t.Errorf("needsJSON = %v, want %v", got, tc.want)
			}
		})
	}
}

// thingV2 is a payload with one required and one optional parameter: the smallest
// input that exercises both setter shapes.
var thingV2 = Payload{
	Type: "thing", TypeVersion: 2, GoName: "ThingV2Entry", SourceOp: "PostThing",
	Params: []Param{
		{WireName: "amount", FieldName: "Amount", GoType: "string", Kind: KindValue, Required: true},
		{WireName: "memo", FieldName: "Memo", GoType: "*string", Kind: KindPointer},
	},
}

// TestEmitPayload asserts one payload's declaration and methods in full.
//
// Worth reading as the specification of a payload's shape: the leading `_ struct{}`
// that makes an unkeyed literal illegal, all seven common fields regardless of what
// the operation bound, then the parameters in Schema order.
//
// Output is pre-gofmt, so the whitespace here is what the generator writes rather
// than what lands on disk.
func TestEmitPayload(t *testing.T) {
	const want = `
// ThingV2Entry is the "thing" Ledger Entry, version 2.
//
// Fields must be set by name. An unkeyed literal will not compile, which is
// deliberate: two parameters of the same type could otherwise be swapped by
// reordering them, and the mistake would be silent.
type ThingV2Entry struct {
	// Prevents an unkeyed composite literal, which would let parameters be
	// supplied positionally.
	_ struct{}

	// Ik is the idempotency key for this Ledger Entry.
	Ik string
	// LedgerIk identifies the Ledger to post this entry to.
	LedgerIk string
	// Posted is an ISO 8601 timestamp, for example "2021-01-01T16:45:00Z". Leave
	// nil to omit it.
	Posted *string
	// Description is also used for this entry's Ledger Lines unless they set their
	// own. Leave nil to omit it.
	Description *string
	// Tags attached to this Ledger Entry. Leave nil to omit it.
	Tags []queries.LedgerEntryTagInput
	// Groups this Ledger Entry is added to. Leave nil to omit it.
	Groups []queries.LedgerEntryGroupInput
	// Conditions that must hold for this Ledger Entry to post. The whole batch
	// rejects if any is not met. Leave nil to omit it.
	Conditions []queries.LedgerEntryConditionInput

	// The parameters of this entry type, in Schema order.

	// Amount is the "amount" parameter.
	Amount string

	// Memo is the "memo" parameter. Optional; leave nil to omit it.
	Memo *string
}

// FragmentBatchEntry marks ThingV2Entry as usable in a batch.
func (ThingV2Entry) FragmentBatchEntry() {}

// MarshalJSON encodes the entry as an AddLedgerEntryInput.
//
// Fields the caller did not set are omitted rather than sent as null, and
// parameters keep the order they have in the Schema.
func (e ThingV2Entry) MarshalJSON() ([]byte, error) {
	ledger := batch.NewObject()
	ledger.Set("ik", e.LedgerIk)

	params := batch.NewObject()
	params.Set("amount", e.Amount)
	batch.SetOpt(params, "memo", e.Memo)

	entry := batch.NewObject()
	entry.Set("ledger", ledger)
	entry.Set("type", "thing")
	entry.Set("typeVersion", 2)
	batch.SetOpt(entry, "posted", e.Posted)
	batch.SetOpt(entry, "description", e.Description)
	batch.SetSlice(entry, "tags", e.Tags)
	batch.SetSlice(entry, "groups", e.Groups)
	batch.SetSlice(entry, "conditions", e.Conditions)
	entry.Set("parameters", params)

	out := batch.NewObject()
	out.Set("ik", e.Ik)
	out.Set("entry", entry)
	return out.MarshalJSON()
}
`

	var b bytes.Buffer
	emitPayload(&b, thingV2)
	assertSource(t, b.String(), want)
}

// TestEmitPayloadUntyped covers the fallback for an operation that bound its
// parameters as a whole rather than field by field.
//
// Two things differ from a typed payload: a raw Parameters field replaces the
// derived ones, and MarshalJSON builds no params object at all, setting the
// caller's JSON directly when it is present.
func TestEmitPayloadUntyped(t *testing.T) {
	const want = `
// RuntimeV1Entry is the "runtime" Ledger Entry, version 1.
//
// Fields must be set by name. An unkeyed literal will not compile, which is
// deliberate: two parameters of the same type could otherwise be swapped by
// reordering them, and the mistake would be silent.
type RuntimeV1Entry struct {
	// Prevents an unkeyed composite literal, which would let parameters be
	// supplied positionally.
	_ struct{}

	// Ik is the idempotency key for this Ledger Entry.
	Ik string
	// LedgerIk identifies the Ledger to post this entry to.
	LedgerIk string
	// Posted is an ISO 8601 timestamp, for example "2021-01-01T16:45:00Z". Leave
	// nil to omit it.
	Posted *string
	// Description is also used for this entry's Ledger Lines unless they set their
	// own. Leave nil to omit it.
	Description *string
	// Tags attached to this Ledger Entry. Leave nil to omit it.
	Tags []queries.LedgerEntryTagInput
	// Groups this Ledger Entry is added to. Leave nil to omit it.
	Groups []queries.LedgerEntryGroupInput
	// Conditions that must hold for this Ledger Entry to post. The whole batch
	// rejects if any is not met. Leave nil to omit it.
	Conditions []queries.LedgerEntryConditionInput

	// Parameters for this entry type could not be typed from the source operation,
	// which did not bind them individually. Supply encoded JSON, or leave nil to
	// omit it.
	Parameters json.RawMessage
}

// FragmentBatchEntry marks RuntimeV1Entry as usable in a batch.
func (RuntimeV1Entry) FragmentBatchEntry() {}

// MarshalJSON encodes the entry as an AddLedgerEntryInput.
//
// Fields the caller did not set are omitted rather than sent as null, and
// parameters keep the order they have in the Schema.
func (e RuntimeV1Entry) MarshalJSON() ([]byte, error) {
	ledger := batch.NewObject()
	ledger.Set("ik", e.LedgerIk)

	entry := batch.NewObject()
	entry.Set("ledger", ledger)
	entry.Set("type", "runtime")
	entry.Set("typeVersion", 1)
	batch.SetOpt(entry, "posted", e.Posted)
	batch.SetOpt(entry, "description", e.Description)
	batch.SetSlice(entry, "tags", e.Tags)
	batch.SetSlice(entry, "groups", e.Groups)
	batch.SetSlice(entry, "conditions", e.Conditions)
	if e.Parameters != nil {
		entry.Set("parameters", e.Parameters)
	}

	out := batch.NewObject()
	out.Set("ik", e.Ik)
	out.Set("entry", entry)
	return out.MarshalJSON()
}
`

	var b bytes.Buffer
	emitPayload(&b, Payload{Type: "runtime", TypeVersion: 1, GoName: "RuntimeV1Entry", Untyped: true})
	assertSource(t, b.String(), want)
}

// TestEmitMarshalWritesEachKind shows all four setter shapes in one function body,
// and that parameters appear in the order given rather than grouped by kind.
func TestEmitMarshalWritesEachKind(t *testing.T) {
	const want = `// MarshalJSON encodes the entry as an AddLedgerEntryInput.
//
// Fields the caller did not set are omitted rather than sent as null, and
// parameters keep the order they have in the Schema.
func (e KindsV1Entry) MarshalJSON() ([]byte, error) {
	ledger := batch.NewObject()
	ledger.Set("ik", e.LedgerIk)

	params := batch.NewObject()
	params.Set("plain", e.Plain)
	batch.SetOpt(params, "opt", e.Opt)
	batch.SetSlice(params, "list", e.List)
	if e.Raw != nil {
		params.Set("raw", e.Raw)
	}

	entry := batch.NewObject()
	entry.Set("ledger", ledger)
	entry.Set("type", "kinds")
	entry.Set("typeVersion", 1)
	batch.SetOpt(entry, "posted", e.Posted)
	batch.SetOpt(entry, "description", e.Description)
	batch.SetSlice(entry, "tags", e.Tags)
	batch.SetSlice(entry, "groups", e.Groups)
	batch.SetSlice(entry, "conditions", e.Conditions)
	entry.Set("parameters", params)

	out := batch.NewObject()
	out.Set("ik", e.Ik)
	out.Set("entry", entry)
	return out.MarshalJSON()
}
`

	var b bytes.Buffer
	emitMarshal(&b, Payload{
		Type: "kinds", TypeVersion: 1, GoName: "KindsV1Entry",
		Params: []Param{
			{WireName: "plain", FieldName: "Plain", GoType: "string", Kind: KindValue, Required: true},
			{WireName: "opt", FieldName: "Opt", GoType: "*string", Kind: KindPointer},
			{WireName: "list", FieldName: "List", GoType: "[]string", Kind: KindSlice},
			{WireName: "raw", FieldName: "Raw", GoType: "json.RawMessage", Kind: KindRaw},
		},
	})
	assertSource(t, b.String(), want)
}

// TestEmitFileHeader asserts everything Emit adds around the payloads: the
// generated-code marker, the package doc, and the import block.
//
// Asserted separately from the payload body so that a change to one does not
// require re-reading the other.
func TestEmitFileHeader(t *testing.T) {
	const want = `// Code generated by github.com/fragment-dev/fragment-go/v4. DO NOT EDIT.

// Package typed_payloads holds one typed payload per Ledger Entry type and
// version found in your GraphQL operations. Pass them to
// queries.AddTypedLedgerEntries to commit them as one atomic batch.
//
// Set fields by name. An unkeyed literal will not compile, so that two
// parameters of the same type cannot be swapped by reordering them.
//
// Note that tags, groups and conditions use the input types from the SDK's
// queries package, not the package these payloads were generated alongside.
package typed_payloads

import (
	"github.com/fragment-dev/fragment-go/v4/batch"
	"github.com/fragment-dev/fragment-go/v4/queries"
)
`

	source, err := Emit([]Payload{thingV2})
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}

	header, _, found := strings.Cut(string(source), "\n// ThingV2Entry")
	if !found {
		t.Fatalf("could not find the payload declaration in:\n%s", source)
	}
	assertSource(t, header, want)
}

// TestEmitImportsEncodingJSON checks the one import that is conditional.
func TestEmitImportsEncodingJSON(t *testing.T) {
	const want = `import (
	"encoding/json"

	"github.com/fragment-dev/fragment-go/v4/batch"
	"github.com/fragment-dev/fragment-go/v4/queries"
)
`

	source, err := Emit([]Payload{{Type: "runtime", TypeVersion: 1, GoName: "RuntimeV1Entry", Untyped: true}})
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}

	_, rest, found := strings.Cut(string(source), "package typed_payloads\n\n")
	if !found {
		t.Fatalf("could not find the package clause in:\n%s", source)
	}
	got, _, found := strings.Cut(rest, "\n// RuntimeV1Entry")
	if !found {
		t.Fatalf("could not find the payload declaration in:\n%s", source)
	}
	assertSource(t, got, want)
}

// TestEmitOutputIsFormatted checks that Emit's result is valid, canonically
// formatted Go: running gofmt over it again changes nothing.
//
// The unit tests above assert pre-gofmt output, so nothing else here would notice
// if the formatting pass were dropped — and since the payloads land in a customer's
// tree, unformatted output would show up as noise in their diffs.
func TestEmitOutputIsFormatted(t *testing.T) {
	source, err := Emit([]Payload{thingV2, {Type: "runtime", TypeVersion: 1, GoName: "RuntimeV1Entry", Untyped: true}})
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}

	// format.Source both parses and canonicalises, so a difference here means the
	// output is either not formatted or not valid Go.
	formatted, err := format.Source(source)
	if err != nil {
		t.Fatalf("output is not valid Go: %v\n%s", err, source)
	}
	assertSource(t, string(source), string(formatted))
}

// TestEmitOrdersPayloadsAsGiven covers the one thing Emit decides about more than
// one payload: it does not sort them.
func TestEmitOrdersPayloadsAsGiven(t *testing.T) {
	source, err := Emit([]Payload{
		{Type: "zulu", TypeVersion: 1, GoName: "ZuluV1Entry"},
		{Type: "alpha", TypeVersion: 1, GoName: "AlphaV1Entry"},
	})
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}

	zuluAt := strings.Index(string(source), "type ZuluV1Entry")
	alphaAt := strings.Index(string(source), "type AlphaV1Entry")
	if zuluAt < 0 || alphaAt < 0 {
		t.Fatalf("both payloads should be present:\n%s", source)
	}
	if zuluAt > alphaAt {
		t.Error("payloads were sorted; they should keep the order they were derived in")
	}
}

// TestEmitReportsUnformattableSource checks the failure path, which returns the
// offending source alongside the error. A generator bug is far easier to diagnose
// from what it produced than from a parse error's line number alone.
func TestEmitReportsUnformattableSource(t *testing.T) {
	// A payload name that is not a legal identifier. Derive cannot produce this —
	// exportedIdent sanitises it — so this reaches Emit only if that guard is
	// removed, which is precisely when the diagnostic matters.
	_, err := Emit([]Payload{{Type: "broken", TypeVersion: 1, GoName: "not a valid name"}})
	if err == nil {
		t.Fatal("expected an error for source that cannot be formatted")
	}
	if !strings.Contains(err.Error(), "formatting generated source") {
		t.Errorf("error should say what failed, got: %v", err)
	}
	if !strings.Contains(err.Error(), "type not a valid name struct") {
		t.Errorf("error should include the offending source, got: %v", err)
	}
}
