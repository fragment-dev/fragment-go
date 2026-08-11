package typedentries

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/vektah/gqlparser/v2/ast"
)

// The shared specification asks each SDK for a runner over its fixtures. Go
// cannot do that in one process: the payloads are generated source and have to
// be compiled before anything can construct them. So each fixture's generated
// output is committed under internal/conformance, and two tests cover between
// them what a runner would.
//
//	this file                         regenerates each fixture and compares it to
//	                                  the committed file, so any change to a
//	                                  generated identifier or signature shows up
//	                                  as a reviewable diff
//	internal/conformance/wire_test.go constructs each fixture's batch from the
//	                                  committed types and compares the JSON to
//	                                  the fixture's expected.json
//
// This lives here rather than in internal/conformance on purpose. That package's
// wire test imports the generated payloads, so it cannot compile while they are
// stale — which is exactly when the files need rewriting. Keeping the writer in
// a package that does not import its own output means `-update` always works.
//
// Run `go test ./internal/typedentries -update` to refresh them.

var update = flag.Bool("update", false, "rewrite the committed generated payloads")

// fixtureDir is where the shared fixtures and the committed output live,
// relative to this package.
const fixtureDir = "../conformance"

// fixtures maps each shared fixture to the directory its generated payloads are
// committed under.
var fixtures = map[string]string{
	"001-basic":          "f001",
	"002-type-versions":  "f002",
	"003-reserved-names": "f003",
	"004-param-order":    "f004",
	"005-unset-omitted":  "f005",
	"006-non-ascii":      "f006",
	// Not a shared fixture: real Fragment CLI output, kept because it is the only
	// input shape customers actually have. It also exercises what the shared
	// fixtures cannot — an operation that binds tags, groups and conditions as
	// entry-level variables, which must not change the payload's field set.
	"007-cli-output": "f007",
	// Not a shared fixture: the untyped-parameters fallback, which the shared
	// fixtures never exercise.
	"008-untyped-parameters": "f008",
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

// TestGeneratedPayloadsAreCurrent is the snapshot test the specification
// requires in place of a shared fixture for backward compatibility: generated
// identifiers and signatures cannot change without a diff here.
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
