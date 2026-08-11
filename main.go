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
	sources := make([]*ast.Source, 0, len(args.Inputs))
	for _, input := range args.Inputs {
		content, err := os.ReadFile(input)
		if err != nil {
			return err
		}
		sources = append(sources, &ast.Source{Name: input, Input: string(content)})
	}

	scalars := make(map[string]string, len(bindings))
	for name, binding := range bindings {
		scalars[name] = goTypeOfBinding(binding.Type)
	}

	payloads, warnings, err := typedentries.Derive(sources, scalars)
	for _, w := range warnings {
		fmt.Println("warning: " + w)
	}
	if err != nil {
		return err
	}

	source, err := typedentries.Emit(payloads)
	if err != nil || source == nil {
		return err
	}

	path := filepath.Join(filepath.Dir(args.Output), typedentries.PackageName, typedentries.PackageName+".go")
	generated[path] = source
	fmt.Printf("Derived %d typed Ledger Entry payload(s) into %s.\n", len(payloads), path)
	return nil
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

	codegenConfig := &generate.Config{
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
	return
}
