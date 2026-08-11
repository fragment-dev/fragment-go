package conformance

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestUnkeyedLiteralDoesNotCompile checks the one requirement the specification
// singles out for Go: parameters must be supplied by name.
//
// Python, Node and Ruby get this from the language. Go does not — an unkeyed
// struct literal is legal, and reordering two parameters of the same type there
// swaps their values without any compile error. The generated payloads defend
// against it with a leading unexported `_ struct{}` field, which makes an
// unkeyed literal illegal.
//
// The test compiles a throwaway program in a package of its own, which is the
// only way to check this: Go permits positional assignment to an unexported
// field from inside the declaring package, so an assertion written in
// typed_payloads would pass while telling us nothing. That is also why the
// generator emits payloads into their own package rather than into the caller's.
func TestUnkeyedLiteralDoesNotCompile(t *testing.T) {
	const program = `package main

import f001 "github.com/fragment-dev/fragment-go/v4/internal/conformance/f001/typed_payloads"

func main() {
	_ = f001.AuthCaptureV1Entry{%s}
}
`

	cases := []struct {
		name        string
		fields      string
		wantCompile bool
	}{
		{
			name:        "keyed literal compiles",
			fields:      `Ik: "ik-1", LedgerIk: "prod", UserId: "u", CaptureAmount: "1"`,
			wantCompile: true,
		},
		{
			name:        "unkeyed literal is rejected",
			fields:      `struct{}{}, "ik-1", "prod", nil, nil, nil, nil, nil, "u", "1"`,
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
			// The scratch program has to sit inside the module: the payloads are
			// under internal/, so a program anywhere else cannot import them.
			dir, err := os.MkdirTemp(".", "keyedcheck-")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.RemoveAll(dir) })

			source := filepath.Join(dir, "main.go")
			if err := os.WriteFile(source, []byte(strings.Replace(program, "%s", tc.fields, 1)), 0o644); err != nil {
				t.Fatal(err)
			}

			// Runs in the test's own directory, which is inside the module.
			cmd := exec.Command("go", "build", "-o", filepath.Join(dir, "out"), source)
			output, err := cmd.CombinedOutput()

			switch {
			case tc.wantCompile && err != nil:
				t.Errorf("expected this to compile, but it failed:\n%s", output)
			case !tc.wantCompile && err == nil:
				t.Error("expected a compile error, but the program built. Payloads are no longer protected against positional construction")
			case !tc.wantCompile:
				if !strings.Contains(string(output), "unexported field") &&
					!strings.Contains(string(output), "too few values") {
					t.Errorf("compile failed for an unexpected reason:\n%s", output)
				}
			}
		})
	}
}
