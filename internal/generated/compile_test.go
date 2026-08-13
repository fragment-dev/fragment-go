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

		// A runtime entry type binds its lines, so the payload gains a Lines field
		// that templated entry types do not have.
		"runtime entry with lines": `
			mutation M($ik: SafeString!, $ledgerIk: SafeString!, $lines: [LedgerLineInput!]!) {
			  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "t", lines: $lines}) { __typename }
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

			if output, err := buildInModule(t, typedentries.PackageName+".go", source); err != nil {
				t.Errorf("generated source does not compile:\n%s\n--- source ---\n%s",
					indent(output), numbered(string(source)))
			}
		})
	}
}

// buildInModule writes source into a throwaway package inside this module, builds
// it, and returns the compiler's output.
//
// Inside the module because everything under test lives under internal/, which
// nothing outside the module may import. The -o keeps the binary of a main package
// out of the working tree; it is accepted and ignored for a non-main package, which
// is still type-checked.
func buildInModule(t *testing.T, filename string, source []byte) (string, error) {
	t.Helper()

	dir, err := os.MkdirTemp(".", "compilecheck-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	if err := os.WriteFile(filepath.Join(dir, filename), source, 0o644); err != nil {
		t.Fatal(err)
	}

	output, err := exec.Command("go", "build", "-o", filepath.Join(dir, "out"), "./"+dir).CombinedOutput()
	return string(output), err
}

// TestUnkeyedLiteralDoesNotCompile checks that parameters can only be supplied by
// name.
//
// An unkeyed struct literal is legal Go, and in one reordering two parameters of
// the same type swaps their values with no compile error and nothing at runtime to
// notice. The generated payloads block it with a leading unexported `_ struct{}`
// field, which makes an unkeyed literal illegal.
//
// The program is built in a package of its own, which is the only way to check
// this: Go permits positional assignment to an unexported field from inside the
// declaring package, so an assertion written in typed_payloads would pass while
// telling us nothing. That is also why the generator emits payloads into a package
// of their own rather than into the one genqlient wrote.
func TestUnkeyedLiteralDoesNotCompile(t *testing.T) {
	const program = `package main

import cli "github.com/fragment-dev/fragment-go/v4/internal/generated/cli/typed_payloads"

func main() {
	_ = cli.CardSettleV1Entry{%s}
}
`

	cases := []struct {
		name        string
		fields      string
		wantCompile bool
	}{
		{
			name:        "keyed literal compiles",
			fields:      `Ik: "ik-1", LedgerIk: "prod", UserId: "u", OrderId: "o", Currency: "USD", Amount: "1"`,
			wantCompile: true,
		},
		{
			name:        "unkeyed literal is rejected",
			fields:      `struct{}{}, "ik-1", "prod", nil, nil, nil, nil, nil, "u", "o", "USD", "1"`,
			wantCompile: false,
		},
		{
			name: "partial unkeyed literal is rejected",
			// Go rejects an unkeyed literal that omits fields on its own, but
			// this also confirms the caller cannot get partway there.
			fields:      `struct{}{}, "ik-1", "prod"`,
			wantCompile: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := strings.Replace(program, "%s", tc.fields, 1)
			output, err := buildInModule(t, "main.go", []byte(source))

			switch {
			case tc.wantCompile && err != nil:
				t.Errorf("expected this to compile, but it failed:\n%s", indent(output))
			case !tc.wantCompile && err == nil:
				t.Error("expected a compile error, but the program built. Payloads are no longer protected against positional construction")
			case !tc.wantCompile:
				if !strings.Contains(output, "unexported field") &&
					!strings.Contains(output, "too few values") {
					t.Errorf("compile failed for an unexpected reason:\n%s", indent(output))
				}
			}
		})
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
