// Package conformance checks the generated typed payloads against the shared
// SDK specification's fixtures.
//
// The specification asks each SDK for a runner over the shared fixtures. Go
// cannot do that in one process: the payloads are generated source and have to
// be compiled before anything can construct them. So the fixtures' generated
// output is committed here instead, and two tests cover between them what a
// runner would.
//
//	generate_test.go  regenerates each fixture and compares it to the committed
//	                  file, so any change to a generated identifier or signature
//	                  shows up as a reviewable diff
//	wire_test.go      constructs each fixture's batch from the committed types
//	                  and compares the JSON to the fixture's expected.json
//
// The committed files are therefore both the snapshot and the input to the wire
// test. Run `go test ./internal/conformance -update` to refresh them.
package conformance

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/vektah/gqlparser/v2/ast"

	"github.com/fragment-dev/fragment-go/v4/internal/typedentries"
)

var update = flag.Bool("update", false, "rewrite the committed generated payloads")

// fixtures maps each shared fixture to the directory its generated payloads are
// committed under.
var fixtures = map[string]string{
	"001-basic":          "f001",
	"002-type-versions":  "f002",
	"003-reserved-names": "f003",
	"004-param-order":    "f004",
	"005-unset-omitted":  "f005",
	"006-non-ascii":      "f006",
}

// testScalars stands in for the generator's binding table, which lists only
// Fragment's own scalars. String is left out on purpose: it is a built-in that
// the derivation resolves itself, and supplying it here would hide a regression
// in that resolution.
var testScalars = map[string]string{
	"SafeString": "string",
	"DateTime":   "string",
}

func generatedPath(dir string) string {
	return filepath.Join(dir, typedentries.PackageName, typedentries.PackageName+".go")
}

// generate derives and emits a fixture's payloads.
func generate(t *testing.T, fixture string) []byte {
	t.Helper()

	path := filepath.Join("testdata", fixture+".graphql")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	payloads, warnings, err := typedentries.Derive(
		[]*ast.Source{{Name: path, Input: string(content)}}, testScalars)
	if err != nil {
		t.Fatalf("deriving %s: %v", fixture, err)
	}
	for _, w := range warnings {
		t.Logf("warning: %s", w)
	}

	source, err := typedentries.Emit(payloads)
	if err != nil {
		t.Fatalf("emitting %s: %v", fixture, err)
	}
	if source == nil {
		t.Fatalf("%s derived no payloads", fixture)
	}
	return source
}

// TestGeneratedPayloadsAreCurrent is the snapshot test the specification
// requires in place of a shared fixture for backward compatibility: generated
// identifiers and signatures cannot change without a diff here.
func TestGeneratedPayloadsAreCurrent(t *testing.T) {
	for fixture, dir := range fixtures {
		t.Run(fixture, func(t *testing.T) {
			want := generate(t, fixture)
			path := generatedPath(dir)

			if *update {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, want, 0o644); err != nil {
					t.Fatal(err)
				}
				t.Logf("wrote %s", path)
				return
			}

			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v\nrun `go test ./internal/conformance -update` to create it", path, err)
			}
			if string(got) != string(want) {
				t.Errorf("%s is stale. Review the change, then run `go test ./internal/conformance -update`", path)
			}
		})
	}
}
