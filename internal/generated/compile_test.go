package generated

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vektah/gqlparser/v2/ast"

	"github.com/fragment-dev/fragment-go/v4/internal/typedentries"
)

// scalars mirrors the generator's real binding table: Fragment's own scalars
// only. The built-ins are deliberately absent, because the derivation resolves
// those itself and supplying them here would hide a regression in it.
var scalars = map[string]string{
	"AlphaNumericString":  "string",
	"Date":                "string",
	"DateTime":            "string",
	"FirstMoment":         "string",
	"Int64":               "string",
	"Int96":               "string",
	"JSON":                "json.RawMessage",
	"JSONObject":          "json.RawMessage",
	"LastMoment":          "string",
	"Parameters":          "json.RawMessage",
	"ParameterizedString": "string",
	"Period":              "string",
	"PeriodFilter":        "string",
	"SafeString":          "string",
	"UTCOffset":           "string",
}

// TestGeneratedSourceCompiles type-checks the generator's output for inputs
// designed to break it.
//
// Nothing else does this. The committed fixtures are compiled because they are
// ordinary packages in this module, but they only cover the six shapes the shared
// specification happens to exercise — all-required string parameters. Every
// codegen defect found so far produced source that gofmt accepted and the Go
// compiler rejected, so a test that only derives or only emits cannot catch them:
// the generated file has to be built.
func TestGeneratedSourceCompiles(t *testing.T) {
	cases := map[string]string{
		// A nullable list is a slice, not a pointer. Dispatching the setter on
		// "not required" rather than on the Go type's shape passes a slice to a
		// pointer helper.
		"optional list parameter": `
			mutation M($ik: SafeString!, $ledgerIk: SafeString!, $tags: [String!], $nested: [[String]]) {
			  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "t", parameters: {tags: $tags, nested: $nested}}) { __typename }
			}`,

		// An enum or input object is emitted into the package genqlient
		// generated for these operations, which the payloads cannot import.
		"non-scalar parameter": `
			mutation M($ik: SafeString!, $ledgerIk: SafeString!, $g: EntryGroupMatchInput!, $c: CurrencyMatchInput) {
			  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "t", parameters: {g: $g, c: $c}}) { __typename }
			}`,

		// A field may not share a name with a method the payload declares.
		"parameter named like a generated method": `
			mutation M($ik: SafeString!, $ledgerIk: SafeString!, $a: String!, $b: String!, $c: String!) {
			  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "t", parameters: {
			    marshalJSON: $a, fragment_batch_entry: $b, parameters: $c
			  }}) { __typename }
			}`,

		// Every common field name is a potential collision.
		"parameters named like every common field": `
			mutation M($ik: SafeString!, $ledgerIk: SafeString!, $a: String!, $b: String!, $c: String!, $d: String!, $e: String!, $f: String!, $g: String!) {
			  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "t", parameters: {
			    ik: $a, ledgerIk: $b, posted: $c, description: $d, tags: $e, groups: $f, conditions: $g
			  }}) { __typename }
			}`,

		// The receiver in a generated MarshalJSON is named e.
		"parameter named like the receiver": `
			mutation M($ik: SafeString!, $ledgerIk: SafeString!, $e: String!) {
			  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "t", parameters: {e: $e}}) { __typename }
			}`,

		// A JSON scalar renders as json.RawMessage and needs the import.
		"json scalar parameters": `
			mutation M($ik: SafeString!, $ledgerIk: SafeString!, $meta: JSON!, $opt: JSONObject) {
			  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "t", parameters: {meta: $meta, opt: $opt}}) { __typename }
			}`,

		// Parameters bound as a whole: the payload carries a raw Parameters field
		// instead of derived ones.
		"untyped parameters": `
			mutation M($ik: SafeString!, $ledgerIk: SafeString!, $parameters: JSON!) {
			  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "t", parameters: $parameters}) { __typename }
			}`,

		// An entry type with no parameters at all, spelled both ways. Neither gets
		// a Parameters field, so both compile a payload whose only fields are the
		// common ones.
		"no parameters": `
			mutation A($ik: SafeString!, $ledgerIk: SafeString!) {
			  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "omitted"}) { __typename }
			}
			mutation B($ik: SafeString!, $ledgerIk: SafeString!) {
			  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "empty", parameters: {}}) { __typename }
			}`,

		// Entry types are free-form Schema strings and reach identifiers.
		"awkward entry type names": `
			mutation A($ik: SafeString!, $ledgerIk: SafeString!) {
			  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "user:funds"}) { __typename }
			}
			mutation B($ik: SafeString!, $ledgerIk: SafeString!) {
			  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "123-numeric"}) { __typename }
			}
			mutation C($ik: SafeString!, $ledgerIk: SafeString!) {
			  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "café-entry"}) { __typename }
			}
			mutation D($ik: SafeString!, $ledgerIk: SafeString!) {
			  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "amount+fee"}) { __typename }
			}`,

		// Parameter names cannot be as hostile as entry types: GraphQL requires
		// them to be Names, so they are always [_A-Za-z][_0-9A-Za-z]*. These are
		// the awkward shapes that remain legal.
		"awkward parameter names": `
			mutation M($ik: SafeString!, $ledgerIk: SafeString!, $a: String!, $b: String!, $c: String!, $d: String!) {
			  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "t", parameters: {
			    _leading: $a, __dunder: $b, x: $c, _: $d
			  }}) { __typename }
			}`,

		// A high version number and several versions of one type together.
		"many versions": `
			mutation A($ik: SafeString!, $ledgerIk: SafeString!, $a: String!) {
			  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "t", typeVersion: 1, parameters: {a: $a}}) { __typename }
			}
			mutation B($ik: SafeString!, $ledgerIk: SafeString!, $a: String!) {
			  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "t", typeVersion: 9999, parameters: {a: $a}}) { __typename }
			}`,
	}

	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			payloads, warnings, err := typedentries.Derive(
				[]*ast.Source{{Name: name + ".graphql", Input: src}}, scalars)
			if err != nil {
				t.Fatalf("Derive: %v", err)
			}
			for _, w := range warnings {
				t.Logf("warning: %s", w)
			}
			if len(payloads) == 0 {
				t.Fatal("derived no payloads, so nothing was compiled")
			}

			source, err := typedentries.Emit(payloads)
			if err != nil {
				t.Fatalf("Emit: %v", err)
			}

			assertCompiles(t, source)
		})
	}
}

// assertCompiles builds the generated source as a package inside this module,
// which is the only way it can resolve the internal imports the payloads use.
func assertCompiles(t *testing.T, source []byte) {
	t.Helper()

	dir, err := os.MkdirTemp(".", "compilecheck-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	path := filepath.Join(dir, typedentries.PackageName+".go")
	if err := os.WriteFile(path, source, 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("go", "build", "./"+dir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("generated source does not compile:\n%s\n--- source ---\n%s",
			indent(string(output)), numbered(string(source)))
	}
}

func indent(s string) string {
	return "\t" + strings.ReplaceAll(strings.TrimRight(s, "\n"), "\n", "\n\t")
}

// numbered prefixes each line with its number, so a compiler error's line
// reference can be found in the failure output.
func numbered(s string) string {
	var b strings.Builder
	for i, line := range strings.Split(s, "\n") {
		fmt.Fprintf(&b, "%4d| %s\n", i+1, line)
	}
	return b.String()
}
