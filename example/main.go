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
		ClientID:     "<API Client ID>",
		ClientSecret: "<API Client Secret>",
		Scope:        "<OAuth Scope>",
		AuthURL:      "<OAuth URL>",
		ApiURL:       "<API URL>",
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
