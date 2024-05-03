package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/Khan/genqlient/generate"
	"github.com/alexflint/go-arg"
)

type cliArgs struct {
	PackageName string   `arg:"--package" default:"main" help:"The package name to use for the generated client."`
	Inputs      []string `arg:"-i,--input,separate" help:"The input files to generate a client from."`
	Output      string   `arg:"-o,--output" help:"The output file to write the generated client to."`
}

func downloadSchemaToTempFile() (string, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.fragment.dev/schema.graphql", nil)
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
		Schema:       []string{schemaFile},
		ContextType:  "github.com/fragment-dev/fragment-go/auth.AuthenticatedContext",
		ClientGetter: "github.com/fragment-dev/fragment-go/client.NewClient",
		Operations:   args.Inputs,
		Bindings: map[string]*generate.TypeBinding{
			"AlphaNumericString":  {Type: "string"},
			"Date":                {Type: "string"},
			"DateTime":            {Type: "string"},
			"Int64":               {Type: "string"},
			"Int96":               {Type: "string"},
			"JSON":                {Type: "encoding/json.RawMessage"},
			"JSONObject":          {Type: "encoding/json.RawMessage"},
			"LastMoment":          {Type: "string"},
			"ParameterizedString": {Type: "string"},
			"Period":              {Type: "string"},
			"SafeString":          {Type: "string"},
			"UTCOffset":           {Type: "string"},
		},
		Package:   args.PackageName,
		Generated: args.Output,
	}

	generated, err := generate.Generate(codegenConfig)
	if err != nil {
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
