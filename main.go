package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Khan/genqlient/generate"
	"github.com/alexflint/go-arg"
	"github.com/bmatcuk/doublestar/v4"
	"github.com/vektah/gqlparser/v2/ast"

	"github.com/fragment-dev/fragment-go/v4/internal/typedentries"
)

type cliArgs struct {
	PackageName string   `arg:"--package" default:"main" help:"The package name to use for the generated client."`
	Inputs      []string `arg:"-i,--input,separate" help:"The input files to generate a client from."`
	Output      string   `arg:"-o,--output" help:"The output file to write the generated client to."`
}

func downloadSchemaToTempFile() (string, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.us-west-2.fragment.dev/schema.graphql", nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{}
	getResp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer getResp.Body.Close()

	tempFile, err := os.CreateTemp("", "schema.graphql")
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	_, err = io.Copy(tempFile, getResp.Body)
	if err != nil {
		return "", err
	}

	return tempFile.Name(), nil
}

// addTypedPayloads derives a typed payload per Ledger Entry type from the same
// operations genqlient just generated from, and adds the result to the set of
// files to write.
//
// The payloads go into a package of their own beside the generated client
// rather than into the client file itself, so that genqlient keeps sole
// ownership of --output and so that callers can never write an unkeyed literal
// of a payload. Nothing is added when the operations contain no typed entries.
func addTypedPayloads(generated map[string][]byte, args cliArgs, bindings map[string]*generate.TypeBinding) error {
	sources, err := readOperations(args.Inputs)
	if err != nil {
		return err
	}

	payloads, warnings, err := typedentries.Derive(sources, scalarBindings(bindings))
	for _, w := range warnings {
		fmt.Fprintln(os.Stderr, "warning: "+w)
	}
	if err != nil {
		return err
	}
	if len(payloads) == 0 {
		return nil
	}

	source, err := typedentries.Emit(payloads)
	if err != nil {
		return err
	}

	path := filepath.Join(filepath.Dir(args.Output), typedentries.PackageName, typedentries.PackageName+".go")
	generated[path] = source
	fmt.Fprintf(os.Stderr, "Derived %d typed Ledger Entry payload(s) into %s.\n", len(payloads), path)
	return nil
}

// readOperations loads the operation documents named by --input.
//
// Each input is expanded as a glob, because that is what genqlient does with the
// same values: reading them literally would break a pattern such as
// 'queries/*.graphql', which worked before typed payloads existed.
func readOperations(inputs []string) ([]*ast.Source, error) {
	var sources []*ast.Source
	for _, input := range inputs {
		matches, err := doublestar.FilepathGlob(input)
		if err != nil {
			return nil, fmt.Errorf("expanding %s: %w", input, err)
		}
		if len(matches) == 0 {
			// Not necessarily a mistake: genqlient has already validated the
			// inputs by this point, so an empty expansion here means the
			// pattern matched nothing that still exists.
			continue
		}
		for _, match := range matches {
			content, err := os.ReadFile(match)
			if err != nil {
				return nil, err
			}
			sources = append(sources, &ast.Source{Name: match, Input: string(content)})
		}
	}
	return sources, nil
}

// scalarBindings converts genqlient's binding table into the GraphQL-to-Go scalar
// map the derivation needs.
func scalarBindings(bindings map[string]*generate.TypeBinding) map[string]string {
	scalars := make(map[string]string, len(bindings))
	for name, binding := range bindings {
		scalars[name] = goTypeOfBinding(binding.Type)
	}
	return scalars
}

// goTypeOfBinding turns a genqlient binding into the type name that appears in
// generated source. Qualified bindings such as "encoding/json.RawMessage" are
// written as the package's last element plus the type.
func goTypeOfBinding(binding string) string {
	i := strings.LastIndex(binding, "/")
	return binding[i+1:]
}

func main() {
	var args cliArgs
	arg.MustParse(&args)

	if len(args.Inputs) == 0 {
		fmt.Println("No input files provided.")
		os.Exit(1)
	}

	tempDir, err := os.MkdirTemp("", "*")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer os.RemoveAll(tempDir)

	schemaFile, err := downloadSchemaToTempFile()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	codegenConfig := newCodegenConfig(args, schemaFile)

	generated, err := generate.Generate(codegenConfig)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err := addTypedPayloads(generated, args, codegenConfig.Bindings); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	for filename, content := range generated {
		err = os.MkdirAll(filepath.Dir(filename), 0o755)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		err = os.WriteFile(filename, content, 0o644)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	fmt.Println("Successfully generated client to " + args.Output + ".")
}

// newCodegenConfig builds the genqlient configuration. It is a function so that
// tests can read the same binding table the CLI uses rather than a copy of it,
// since a stand-in table is what hid two scalar bugs.
func newCodegenConfig(args cliArgs, schemaFile string) *generate.Config {
	return &generate.Config{
		Schema:      []string{schemaFile},
		Operations:  args.Inputs,
		ContextType: "context.Context",
		Bindings: map[string]*generate.TypeBinding{
			"AlphaNumericString":  {Type: "string"},
			"Date":                {Type: "string"},
			"DateTime":            {Type: "string"},
			"FirstMoment":         {Type: "string"},
			"Int64":               {Type: "string"},
			"Int96":               {Type: "string"},
			"JSON":                {Type: "encoding/json.RawMessage"},
			"JSONObject":          {Type: "encoding/json.RawMessage"},
			"LastMoment":          {Type: "string"},
			"Parameters":          {Type: "encoding/json.RawMessage"},
			"ParameterizedString": {Type: "string"},
			"Period":              {Type: "string"},
			"PeriodFilter":        {Type: "string"},
			"SafeString":          {Type: "string"},
			"UTCOffset":           {Type: "string"},
		},
		Optional:  "pointer",
		Package:   args.PackageName,
		Generated: args.Output,
	}
}
