package typedentries

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/vektah/gqlparser/v2/ast"
)

// scalars mirrors the generator's binding table, which lists only Fragment's own
// scalars. String, Int and friends are deliberately absent: they are built in,
// and leaving them out here is what makes these tests exercise the same
// resolution path the CLI uses.
var scalars = map[string]string{
	"SafeString": "string",
	"DateTime":   "string",
	"JSON":       "json.RawMessage",
}

func derive(t *testing.T, src string) ([]Payload, []string) {
	t.Helper()
	payloads, warnings, err := Derive([]*ast.Source{{Name: "test.graphql", Input: src}}, scalars)
	if err != nil {
		t.Fatalf("Derive: %v", err)
	}
	return payloads, warnings
}

// TestRecognitionSkipsNonQualifyingOperations covers the requirement that an
// operation failing any recognition condition is skipped silently rather than
// reported as an error. Each case fails exactly one condition.
func TestRecognitionSkipsNonQualifyingOperations(t *testing.T) {
	cases := map[string]string{
		"a query rather than a mutation": `
			query Q($ik: SafeString!, $ledgerIk: SafeString!) {
			  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "t"}) { __typename }
			}`,
		"more than one selection": `
			mutation M($ik: SafeString!, $ledgerIk: SafeString!) {
			  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "t"}) { __typename }
			  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "u"}) { __typename }
			}`,
		"a different field": `
			mutation M($ik: SafeString!) {
			  reverseLedgerEntry(ik: $ik, entry: {type: "t"}) { __typename }
			}`,
		"no entry argument": `
			mutation M($ik: SafeString!) {
			  addLedgerEntry(ik: $ik) { __typename }
			}`,
		"entry bound to a variable": `
			mutation M($ik: SafeString!, $entry: LedgerEntryInput!) {
			  addLedgerEntry(ik: $ik, entry: $entry) { __typename }
			}`,
		"type bound to a variable": `
			mutation M($ik: SafeString!, $entryType: String!) {
			  addLedgerEntry(ik: $ik, entry: {type: $entryType}) { __typename }
			}`,
		"no type at all": `
			mutation M($ik: SafeString!, $ledgerIk: SafeString!) {
			  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}}) { __typename }
			}`,
	}

	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			payloads, _ := derive(t, src)
			if len(payloads) != 0 {
				t.Errorf("expected no payloads, got %d: %+v", len(payloads), payloads)
			}
		})
	}
}

// TestSDKOperationsDeriveNothing runs the derivation over the SDK's own
// operations. Those include addLedgerEntry, whose type is a variable, so the
// generator must find nothing rather than emitting a bogus payload — and the
// SDK's own `make codegen` must stay a no-op.
func TestSDKOperationsDeriveNothing(t *testing.T) {
	content, err := os.ReadFile("../../queries/queries.graphql")
	if err != nil {
		t.Fatalf("reading the SDK's operations: %v", err)
	}

	payloads, warnings, err := Derive(
		[]*ast.Source{{Name: "queries.graphql", Input: string(content)}}, scalars)
	if err != nil {
		t.Fatalf("Derive: %v", err)
	}
	if len(payloads) != 0 {
		t.Errorf("the SDK's own operations should derive no typed payloads, got %d: %+v", len(payloads), payloads)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
}

// TestOperationNameIsIrrelevant covers the requirement that generators must not
// depend on a naming convention.
func TestOperationNameIsIrrelevant(t *testing.T) {
	payloads, _ := derive(t, `
		mutation zzz_unconventional_name($ik: SafeString!, $ledgerIk: SafeString!, $amount: String!) {
		  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "t", parameters: {amount: $amount}}) { __typename }
		}`)

	if len(payloads) != 1 {
		t.Fatalf("expected 1 payload, got %d", len(payloads))
	}
	if payloads[0].GoName != "TV1Entry" {
		t.Errorf("GoName = %q, want TV1Entry", payloads[0].GoName)
	}
}

// TestUnpinnedVersionNormalisesToOne covers the requirement that an operation
// pinning no typeVersion is treated as version 1 everywhere: identity, name and
// wire value.
func TestUnpinnedVersionNormalisesToOne(t *testing.T) {
	payloads, _ := derive(t, `
		mutation A($ik: SafeString!, $ledgerIk: SafeString!, $amount: String!) {
		  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "t", parameters: {amount: $amount}}) { __typename }
		}`)

	if len(payloads) != 1 {
		t.Fatalf("expected 1 payload, got %d", len(payloads))
	}
	if payloads[0].TypeVersion != 1 {
		t.Errorf("TypeVersion = %d, want 1", payloads[0].TypeVersion)
	}
}

// TestUnpinnedAndVersionOneShareAnIdentity follows from normalisation: because
// unpinned resolves to 1, the two describe the same model and must collapse into
// one payload rather than producing a duplicate.
func TestUnpinnedAndVersionOneShareAnIdentity(t *testing.T) {
	payloads, warnings := derive(t, `
		mutation A($ik: SafeString!, $ledgerIk: SafeString!, $amount: String!) {
		  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "t", parameters: {amount: $amount}}) { __typename }
		}
		mutation B($ik: SafeString!, $ledgerIk: SafeString!, $amount: String!) {
		  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "t", typeVersion: 1, parameters: {amount: $amount}}) { __typename }
		}`)

	if len(payloads) != 1 {
		t.Fatalf("expected 1 payload, got %d: %+v", len(payloads), payloads)
	}
	if payloads[0].SourceOp != "A" {
		t.Errorf("SourceOp = %q, want A: the first occurrence should win", payloads[0].SourceOp)
	}
	if len(warnings) != 0 {
		t.Errorf("identical parameters should not warn, got %v", warnings)
	}
}

// TestDuplicateIdentityWithDifferentParametersWarns covers the case where two
// operations claim the same identity but disagree. That means the .graphql is
// stale rather than that the input is invalid, so generation continues with the
// first and the caller is told.
func TestDuplicateIdentityWithDifferentParametersWarns(t *testing.T) {
	payloads, warnings := derive(t, `
		mutation A($ik: SafeString!, $ledgerIk: SafeString!, $amount: String!) {
		  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "t", parameters: {amount: $amount}}) { __typename }
		}
		mutation B($ik: SafeString!, $ledgerIk: SafeString!, $other: String!) {
		  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "t", parameters: {other: $other}}) { __typename }
		}`)

	if len(payloads) != 1 {
		t.Fatalf("expected 1 payload, got %d", len(payloads))
	}
	if payloads[0].Params[0].WireName != "amount" {
		t.Errorf("kept %q, want the first occurrence, amount", payloads[0].Params[0].WireName)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "different parameters") {
		t.Errorf("expected a warning about differing parameters, got %v", warnings)
	}
}

// TestDistinctVersionsAreDistinctPayloads covers identity being the
// (type, typeVersion) pair rather than the type alone.
//
// The parameter sets are what distinguish the two versions, so the test checks
// that they are derived correctly.
func TestDistinctVersionsAreDistinctPayloads(t *testing.T) {
	payloads, _ := derive(t, `
		mutation A($ik: SafeString!, $ledgerIk: SafeString!, $amount: String!) {
		  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "t", typeVersion: 1, parameters: {amount: $amount}}) { __typename }
		}
		mutation B($ik: SafeString!, $ledgerIk: SafeString!, $amount: String!, $fee: String!) {
		  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "t", typeVersion: 2, parameters: {amount: $amount, fee: $fee}}) { __typename }
		}`)

	if len(payloads) != 2 {
		t.Fatalf("expected 2 payloads, got %d", len(payloads))
	}

	want := []struct {
		goName  string
		version int
		params  []Param
	}{
		{
			goName:  "TV1Entry",
			version: 1,
			params: []Param{
				{WireName: "amount", FieldName: "Amount", GoType: "string", Kind: KindValue, Required: true},
			},
		},
		{
			goName:  "TV2Entry",
			version: 2,
			params: []Param{
				{WireName: "amount", FieldName: "Amount", GoType: "string", Kind: KindValue, Required: true},
				{WireName: "fee", FieldName: "Fee", GoType: "string", Kind: KindValue, Required: true},
			},
		},
	}

	for i, w := range want {
		got := payloads[i]

		if got.GoName != w.goName {
			t.Errorf("payload %d name = %q, want %q", i, got.GoName, w.goName)
		}
		if got.TypeVersion != w.version {
			t.Errorf("payload %d version = %d, want %d", i, got.TypeVersion, w.version)
		}
		if !slices.Equal(got.Params, w.params) {
			t.Errorf("payload %d (%s) parameters = %+v, want %+v", i, w.goName, got.Params, w.params)
		}
	}
}

// TestNonLiteralTypeVersionIsSkipped covers a case the specification does not
// settle. Identity comes from the integer literal, so a variable version leaves
// it undefined. Treating it as absent would pin the payload to version 1 even
// though the operation accepts any version, which would silently post the wrong
// version, so the operation is skipped with a warning instead.
func TestNonLiteralTypeVersionIsSkipped(t *testing.T) {
	payloads, warnings := derive(t, `
		mutation A($ik: SafeString!, $ledgerIk: SafeString!, $v: Int, $amount: String!) {
		  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "t", typeVersion: $v, parameters: {amount: $amount}}) { __typename }
		}`)

	if len(payloads) != 0 {
		t.Fatalf("expected the operation to be skipped, got %+v", payloads)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "non-literal typeVersion") {
		t.Errorf("expected a warning about the non-literal version, got %v", warnings)
	}
}

// TestNonVariableParametersAreSkipped covers the requirement that a parameter
// whose value the operation fixes is not something the caller supplies.
func TestNonVariableParametersAreSkipped(t *testing.T) {
	payloads, _ := derive(t, `
		mutation A($ik: SafeString!, $ledgerIk: SafeString!, $amount: String!) {
		  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "t", parameters: {amount: $amount, fixed: "no", alsoFixed: 42}}) { __typename }
		}`)

	if len(payloads) != 1 {
		t.Fatalf("expected 1 payload, got %d", len(payloads))
	}
	if len(payloads[0].Params) != 1 || payloads[0].Params[0].WireName != "amount" {
		t.Errorf("expected only the variable-bound parameter, got %+v", payloads[0].Params)
	}
}

// TestParametersBoundAsAWholeFallBackToUntyped covers an operation that binds its
// parameters to a single variable. The entry type does take parameters; the
// operation just does not say what they are, so the caller supplies raw JSON.
func TestParametersBoundAsAWholeFallBackToUntyped(t *testing.T) {
	payloads, _ := derive(t, `
		mutation A($ik: SafeString!, $ledgerIk: SafeString!, $parameters: JSON!) {
		  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "t", parameters: $parameters}) { __typename }
		}`)

	if len(payloads) != 1 {
		t.Fatalf("expected 1 payload, got %d", len(payloads))
	}
	if !payloads[0].Untyped {
		t.Error("expected the payload to fall back to untyped parameters")
	}
	if len(payloads[0].Params) != 0 {
		t.Errorf("expected no derived parameters, got %+v", payloads[0].Params)
	}
}

// TestEntryTypeWithNoParametersIsNotUntyped covers an entry type that takes no
// parameters, whether the operation omits the parameters field or gives an empty
// object.
//
// Both mean the same thing and must behave the same. Neither is the untyped
// fallback: that exists for parameters the generator could not type, and treating
// "there are none" as "I could not tell" hands the caller a raw JSON field there is
// nothing to put in.
func TestEntryTypeWithNoParametersIsNotUntyped(t *testing.T) {
	for name, src := range map[string]string{
		"parameters field omitted": `
			mutation A($ik: SafeString!, $ledgerIk: SafeString!) {
			  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "t"}) { __typename }
			}`,
		"parameters is an empty object": `
			mutation A($ik: SafeString!, $ledgerIk: SafeString!) {
			  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "t", parameters: {}}) { __typename }
			}`,
	} {
		t.Run(name, func(t *testing.T) {
			payloads, _ := derive(t, src)
			if len(payloads) != 1 {
				t.Fatalf("expected 1 payload, got %d", len(payloads))
			}
			if payloads[0].Untyped {
				t.Error("an entry type with no parameters should not get the raw parameters field")
			}
			if len(payloads[0].Params) != 0 {
				t.Errorf("expected no parameters, got %+v", payloads[0].Params)
			}

			// The two spellings have to produce identical source, or the generated
			// API would depend on how the operation happened to be written.
			source, err := Emit(payloads)
			if err != nil {
				t.Fatalf("Emit: %v", err)
			}
			if strings.Contains(string(source), "Parameters json.RawMessage") {
				t.Errorf("payload should have no Parameters field:\n%s", source)
			}
			if !strings.Contains(string(source), `entry.Set("parameters", params)`) {
				t.Errorf("expected an empty parameters object on the wire:\n%s", source)
			}
		})
	}
}

// TestParameterTypesComeFromVariableDefinitions covers the requirement that a
// parameter's type and required-ness are read from the variable definition rather
// than guessed from the field name.
func TestParameterTypesComeFromVariableDefinitions(t *testing.T) {
	payloads, _ := derive(t, `
		mutation A(
		  $ik: SafeString!
		  $ledgerIk: SafeString!
		  $required: String!
		  $optional: String
		  $count: Int!
		  $currency: CurrencyMatchInput
		  $tags: [String!]
		) {
		  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "t", parameters: {
		    required: $required, optional: $optional, count: $count, currency: $currency, tags: $tags
		  }}) { __typename }
		}`)

	if len(payloads) != 1 {
		t.Fatalf("expected 1 payload, got %d", len(payloads))
	}

	// Kind is what decides how the field is written, and it follows the shape of
	// the Go type rather than Required: a nullable list is a slice, not a
	// pointer, so treating "not required" as "pointer" would emit a call that
	// does not compile.
	want := []Param{
		{WireName: "required", FieldName: "Required", GoType: "string", Kind: KindValue, Required: true},
		{WireName: "optional", FieldName: "Optional", GoType: "*string", Kind: KindPointer, Required: false},
		{WireName: "count", FieldName: "Count", GoType: "int", Kind: KindValue, Required: true},
		// A non-scalar Schema type falls back to raw JSON, because the generated
		// payloads cannot import the package genqlient emitted it into.
		{WireName: "currency", FieldName: "Currency", GoType: "json.RawMessage", Kind: KindRaw, Required: false},
		{WireName: "tags", FieldName: "Tags2", GoType: "[]string", Kind: KindSlice, Required: false},
	}

	got := payloads[0].Params
	if len(got) != len(want) {
		t.Fatalf("expected %d parameters, got %d: %+v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("parameter %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestParameterOrderIsSourceOrder covers the requirement that parameters keep the
// order they appear in, rather than being sorted or grouped required-first. All
// four SDKs read the same document, so source order is the only order they can
// agree on without coordinating.
func TestParameterOrderIsSourceOrder(t *testing.T) {
	payloads, _ := derive(t, `
		mutation A($ik: SafeString!, $ledgerIk: SafeString!, $zulu: String!, $alpha: String, $mike: String!) {
		  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "t", parameters: {zulu: $zulu, alpha: $alpha, mike: $mike}}) { __typename }
		}`)

	var order []string
	for _, p := range payloads[0].Params {
		order = append(order, p.WireName)
	}
	want := []string{"zulu", "alpha", "mike"}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("parameter order = %v, want %v", order, want)
		}
	}
}

// TestCollidingFieldNamesStayDistinct covers the requirement that escaping never
// merges two parameters. Both user_id and userId reduce to UserId, so the second
// has to be renamed — while keeping its own wire name, so each still carries its
// own value.
func TestCollidingFieldNamesStayDistinct(t *testing.T) {
	payloads, warnings := derive(t, `
		mutation A($ik: SafeString!, $ledgerIk: SafeString!, $a: String!, $b: String!) {
		  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "t", parameters: {user_id: $a, userId: $b}}) { __typename }
		}`)

	params := payloads[0].Params
	if len(params) != 2 {
		t.Fatalf("expected 2 parameters, got %d", len(params))
	}
	if params[0].FieldName == params[1].FieldName {
		t.Fatalf("both parameters landed on %q; one value would overwrite the other", params[0].FieldName)
	}
	if params[0].FieldName != "UserId" || params[1].FieldName != "UserId2" {
		t.Errorf("field names = %q, %q; want UserId, UserId2 (first occurrence keeps the plain name)",
			params[0].FieldName, params[1].FieldName)
	}
	if params[0].WireName != "user_id" || params[1].WireName != "userId" {
		t.Errorf("wire names were altered: %q, %q", params[0].WireName, params[1].WireName)
	}
	if len(warnings) != 1 {
		t.Errorf("expected a warning about the rename, got %v", warnings)
	}
}

// TestParametersCannotShadowCommonFields covers the collision case that actually
// occurs in practice: an entry type with a parameter named "posted", which would
// otherwise land on the common Posted field and be sent as the entry's timestamp.
func TestParametersCannotShadowCommonFields(t *testing.T) {
	payloads, _ := derive(t, `
		mutation A($ik: SafeString!, $ledgerIk: SafeString!, $p: String!, $t: String!, $c: String!) {
		  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "t", parameters: {posted: $p, tags: $t, conditions: $c}}) { __typename }
		}`)

	for _, p := range payloads[0].Params {
		for _, reserved := range reservedFieldNames {
			if p.FieldName == reserved {
				t.Errorf("parameter %q was assigned the reserved field name %s", p.WireName, reserved)
			}
		}
	}
}

// TestEntryTypesCollapsingOntoOneNameIsAnError covers the one collision that
// cannot be resolved by suffixing. A payload's name has to depend only on its own
// identity, so renaming one here would rename it again whenever an unrelated
// entry type was added, breaking existing call sites for a purely additive
// Schema change.
func TestEntryTypesCollapsingOntoOneNameIsAnError(t *testing.T) {
	_, _, err := Derive([]*ast.Source{{Name: "t.graphql", Input: `
		mutation A($ik: SafeString!, $ledgerIk: SafeString!) {
		  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "user_funds"}) { __typename }
		}
		mutation B($ik: SafeString!, $ledgerIk: SafeString!) {
		  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "user-funds"}) { __typename }
		}`}}, scalars)

	if err == nil {
		t.Fatal("expected an error for two entry types producing one identifier")
	}
	if !strings.Contains(err.Error(), "same Go identifier") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestCommonFieldsAreFixed pins the requirement that the common-field set comes
// from LedgerEntryInput rather than from the source operation.
//
// This cannot be checked on the wire: an implementation that derived the set from
// the operation would still produce the right JSON for any fixture whose operation
// happens to bind those fields. The guarantee is about where the list comes from,
// so the list itself is what has to be asserted.
func TestCommonFieldsAreFixed(t *testing.T) {
	var got []string
	for _, f := range CommonFields {
		got = append(got, f.Name)
	}

	want := []string{"Ik", "LedgerIk", "Posted", "Description", "Tags", "Groups", "Conditions"}
	if len(got) != len(want) {
		t.Fatalf("common fields = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("common fields = %v, want %v", got, want)
		}
	}

	// lines cannot be combined with an entry that has a type, so a payload must
	// never offer it.
	for _, f := range CommonFields {
		if strings.EqualFold(f.Name, "lines") || strings.EqualFold(f.Wire, "lines") {
			t.Error("lines must not be a common field: it cannot be used with a typed entry")
		}
	}
}

// TestCommonFieldsAreEmittedRegardlessOfWhatTheOperationBinds is the same
// requirement from the other side: a payload derived from an operation that binds
// none of the optional entry fields still carries all of them.
//
// This is what keeps a CLI change to its bindings from moving a payload's surface.
func TestCommonFieldsAreEmittedRegardlessOfWhatTheOperationBinds(t *testing.T) {
	payloads, _ := derive(t, `
		mutation A($ik: SafeString!, $ledgerIk: SafeString!) {
		  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "t"}) { __typename }
		}`)

	source, err := Emit(payloads)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}

	for _, f := range CommonFields {
		if !strings.Contains(string(source), "\t"+f.Name+" "+f.Type) {
			t.Errorf("common field %s %s is missing from a payload whose operation binds nothing", f.Name, f.Type)
		}
	}
}

// TestCommonFieldsDoNotDriftFromReservedNames guards the pair of lists that have
// to agree. Adding a common field without reserving its name would silently
// reopen the collision class that TestParametersCannotShadowCommonFields covers.
func TestCommonFieldsDoNotDriftFromReservedNames(t *testing.T) {
	for _, f := range CommonFields {
		if !isReserved(f.Name) {
			t.Errorf("common field %s is not in reservedFieldNames, so a parameter could shadow it", f.Name)
		}
	}
	for _, method := range generatedMethods {
		if !isReserved(method) {
			t.Errorf("generated method %s is not in reservedFieldNames, so a field could collide with it", method)
		}
	}
}

// TestEmittedSourceCompilesForEveryScalarBinding guards the generated file's
// import block. A parameter bound to a JSON scalar renders as json.RawMessage,
// which needs encoding/json imported; without it the generated package would not
// build, and Emit's own gofmt pass does not catch a missing import.
func TestEmittedSourceCompilesForEveryScalarBinding(t *testing.T) {
	payloads, _ := derive(t, `
		mutation A($ik: SafeString!, $ledgerIk: SafeString!, $metadata: JSON!, $optional: JSON, $amount: String!) {
		  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "t", parameters: {
		    metadata: $metadata, optional: $optional, amount: $amount
		  }}) { __typename }
		}`)

	source, err := Emit(payloads)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}

	got := string(source)
	if !strings.Contains(got, "json.RawMessage") {
		t.Fatal("expected a json.RawMessage field")
	}
	if !strings.Contains(got, `"encoding/json"`) {
		t.Errorf("generated source uses json.RawMessage but does not import encoding/json:\n%s", got)
	}
}

// TestEmitReturnsNothingForNoPayloads keeps the SDK's own codegen a no-op, so
// that `make codegen` does not start writing an empty package.
func TestEmitReturnsNothingForNoPayloads(t *testing.T) {
	source, err := Emit(nil)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if source != nil {
		t.Errorf("expected no source, got:\n%s", source)
	}
}

// TestPayloadNamesCarryTheirVersion covers the backward-compatibility
// consequence that a name depends only on its own identity. Deriving v1 alone and
// deriving it alongside v2 must produce the same name, or adding a version would
// be a rename.
func TestPayloadNamesCarryTheirVersion(t *testing.T) {
	const v1 = `
		mutation A($ik: SafeString!, $ledgerIk: SafeString!, $amount: String!) {
		  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "t", typeVersion: 1, parameters: {amount: $amount}}) { __typename }
		}`
	const v2 = `
		mutation B($ik: SafeString!, $ledgerIk: SafeString!, $amount: String!, $fee: String!) {
		  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "t", typeVersion: 2, parameters: {amount: $amount, fee: $fee}}) { __typename }
		}`

	alone, _ := derive(t, v1)
	together, _ := derive(t, v1+v2)

	if alone[0].GoName != together[0].GoName {
		t.Errorf("adding version 2 renamed version 1 from %q to %q", alone[0].GoName, together[0].GoName)
	}
}

// TestNewOptionalParameterDoesNotRenameExistingFields covers the other additive
// change the specification guarantees: adding an optional parameter must not
// disturb the fields already there.
func TestNewOptionalParameterDoesNotRenameExistingFields(t *testing.T) {
	before, _ := derive(t, `
		mutation A($ik: SafeString!, $ledgerIk: SafeString!, $amount: String!) {
		  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "t", parameters: {amount: $amount}}) { __typename }
		}`)
	after, _ := derive(t, `
		mutation A($ik: SafeString!, $ledgerIk: SafeString!, $amount: String!, $memo: String) {
		  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "t", parameters: {amount: $amount, memo: $memo}}) { __typename }
		}`)

	if before[0].Params[0] != after[0].Params[0] {
		t.Errorf("existing parameter changed from %+v to %+v", before[0].Params[0], after[0].Params[0])
	}
	if before[0].GoName != after[0].GoName {
		t.Errorf("payload was renamed from %q to %q", before[0].GoName, after[0].GoName)
	}
}
