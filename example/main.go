package main

import (
	"context"
	"fmt"
	"os"

	"github.com/fragment-dev/fragment-go/client"
	"github.com/fragment-dev/fragment-go/queries"
)

func main() {
	tokenParams := &client.GetTokenParams{
		ClientID:     "cp1ti1h8ur4915t3ft6itr0rc",
		ClientSecret: "1u8il886b8jclh6mb8iircmrv1bivh524r9v0173rhdihcbaacej",
		Scope:        "https://api.fragment.dev/*",
		AuthURL:      "https://auth.dev-us-east-1.engineering.fragment.dev/oauth2/token",
		ApiURL:       "https://api.dev-us-east-1.engineering.fragment.dev/graphql",
	}

	graphqlClient, err := client.NewClient(tokenParams)
	if err != nil {
		fmt.Println("Failed to get authenticated context.")
		fmt.Println(err)
		os.Exit(1)
	}

	data, err := queries.CreateLedger(
		context.Background(),
		graphqlClient,
		"test-ledger",
		queries.CreateLedgerInput{Name: "Test Ledger"},
		"test-schema")

	if err != nil {
		fmt.Println("Failed to create ledger.")
		fmt.Println(err)
		os.Exit(1)
	}

	if respBytes, err := data.MarshalJSON(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	} else {
		fmt.Println("Successfully created ledger.")
		fmt.Println(string(respBytes))
		return
	}
}
