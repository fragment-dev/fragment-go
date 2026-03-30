# fragment-go

[Fragment](https://fragment.dev) is the Ledger API for engineers that move money. Stop wrangling payment tables, debugging balance errors, and hacking together data pipelines. Start shipping the features that make a difference.

> We've upgraded the module import path to `github.com/fragment-dev/fragment-go/v4`. Use `/v4` for the latest version, or keep using `/v3` for the previous major version. See the [Upgrading SDK Versions](#upgrading-sdk-versions) section for details.

## Installation

This library requires Go 1.20+.

``` shell
go get -u github.com/fragment-dev/fragment-go/v4
```

## Usage

To start issuing queries, you'll first need to create a client with the API credentials. You can generate credentials using the Fragment [dashboard](https://dashboard.fragment.dev/go/s/api-clients).

``` go
import (
  "context"
  "fmt"
  "os"
  
  "github.com/fragment-dev/fragment-go/v4/client"
  "github.com/fragment-dev/fragment-go/v4/queries"
)

func main() {
  // Create a client
  graphqlClient, err := client.NewClient(
    &client.GetTokenParams{
      ClientID:     "Client ID from Dashboard",
      ClientSecret: "Client Secret from Dashboard",
      Scope:        "OAuth Scope from Dashboard",
      AuthURL:      "OAuth URL from Dashboard",
      ApiURL:       "API URL from Dashboard",
    },
  )
  
  if err != nil {
    fmt.Println(err)
    os.Exit(1)
  }
  
  fmt.Println("Successfully Authenticated!")
  
  // Use one of the predefined queries available
  response, _ := queries.GetLedger(context.Background(), graphqlClient, "your-ledger-ik")
  if response.Ledger != nil {
    fmt.Println("Retrieved Ledger " + response.Ledger.GetName())
  }
}
```

Read the [Examples](#Examples) section to learn how to post a Ledger Entry and read balances.

We appreciate feedback; please open an [issue](https://github.com/fragment-dev/fragment-go/issues) with questions, bugs, or suggestions.

## Using custom queries

While the SDK comes with predefined GraphQL queries, you may want to customize these queries for your product. In order to do that, run:

``` shell
go run github.com/fragment-dev/fragment-go/v4 \
  --input <path-to-your-graphql-queries-file.graphql>
  --output <path-to-the-output.go>
  --package <package-name>
```

### An end-to-end example

Say you're developing within the `main` package of your product and you have the following custom GraphQL query saved to `queries.graphql`.

``` graphql
query GetLatestSchema($key: SafeString!) {
  schema(schema: { key: $key }) {
    key
    name
    version {
      created
      version
      json
    }
  }
}
```

Run the SDK codegen to generate the code for your GraphQL query.

``` shell
go run github.com/fragment-dev/fragment-go/v4 \
  --input queries.graphql
  --output queries.go
  --package main
```

This should generate a `queries.go` file in your current working directory. You can then issue the above GraphQL request by calling `GetLatestSchema`:

``` go
package main

import (
	"context"
	"fmt"
)

func main() {
	response, _ := GetLatestSchema(
		context.Background(),
		graphqlClient,
		"your-schema-key",
	)

	fmt.Println("Latest version of Schema is: ", response.Schema.GetVersion().Version)
	// Alternatively
	fmt.Println("Latest version of Schema is: ", response.Schema.Version.Version)
}

```

## Examples

### Post a Ledger Entry

To [post](https://fragment.dev/docs#post-ledger-entries-post-to-the-api) a Ledger Entry defined in your schema:

``` go
package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/fragment-dev/fragment-go/v4/queries"
)

type UserFundsAccountParameters struct {
	FundingAmount string `json:"funding_amount"`
	UserID        string `json:"user_id"`
}

func main() {
	serializedParams, _ := json.Marshal(&UserFundsAccountParameters{
		FundingAmount: "100",
		UserID:        "user-1",
	})

	var posted string = "1968-01-01T16:45:00Z"
	response, _ := queries.AddLedgerEntry(
		context.Background(),
		graphqlClient,
		"some-ik",
		"your-ledger-ik",
		"user_funds_account",
		&posted,
		json.RawMessage(&serializedParams),
		[]queries.LedgerEntryTagInput{},
		[]queries.LedgerEntryGroupInput{},
	)

	switch r := (response.AddLedgerEntry).(type) {
	case *queries.AddLedgerEntryAddLedgerEntryAddLedgerEntryResult:
		fmt.Println("Posted Entry with IK: ", v.Entry.Ik)
		break
	case *queries.AddLedgerEntryAddLedgerEntryInternalError:
	case *queries.AddLedgerEntryAddLedgerEntryBadRequestError:
		fmt.Println("Received error: ", v.Message)
		break
	}
}
```

### Read a Ledger Account's balance

To read a Ledger Account's [balance](https://fragment.dev/docs#read-balances-latest):

``` go
package main

import (
	"context"
	"fmt"

	"github.com/fragment-dev/fragment-go/v4/queries"
)

func main() {
	response, _ := queries.GetLedgerAccountBalance(
		context.Background(),
		graphqlClient,
		"liabilities/user:user-1/available",
		"your-ledger-ik",
		&queries.CurrencyMatchInput{queries.CurrencyCodeUsd, nil},
		nil,
		nil,
	)

	fmt.Println("Latest balance of account is: ", response.LedgerAccount.OwnBalance)
}
```

### Read a Schema

To get a Schema from your Workspace:

``` go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/fragment-dev/fragment-go/v4/queries"
)

func main() {
	data, err := queries.GetSchema(context.Background(), graphqlClient, "test-schema", nil)
	if err != nil {
		fmt.Println("Failed to get schema.")
		fmt.Println(err)
		os.Exit(1)
	}

	// Marshal the entire response to pretty JSON
	jsonData, err := json.MarshalIndent(data.Schema.Version.Json, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling to JSON: %v\n", err)
		os.Exit(1)
	}
    # Save the Schema as a file in your repository
	err = os.WriteFile("fragment-schema.json", jsonData, 0644)
	if err != nil {
		fmt.Printf("Error writing file: %v\n", err)
		os.Exit(1)
	}
}
```

### Store a Schema

To [store](https://fragment.dev/api-reference/api-mutations#storeschema) a new version of your Schema:

``` go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/fragment-dev/fragment-go/v4/queries"
)

func main() {
    // Read and unmarshal the JSON into SchemaInput
    jsonData, _ := os.ReadFile("fragment-schema.json")
    
    var schemaInput queries.SchemaInput
    json.Unmarshal(jsonData, &schemaInput)
    
    // Set the name since it's not on the JSON object
    schemaInput.Name = &schemaInput.Key
    
    response, _ := queries.StoreSchema(context.Background(), graphqlClient, schemaInput)
}
```

## Upgrading SDK Versions
### Changes from v3.0.0 to v4.0.0
- Module import path updated to `/v4`: Update all imports from `github.com/fragment-dev/fragment-go/v3` to `github.com/fragment-dev/fragment-go/v4`

### Changes from v2.0.0 to v3.0.0

- Removed `AuthenticatedContext` and replaced it with `client.NewClient()`
- Token management is now handled by the client
- The client and a generic `context.Context` are now passed to each query

With this change, the SDK now conforms to the Go standards around context usage.

In v2.0.0 you would have set up an `AuthenticatedContext` with your API credentials and then passed it into every query function:

```
authenticatedContext, _ := auth.GetAuthenticatedContext(
	  context.Background(),
    &auth.GetTokenParams{
		ClientID:     "<API Client ID>",
		ClientSecret: "<API Client Secret>",
		Scope:        "<OAuth Scope>",
		AuthURL:      "<OAuth URL>",
		ApiURL:       "<API URL>",
	})

	data, _ := queries.CreateLedger(
		authenticatedContext,
		"test-ledger",
		queries.CreateLedgerInput{Name: "Test Ledger"},
		"test-schema")
```

In v3.x, you'll now initialize a client with your API credentials and then passed it into every query function:

```
	tokenParams := &client.GetTokenParams{
		ClientID:     "<API Client ID>",
		ClientSecret: "<API Client Secret>",
		Scope:        "<OAuth Scope>",
		AuthURL:      "<OAuth URL>",
		ApiURL:       "<API URL>",
	}

	graphqlClient, _ := client.NewClient(tokenParams)

	data, _ := queries.CreateLedger(
		context.Background(),
		graphqlClient,
		"test-ledger",
		queries.CreateLedgerInput{Name: "Test Ledger"},
		"test-schema")
```

### Changes from v1.20 to v2.0.0

See details and how to fix breaking changes [here](https://github.com/fragment-dev/fragment-go/releases/tag/v2.0.0).