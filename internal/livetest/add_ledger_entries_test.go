package livetest

import (
	"context"
	"github.com/Khan/genqlient/graphql"
	"testing"

	"github.com/google/uuid"

	"github.com/fragment-dev/fragment-go/v4/batch"
	"github.com/fragment-dev/fragment-go/v4/queries"

	cli "github.com/fragment-dev/fragment-go/v4/internal/generated/cli/typed_payloads"
)

// TestAddTypedLedgerEntries posts a batch built from the generated typed payloads.
//
// The payloads come from internal/generated/cli, which this repo's own codegen
// derived from real Fragment CLI output for the same Schema this test stores. So
// the test exercises the generator's product, not a hand-written approximation of
// it.
//
// This is the case the shared specification records as untested in every SDK: a
// typed payload sends an entry object with a type and no lines, and nothing has
// confirmed the API accepts that.
func TestAddTypedLedgerEntries(t *testing.T) {
	ctx := context.Background()
	c := requireClient(t)
	ledgerIk := setup(t, ctx, c)

	orderID := uuid.NewString()
	posted := "2026-01-01T00:00:00Z"

	// Two entries of different types in one atomic batch: the order is placed,
	// then the card settles against the receivable it created.
	placed := cli.OrderPlacedV1Entry{
		Ik:           "batch-placed-" + orderID,
		LedgerIk:     ledgerIk,
		Posted:       &posted,
		UserId:       "user-1",
		OrderId:      orderID,
		OrderCost:    "1000",
		Currency:     "USD",
		PlatformFee:  "100",
		DriverFee:    "200",
		RestaurantId: "restaurant-1",
		DriverId:     "driver-1",
		Tags:         []queries.LedgerEntryTagInput{{Key: "source", Value: "livetest"}},
	}
	settled := cli.CardSettleV1Entry{
		Ik:       "batch-settled-" + orderID,
		LedgerIk: ledgerIk,
		Posted:   &posted,
		UserId:   "user-1",
		OrderId:  orderID,
		Currency: "USD",
		Amount:   "1300",
	}

	resp, err := queries.AddTypedLedgerEntries(ctx, c, placed, settled)
	if err != nil {
		t.Fatalf("addLedgerEntries: %v", err)
	}

	result := requireBatchResult(t, resp)
	if len(result.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result.Results))
	}

	// Results come back in the order the entries were sent, which is what lets a
	// caller match them to its own inputs.
	wantIks := []string{placed.Ik, settled.Ik}
	for i, want := range wantIks {
		got := result.Results[i]
		if got.Entry.Ik != want {
			t.Errorf("result %d ik = %q, want %q", i, got.Entry.Ik, want)
		}
		if got.IsIkReplay {
			t.Errorf("result %d was reported as a replay on first post", i)
		}
		if len(got.Lines) == 0 {
			t.Errorf("result %d has no lines", i)
		}
	}
}

// TestAddTypedLedgerEntriesReportsIkReplay reposts a batch unchanged.
//
// Idempotency keys are per entry rather than per batch, so a resend must commit
// nothing new and report every entry as a replay. This is what makes a retry after
// an ambiguous failure safe.
func TestAddTypedLedgerEntriesReportsIkReplay(t *testing.T) {
	ctx := context.Background()
	c := requireClient(t)
	ledgerIk := setup(t, ctx, c)

	orderID := uuid.NewString()
	entry := cli.OrderPlacedV1Entry{
		Ik:           "replay-" + orderID,
		LedgerIk:     ledgerIk,
		UserId:       "user-1",
		OrderId:      orderID,
		OrderCost:    "1000",
		Currency:     "USD",
		PlatformFee:  "100",
		DriverFee:    "200",
		RestaurantId: "restaurant-1",
		DriverId:     "driver-1",
	}

	first := requireBatchResult(t, mustPost(t, ctx, c, entry))
	if first.Results[0].IsIkReplay {
		t.Error("first post should not be a replay")
	}

	second := requireBatchResult(t, mustPost(t, ctx, c, entry))
	if !second.Results[0].IsIkReplay {
		t.Error("second post of the same ik should be reported as a replay")
	}
	if second.Results[0].Entry.Id != first.Results[0].Entry.Id {
		t.Errorf("replay returned a different entry: %q then %q",
			first.Results[0].Entry.Id, second.Results[0].Entry.Id)
	}
}

// TestAddTypedLedgerEntriesMixedWithRaw checks that a typed payload and a raw
// input commit together, since the two serialize by different routes.
func TestAddTypedLedgerEntriesMixedWithRaw(t *testing.T) {
	ctx := context.Background()
	c := requireClient(t)
	ledgerIk := setup(t, ctx, c)

	orderID := uuid.NewString()
	typed := cli.OrderPlacedV1Entry{
		Ik:           "mixed-typed-" + orderID,
		LedgerIk:     ledgerIk,
		UserId:       "user-1",
		OrderId:      orderID,
		OrderCost:    "1000",
		Currency:     "USD",
		PlatformFee:  "100",
		DriverFee:    "200",
		RestaurantId: "restaurant-1",
		DriverId:     "driver-1",
	}

	// The same entry type by the untyped route, with its own order so the two do
	// not interact.
	rawOrderID := uuid.NewString()
	rawParams := mustJSON(t, map[string]string{
		"user_id":       "user-2",
		"order_id":      rawOrderID,
		"order_cost":    "500",
		"currency":      "USD",
		"platform_fee":  "50",
		"driver_fee":    "75",
		"restaurant_id": "restaurant-2",
		"driver_id":     "driver-2",
	})
	entryType := "order_placed"
	raw := queries.RawEntry{Input: queries.AddLedgerEntryInput{
		Ik: "mixed-raw-" + rawOrderID,
		Entry: queries.LedgerEntryInput{
			Ledger:     &queries.LedgerMatchInput{Ik: &ledgerIk},
			Type:       &entryType,
			Parameters: &rawParams,
		},
	}}

	result := requireBatchResult(t, mustPost(t, ctx, c, typed, raw))
	if len(result.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result.Results))
	}
	if result.Results[0].Entry.Ik != typed.Ik {
		t.Errorf("result 0 ik = %q, want the typed entry %q", result.Results[0].Entry.Ik, typed.Ik)
	}
	if result.Results[1].Entry.Ik != raw.Input.Ik {
		t.Errorf("result 1 ik = %q, want the raw entry %q", result.Results[1].Entry.Ik, raw.Input.Ik)
	}
}

// TestAddTypedLedgerEntriesIsAtomic sends a good entry alongside one that must
// fail, and checks the good one was not written.
//
// The failing entry names a currency the Schema's accounts do not use, so the API
// rejects it while the batch is otherwise valid.
func TestAddTypedLedgerEntriesIsAtomic(t *testing.T) {
	ctx := context.Background()
	c := requireClient(t)
	ledgerIk := setup(t, ctx, c)

	orderID := uuid.NewString()
	good := cli.OrderPlacedV1Entry{
		Ik:           "atomic-good-" + orderID,
		LedgerIk:     ledgerIk,
		UserId:       "user-1",
		OrderId:      orderID,
		OrderCost:    "1000",
		Currency:     "USD",
		PlatformFee:  "100",
		DriverFee:    "200",
		RestaurantId: "restaurant-1",
		DriverId:     "driver-1",
	}
	bad := good
	bad.Ik = "atomic-bad-" + orderID
	bad.OrderId = uuid.NewString()
	bad.Currency = "NOT_A_CURRENCY"

	resp, err := queries.AddTypedLedgerEntries(ctx, c, good, bad)
	if err != nil {
		t.Fatalf("addLedgerEntries: %v", err)
	}

	switch r := resp.GetAddLedgerEntries().(type) {
	case *queries.AddLedgerEntriesAddLedgerEntriesAddLedgerEntriesResult:
		t.Fatalf("expected the batch to fail, but it committed %d entries", len(r.Results))
	case *queries.AddLedgerEntriesAddLedgerEntriesAddLedgerEntriesError:
		// The per-entry list is the point: it carries the ik that tells a caller
		// which entry to fix.
		if len(r.Errors) == 0 {
			t.Error("expected per-entry errors, got only the top-level message")
		}
		for _, e := range r.Errors {
			t.Logf("entry %s failed: %s: %s", e.Ik, e.Code, e.Message)
			if e.Ik == good.Ik {
				t.Errorf("the valid entry %s was reported as failing", good.Ik)
			}
		}
	default:
		t.Logf("batch rejected as %s", describe(resp.GetAddLedgerEntries()))
	}

	// Atomicity: reposting the good entry alone must be a fresh write, not a
	// replay, because the failed batch wrote nothing.
	again := requireBatchResult(t, mustPost(t, ctx, c, good))
	if again.Results[0].IsIkReplay {
		t.Error("the valid entry from the failed batch was committed; the batch was not atomic")
	}
}

func mustPost(t *testing.T, ctx context.Context, c graphql.Client, entries ...batch.Entry) *queries.AddLedgerEntriesResponse {
	t.Helper()
	resp, err := queries.AddTypedLedgerEntries(ctx, c, entries...)
	if err != nil {
		t.Fatalf("addLedgerEntries: %v", err)
	}
	return resp
}

// requireBatchResult narrows the response union, failing with the API's own message
// when the batch did not succeed.
func requireBatchResult(t *testing.T, resp *queries.AddLedgerEntriesResponse) *queries.AddLedgerEntriesAddLedgerEntriesAddLedgerEntriesResult {
	t.Helper()

	result, ok := resp.GetAddLedgerEntries().(*queries.AddLedgerEntriesAddLedgerEntriesAddLedgerEntriesResult)
	if !ok {
		if e, isErr := resp.GetAddLedgerEntries().(*queries.AddLedgerEntriesAddLedgerEntriesAddLedgerEntriesError); isErr {
			for _, entry := range e.Errors {
				t.Logf("entry %s: %s: %s", entry.Ik, entry.Code, entry.Message)
			}
		}
		t.Fatalf("addLedgerEntries failed: %s", describe(resp.GetAddLedgerEntries()))
	}
	if len(result.Results) == 0 {
		t.Fatal("batch succeeded with no results")
	}
	return result
}
