package typedentries

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/vektah/gqlparser/v2/ast"
)

// Generated payloads are source, so nothing can construct one until it has been
// compiled. That rules out a test that generates and then exercises in one pass,
// so each fixture's output is committed under internal/generated instead and two
// tests split the work:
//
//	this file                        regenerates each fixture and compares it to
//	                                 the committed file, so a change to any
//	                                 generated identifier or signature shows up as
//	                                 a reviewable diff
//	internal/generated/wire_test.go  builds batches from the committed types and
//	                                 asserts the exact JSON they produce
//
// Committing the output has a second benefit: those packages are ordinary packages
// in this module, so `go build ./...` type-checks the generator's product.
//
// This lives here rather than in internal/generated on purpose. That package's
// tests import the generated payloads, so it cannot compile while they are stale —
// which is exactly when the files need rewriting. Keeping the writer in a package
// that does not import its own output means `-update` always works.
//
// Run `go test ./internal/typedentries -update` to refresh them.

var update = flag.Bool("update", false, "rewrite the committed generated payloads")

// fixtureDir is where the operation documents and the committed output live,
// relative to this package.
const fixtureDir = "../generated"

// fixtures maps each operation document to the directory its generated payloads
// are committed under.
var fixtures = map[string]string{
	// Real Fragment CLI output: the only input shape customers actually have.
	"cli": "cli",
	// Hand-written awkward cases the CLI does not produce.
	"edge": "edge",
}

func generatedPath(dir string) string {
	return filepath.Join(fixtureDir, dir, PackageName, PackageName+".go")
}

// generateFixture derives and emits a fixture's payloads.
func generateFixture(t *testing.T, fixture string) []byte {
	t.Helper()

	path := filepath.Join(fixtureDir, "testdata", fixture+".graphql")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	payloads, warnings, err := Derive(
		[]*ast.Source{{Name: path, Input: string(content)}}, scalars)
	if err != nil {
		t.Fatalf("deriving %s: %v", fixture, err)
	}
	for _, w := range warnings {
		t.Logf("warning: %s", w)
	}

	source, err := Emit(payloads)
	if err != nil {
		t.Fatalf("emitting %s: %v", fixture, err)
	}
	if source == nil {
		t.Fatalf("%s derived no payloads", fixture)
	}
	return source
}

// TestGeneratedPayloadsAreCurrent is the snapshot test: generated identifiers and
// signatures cannot change without a diff here.
//
// That matters because callers write those identifiers by hand. An accidental
// rename is a breaking change for every call site, and nothing else in the test
// suite would notice.
func TestGeneratedPayloadsAreCurrent(t *testing.T) {
	for fixture, dir := range fixtures {
		t.Run(fixture, func(t *testing.T) {
			want := generateFixture(t, fixture)
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
				t.Fatalf("reading %s: %v\nrun `go test ./internal/typedentries -update` to create it", path, err)
			}
			if string(got) != string(want) {
				t.Errorf("%s is stale. Review the change, then run `go test ./internal/typedentries -update`", path)
			}
		})
	}
}
