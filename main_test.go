package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Khan/genqlient/generate"

	"github.com/fragment-dev/fragment-go/v4/internal/typedentries"
)

func TestGoTypeOfBinding(t *testing.T) {
	cases := map[string]string{
		"string":                   "string",
		"int":                      "int",
		"encoding/json.RawMessage": "json.RawMessage",
		"time.Time":                "time.Time",
		"github.com/x/y/pkg.Thing": "pkg.Thing",
	}
	for binding, want := range cases {
		if got := goTypeOfBinding(binding); got != want {
			t.Errorf("goTypeOfBinding(%q) = %q, want %q", binding, got, want)
		}
	}
}

// TestScalarBindingsCoversTheRealBindings runs the derivation's scalar map through
// the table the CLI actually passes, rather than a hand-written stand-in.
//
// Every scalar bug found so far came from a test map that supplied something the
// real table does not, so this asserts against the real one: each of Fragment's
// scalars must resolve to a Go type the generated package can name, meaning either
// a builtin or something qualified by an import the generated file has.
func TestScalarBindingsCoversTheRealBindings(t *testing.T) {
	bindings := codegenBindings()
	scalars := scalarBindings(bindings)

	if len(scalars) != len(bindings) {
		t.Errorf("scalarBindings dropped entries: got %d, want %d", len(scalars), len(bindings))
	}

	// json.RawMessage is the one qualified type the generated file imports; any
	// other qualified name would not compile there.
	allowed := map[string]bool{
		"string": true, "int": true, "int64": true, "float64": true, "bool": true,
		"json.RawMessage": true,
	}
	for scalar, goType := range scalars {
		if !allowed[goType] {
			t.Errorf("scalar %s maps to %s, which the generated package cannot name; "+
				"either add its import to typedentries.Emit or bind the scalar differently",
				scalar, goType)
		}
	}
}

// TestReadOperationsExpandsGlobs covers the input handling. genqlient globs these
// values, so reading them literally would break a pattern that worked before typed
// payloads existed.
func TestReadOperationsExpandsGlobs(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.graphql", "b.graphql", "ignored.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("# "+name+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("glob", func(t *testing.T) {
		sources, err := readOperations([]string{filepath.Join(dir, "*.graphql")})
		if err != nil {
			t.Fatalf("readOperations: %v", err)
		}
		if len(sources) != 2 {
			t.Fatalf("expected 2 sources, got %d", len(sources))
		}
		// Named by the file they came from, not by the pattern, so a parse error
		// points at something openable.
		for _, src := range sources {
			if filepath.Ext(src.Name) != ".graphql" {
				t.Errorf("source name %q should be a real path", src.Name)
			}
		}
	})

	t.Run("literal path", func(t *testing.T) {
		sources, err := readOperations([]string{filepath.Join(dir, "a.graphql")})
		if err != nil {
			t.Fatalf("readOperations: %v", err)
		}
		if len(sources) != 1 {
			t.Fatalf("expected 1 source, got %d", len(sources))
		}
	})

	t.Run("pattern matching nothing", func(t *testing.T) {
		// Not an error: genqlient has already validated the inputs by this point.
		sources, err := readOperations([]string{filepath.Join(dir, "*.absent")})
		if err != nil {
			t.Fatalf("readOperations: %v", err)
		}
		if len(sources) != 0 {
			t.Fatalf("expected no sources, got %d", len(sources))
		}
	})
}

// TestAddTypedPayloadsWritesNothingWithoutTypedEntries keeps the SDK's own codegen
// a no-op. queries.graphql binds its entry type as a variable, so it must derive
// nothing, and `make codegen` must not start writing an empty package.
func TestAddTypedPayloadsWritesNothingWithoutTypedEntries(t *testing.T) {
	generated := map[string][]byte{}
	args := cliArgs{
		Inputs: []string{"queries/queries.graphql"},
		Output: "queries/queries.go",
	}

	if err := addTypedPayloads(generated, args, codegenBindings()); err != nil {
		t.Fatalf("addTypedPayloads: %v", err)
	}
	if len(generated) != 0 {
		t.Errorf("expected no files, got %v", keys(generated))
	}
}

// TestAddTypedPayloadsWritesToNestedPackage checks the output path, which is what
// puts the payloads in a package of their own and so makes unkeyed literals a
// compile error for callers.
func TestAddTypedPayloadsWritesToNestedPackage(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "ops.graphql")
	if err := os.WriteFile(input, []byte(`
		mutation PostThing($ik: SafeString!, $ledgerIk: SafeString!, $amount: String!) {
		  addLedgerEntry(ik: $ik, entry: {ledger: {ik: $ledgerIk}, type: "thing", parameters: {amount: $amount}}) { __typename }
		}`), 0o644); err != nil {
		t.Fatal(err)
	}

	generated := map[string][]byte{}
	args := cliArgs{Inputs: []string{input}, Output: filepath.Join(dir, "gen", "client.go")}

	if err := addTypedPayloads(generated, args, codegenBindings()); err != nil {
		t.Fatalf("addTypedPayloads: %v", err)
	}

	want := filepath.Join(dir, "gen", typedentries.PackageName, typedentries.PackageName+".go")
	source, ok := generated[want]
	if !ok {
		t.Fatalf("expected a file at %s, got %v", want, keys(generated))
	}
	if len(source) == 0 {
		t.Error("generated file is empty")
	}
}

func keys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// codegenBindings returns the binding table main uses, so tests exercise the same
// values as the CLI.
func codegenBindings() map[string]*generate.TypeBinding {
	return newCodegenConfig(cliArgs{}, "").Bindings
}
