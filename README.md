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

### Post a batch of Ledger Entries

To post several Ledger Entries in one atomic batch, use `AddTypedLedgerEntries`. Unlike
`AddLedgerEntry`, which takes parameters as an opaque `json.RawMessage`, a batch is built
from typed payloads that the codegen derives from your Schema — so a missing or misspelled
parameter is a compile error rather than an API error.

Running the codegen (see [Using custom queries](#using-custom-queries)) produces a
`typed_payloads` package next to your generated client, holding one struct per Ledger
Entry type and version:

``` shell
go run github.com/fragment-dev/fragment-go/v4 \
  --input queries.graphql \
  --output fragment/client.go \
  --package fragment
# writes fragment/client.go and fragment/typed_payloads/typed_payloads.go
```

``` go
package main

import (
	"context"
	"fmt"

	"github.com/fragment-dev/fragment-go/v4/queries"

	"myapp/fragment/typed_payloads"
)

func main() {
	posted := "1968-01-01T16:45:00Z"

	response, err := queries.AddTypedLedgerEntries(
		context.Background(),
		graphqlClient,
		typed_payloads.UserFundsAccountV1Entry{
			Ik:            "ik-1",
			LedgerIk:      "your-ledger-ik",
			Posted:        &posted,
			UserId:        "user-1",
			FundingAmount: "100",
		},
		typed_payloads.AuthCaptureV2Entry{
			Ik:            "ik-2",
			LedgerIk:      "your-ledger-ik",
			UserId:        "user-1",
			CaptureAmount: "25",
		},
	)
	if err != nil {
		fmt.Println(err)
		return
	}

	switch r := response.GetAddLedgerEntries().(type) {
	case *queries.AddLedgerEntriesAddLedgerEntriesAddLedgerEntriesResult:
		for _, result := range r.Results {
			fmt.Println("Posted", result.Entry.Ik, "replay:", result.IsIkReplay)
		}
	case *queries.AddLedgerEntriesAddLedgerEntriesAddLedgerEntriesError:
		// One error per failing entry, each with the ik that identifies it.
		for _, e := range r.Errors {
			fmt.Println("Entry", e.Ik, "failed:", e.Message)
		}
	case *queries.AddLedgerEntriesAddLedgerEntriesBadRequestError:
		fmt.Println("Bad request:", r.Message)
	case *queries.AddLedgerEntriesAddLedgerEntriesInternalError:
		fmt.Println("Internal error:", r.Message)
	}
}
```

A few things worth knowing:

- **The batch is atomic.** Either every entry commits or none do, so there is no partial
  state to reconcile after an error.
- **A batch is limited to 30 Ledger Lines in total**, not 30 entries. How many entries
  that allows depends on your entry types: one posting 2 lines fits 15 per batch, one
  posting 6 lines fits 5. Exceeding it is a `BadRequestError`, and since the batch is
  atomic that means nothing commits — so size your chunks by lines, not entries. The
  SDK cannot check this for you: an entry type's line count lives in your Schema, not
  in the generated operations.
- **Idempotency keys are per entry**, not per batch. Retrying a batch that partly
  succeeded reports `IsIkReplay` on the entries that had already committed.
- **Fields must be set by name.** An unkeyed struct literal will not compile. This is
  deliberate: two parameters of the same type could otherwise be swapped by reordering
  them, and nothing would catch it.
- **Unset fields are omitted**, not sent as `null`.
- **Struct names always carry a version**, so adding a new version of an entry type to
  your Schema never renames the existing one.
- **`Tags`, `Groups` and `Conditions` use the input types from
  `github.com/fragment-dev/fragment-go/v4/queries`**, not the ones in your own
  generated package. The two are identical in shape but are distinct Go types, so
  build them as `queries.LedgerEntryTagInput{...}` even if your generated package has
  a `LedgerEntryTagInput` of its own.
- **A parameter whose Schema type is an enum or input object** is typed as
  `json.RawMessage` rather than precisely, for the same reason; the codegen prints a
  warning when it happens. Parameters generated by the Fragment CLI are always
  scalars, so this only comes up in hand-written operations.

To mix an untyped entry into a batch — for a one-off entry with explicit `lines`, or an
entry type your operations do not cover — wrap it in `queries.RawEntry`:

``` go
response, err := queries.AddTypedLedgerEntries(
	context.Background(),
	graphqlClient,
	typed_payloads.UserFundsAccountV1Entry{ /* ... */ },
	queries.RawEntry{Input: queries.AddLedgerEntryInput{
		Ik:    "ik-3",
		Entry: queries.LedgerEntryInput{ /* ... */ },
	}},
)
```

The rules the codegen follows are shared across the Fragment SDKs; see
[`docs/spec-conformance.md`](docs/spec-conformance.md) for how this one implements them.

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
### Changes from v3.1.0 to v4.0.0

#### Changes in this version

- `GetLedgerAccountBalance` now returns total `balance` (self + children) instead of `ownBalance`
- `GetLedgerAccountBalanceWithChildRollup` has been removed
- `ListLedgerAccountBalances` and `ListMultiCurrencyLedgerAccountBalances` now accept `consistencyMode` on `childBalance`, `childBalances`, `balance`, and `balances` fields

#### How to Upgrade

1. Upgrade your schema to use total balance consistency.
   1. Edit your schema JSON. Change `ownBalanceUpdates` to `totalBalanceUpdates` in ledger account consistency config. Change `ownBalance` to `totalBalance` in entry conditions. A schema can have only one of `ownBalanceUpdates` or `totalBalanceUpdates`.
   2. Deploy the new schema.
2. You can now set `consistencyConfig.totalBalanceUpdates: strong` on any account in the tree, and its balance will be strongly consistent.
3. Upgrade your Fragment SDK to the latest version.
   1. Update imports from `github.com/fragment-dev/fragment-go/v3` to `github.com/fragment-dev/fragment-go/v4`.
   2. `GetLedgerAccountBalance` now returns total `balance` (self + children) instead of `ownBalance`.
   3. Change `$ownBalanceConsistencyMode` to `$balanceConsistencyMode`.
   4. Use `GetLedgerAccountBalance` instead of `GetLedgerAccountBalanceWithChildRollup`.

### Changes in v3.1.0
- Module import path updated to `/v3`: Update all imports from `github.com/fragment-dev/fragment-go` to `github.com/fragment-dev/fragment-go/v3`

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