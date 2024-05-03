# fragment-go

[Fragment](https://fragment.dev) is the Ledger API for engineers that move money. Stop wrangling payment tables, debugging balance errors, and hacking together data pipelines. Start shipping the features that make a difference.

## Installation

This library requires Go 1.20+.

``` shell
go get -u github.com/fragment-dev/fragment-go
```

## Usage

To start issuing queries, you'll first need to create an `auth.AuthenticatedContext`. You can generate credentials using the Fragment [dashboard](https://dashboard.fragment.dev/go/s/api-clients).

``` go
import (
  "context"
  "fmt"
  "os"
  
  "github.com/fragment-dev/fragment-go/auth"
  "github.com/fragment-dev/fragment-go/queries"
)

func main() {
  // Create an authenticated context
  authenticatedContext, err := auth.GetAuthenticatedContext(
    context.Background(),
    &auth.GetTokenParams{
      ClientId:     "Client ID from Dashboard"
      ClientSecret: "Client Secret from Dashboard"
      Scope:        "OAuth Scope from Dashboard"
      AuthUrl:      "OAuth URL from Dashboard"
      ApiUrl:       "API URL from Dashboard"
    }
  )
  
  if err != nil {
    fmt.Println(err)
    os.Exit(1)
  }
  
  fmt.Println("Successfully Authenticated!")
  
  // Use one of the predefined queries available
  response, _ := queries.GetLedger(authenticatedContext, "your-ledger-ik")
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
go run github.com/fragment-dev/fragment-go \
  --input <path-to-your-graphql-queries-file.graphql>
  --output <path-to-the-output.go>
  --package <package-name>
```

## Examples

### Post a Ledger Entry

To [post](https://fragment.dev/docs#post-ledger-entries-post-to-the-api) a Ledger Entry defined in your schema:

``` go
import (
  "encoding/json"
  
  "github.com/fragment-dev/fragment-go/queries"
)

type UserFundsAccountParameters struct {
  FundingAmount string `json:"funding_amount"`
  UserId        string `json:"user_id"`
}

serializedParams, _ := json.Marshal(&UserFundsAccountParameters{
  FundingAmount: "100",
  UserId:        "user-1",
})

var posted string = "1968-01-01T16:45:00Z"
response, _ := queries.AddLedgerEntry(
  authenticatedContext,
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
```

### Read a Ledger Account's balance

To read a Ledger Account's [balance](https://fragment.dev/docs#read-balances-latest):

``` go
import (
  "github.com/fragment-dev/fragment-go/queries"
)

response, _ := queries.GetLedgerAccountBalance(
  authenticatedContext,
  "liabilities/user:user-1/available",
  "your-ledger-ik",
  &queries.CurrencyMatchInput{queries.CurrencyCodeUsd, nil},
  nil,
  nil,
)

fmt.Println("Latest balance of account is: ", response.LedgerAccount.OwnBalance)
```
