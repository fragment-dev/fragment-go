package livetest

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/fragment-dev/fragment-go/v4/queries"
)

// TestSchemaTestdataParses checks the stored Schema against SchemaInput without
// needing credentials, so it runs in CI alongside everything else.
//
// It is worth having separately because a mismatch here fails the live tests in a
// confusing way: encoding/json ignores unknown fields, so a renamed or missing
// field would leave the Schema silently short of its entry types and every later
// assertion would fail on a Ledger whose Schema defines nothing.
func TestSchemaTestdataParses(t *testing.T) {
	raw, err := os.ReadFile("testdata/marketplace-schema.json")
	if err != nil {
		t.Fatal(err)
	}

	var schema queries.SchemaInput
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("the Schema does not fit SchemaInput: %v", err)
	}

	if schema.LedgerEntries == nil || len(schema.LedgerEntries.Types) == 0 {
		t.Fatal("no entry types were parsed; storing this Schema would create one with no entries")
	}
	if len(schema.ChartOfAccounts.Accounts) == 0 {
		t.Error("no accounts were parsed")
	}

	// The entry types the live tests post. Their absence would show up as a
	// confusing API rejection rather than a missing-fixture error.
	want := map[string]bool{"order_placed": false, "card_settle": false}
	for _, entryType := range schema.LedgerEntries.Types {
		if _, ok := want[entryType.Type]; ok {
			want[entryType.Type] = true
		}
	}
	for entryType, found := range want {
		if !found {
			t.Errorf("the Schema does not define %q, which the live tests post", entryType)
		}
	}

	t.Logf("parsed %d entry types and %d accounts",
		len(schema.LedgerEntries.Types), len(schema.ChartOfAccounts.Accounts))
}
