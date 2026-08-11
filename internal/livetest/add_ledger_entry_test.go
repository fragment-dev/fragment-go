package livetest

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/fragment-dev/fragment-go/v4/queries"
)

// TestAddLedgerEntry posts one entry the untyped way, which is the path that
// existed before typed payloads: parameters as an opaque JSON blob.
//
// It is the control for TestAddTypedLedgerEntries. If both fail, the Schema or the
// credentials are wrong; if only the batch fails, the typed payloads are.
func TestAddLedgerEntry(t *testing.T) {
	ctx := context.Background()
	c := requireClient(t)
	ledgerIk := setup(t, ctx, c)

	orderID := uuid.NewString()
	parameters, err := json.Marshal(map[string]string{
		"user_id":       "user-1",
		"order_id":      orderID,
		"order_cost":    "1000",
		"currency":      "USD",
		"platform_fee":  "100",
		"driver_fee":    "200",
		"restaurant_id": "restaurant-1",
		"driver_id":     "driver-1",
	})
	if err != nil {
		t.Fatal(err)
	}

	posted := "2026-01-01T00:00:00Z"
	ik := "single-" + orderID

	resp, err := queries.AddLedgerEntry(
		ctx, c,
		ik,
		ledgerIk,
		"order_placed",
		nil, // typeVersion: unpinned, which the API resolves to 1
		&posted,
		json.RawMessage(parameters),
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("addLedgerEntry: %v", err)
	}

	result, ok := resp.GetAddLedgerEntry().(*queries.AddLedgerEntryAddLedgerEntryAddLedgerEntryResult)
	if !ok {
		t.Fatalf("addLedgerEntry failed: %s", describe(resp.GetAddLedgerEntry()))
	}
	if result.Entry.Ik != ik {
		t.Errorf("entry ik = %q, want %q", result.Entry.Ik, ik)
	}
	if len(result.Lines) == 0 {
		t.Error("expected the entry to have lines")
	}
	t.Logf("posted %s with %d lines", result.Entry.Ik, len(result.Lines))
}
