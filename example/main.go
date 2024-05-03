package main

import (
	"context"
	"fmt"
	"os"

	"github.com/fragment-dev/fragment-go/auth"
	"github.com/fragment-dev/fragment-go/queries"
)

func main() {
	fmt.Println("Running fragment-go SDK example.")

	authenticatedContext, err := auth.GetAuthenticatedContext(context.Background(), &auth.GetTokenParams{
		ClientId:     "<API Client ID>",
		ClientSecret: "<API Client Secret>",
		Scope:        "<OAuth Scope>",
		AuthUrl:      "<OAuth URL>",
		ApiUrl:       "<API URL>",
	})
	if err != nil {
		fmt.Println("Failed to get authenticated context.")
		fmt.Println(err)
		os.Exit(1)
	}

	data, err := queries.CreateLedger(
		authenticatedContext,
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
