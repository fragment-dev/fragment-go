package livetest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/google/uuid"

	"github.com/fragment-dev/fragment-go/v4/client"
	"github.com/fragment-dev/fragment-go/v4/queries"
)

// schemaKey is fixed rather than per-run. storeSchema stores a new version each
// time, so reusing the key keeps the Workspace from filling up with Schemas while
// still giving every run current definitions.
const schemaKey = "fragment-go-livetest"

// requireClient builds an authenticated client, or skips the test when the
// environment has no credentials.
func requireClient(t *testing.T) graphql.Client {
	t.Helper()

	params := &client.GetTokenParams{
		ClientID:     os.Getenv("FRAGMENT_CLIENT_ID"),
		ClientSecret: os.Getenv("FRAGMENT_CLIENT_SECRET"),
		Scope:        os.Getenv("FRAGMENT_SCOPE"),
		AuthURL:      os.Getenv("FRAGMENT_AUTH_URL"),
		ApiURL:       os.Getenv("FRAGMENT_API_URL"),
	}
	if params.ClientID == "" || params.ClientSecret == "" || params.ApiURL == "" {
		t.Skip("set FRAGMENT_CLIENT_ID, FRAGMENT_CLIENT_SECRET, FRAGMENT_SCOPE, FRAGMENT_AUTH_URL and FRAGMENT_API_URL to run the live tests")
	}

	c, err := client.NewClient(params)
	if err != nil {
		t.Fatalf("building client: %v", err)
	}
	return c
}

// storeSchema stores the marketplace template Schema, which is the Schema the
// operations in internal/conformance/testdata/007-cli-output.graphql were
// generated from — so the typed payloads under f007 describe exactly its entry
// types.
func storeSchema(t *testing.T, ctx context.Context, c graphql.Client) {
	t.Helper()

	raw, err := os.ReadFile("testdata/marketplace-schema.json")
	if err != nil {
		t.Fatalf("reading the Schema: %v", err)
	}

	var schema queries.SchemaInput
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("parsing the Schema: %v", err)
	}
	// The file carries the template's own key. Override it so the live tests own
	// their Schema rather than writing over something else in the Workspace.
	schema.Key = schemaKey
	name := schemaKey
	schema.Name = &name

	resp, err := queries.StoreSchema(ctx, c, schema)
	if err != nil {
		t.Fatalf("storeSchema: %v", err)
	}

	switch r := resp.GetStoreSchema().(type) {
	case *queries.StoreSchemaStoreSchemaStoreSchemaResult:
		t.Logf("stored Schema %s version %d", r.Schema.Key, r.Schema.Version.Version)
	default:
		t.Fatalf("storeSchema failed: %s", describe(resp.GetStoreSchema()))
	}
}

// createLedger creates a Ledger for this run and returns its ik. Each run gets its
// own so that concurrent runs, and reruns after a failure, cannot collide on entry
// idempotency keys.
func createLedger(t *testing.T, ctx context.Context, c graphql.Client) string {
	t.Helper()

	ik := fmt.Sprintf("livetest-%d-%s", time.Now().Unix(), uuid.NewString()[:8])

	resp, err := queries.CreateLedger(ctx, c, ik, queries.CreateLedgerInput{Name: ik}, schemaKey)
	if err != nil {
		t.Fatalf("createLedger: %v", err)
	}

	switch resp.GetCreateLedger().(type) {
	case *queries.CreateLedgerCreateLedgerCreateLedgerResult:
		t.Logf("created Ledger %s", ik)
	default:
		t.Fatalf("createLedger failed: %s", describe(resp.GetCreateLedger()))
	}
	return ik
}

// setup stores the Schema and creates a Ledger, returning the Ledger's ik.
func setup(t *testing.T, ctx context.Context, c graphql.Client) string {
	t.Helper()
	storeSchema(t, ctx, c)
	return createLedger(t, ctx, c)
}

// describe renders an unexpected union member for a failure message. The generated
// error types share these fields but not an interface, so this reaches for them
// reflectively rather than enumerating every case at every call site.
func describe(v any) string {
	type apiError interface {
		GetCode() string
		GetMessage() string
	}
	if e, ok := v.(apiError); ok {
		return fmt.Sprintf("%T: %s: %s", v, e.GetCode(), e.GetMessage())
	}
	return fmt.Sprintf("%T", v)
}

// mustJSON encodes a value for use as an untyped parameters payload.
func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
