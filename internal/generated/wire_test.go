// Package generated holds the committed output of this SDK's typed-payload code
// generator, and the tests over it.
//
// Two operation documents are generated from, both in testdata:
//
//	cli.graphql   real Fragment CLI output, the only input shape customers have
//	edge.graphql  hand-written awkward cases the CLI does not produce
//
// The generated packages beside them are committed rather than produced at test
// time, because generated source has to be compiled before anything can construct
// a payload from it. That also means `go build ./...` type-checks the generator's
// product. `TestGeneratedPayloadsAreCurrent` in internal/typedentries keeps them
// current; `go test ./internal/typedentries -update` rewrites them.
package generated

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/Khan/genqlient/graphql"

	"github.com/fragment-dev/fragment-go/v4/batch"
	"github.com/fragment-dev/fragment-go/v4/queries"

	cli "github.com/fragment-dev/fragment-go/v4/internal/generated/cli/typed_payloads"
	edge "github.com/fragment-dev/fragment-go/v4/internal/generated/edge/typed_payloads"
)

// capturingClient encodes the request's variables and records the result instead
// of sending it, so a test can inspect exactly what would go on the wire.
//
// It marshals rather than merely storing the request because that is what a real
// transport does: an encoding failure has to surface as an error from MakeRequest,
// or a test could not tell a broken payload from a working one.
type capturingClient struct {
	req  *graphql.Request
	body []byte
}

func (c *capturingClient) MakeRequest(_ context.Context, req *graphql.Request, _ *graphql.Response) error {
	c.req = req

	body, err := json.Marshal(req.Variables)
	if err != nil {
		return err
	}
	c.body = body
	return nil
}

func ptr[T any](v T) *T { return &v }

// wireOf posts a batch through the real entrypoint and returns the variables it
// would have sent. Going through AddTypedLedgerEntries rather than calling
// MarshalJSON directly means the batch variables and the operation wiring are
// covered too, not just the generated payloads.
func wireOf(t *testing.T, entries ...batch.Entry) []byte {
	t.Helper()

	client := &capturingClient{}
	if _, err := queries.AddTypedLedgerEntries(context.Background(), client, entries...); err != nil {
		t.Fatalf("AddTypedLedgerEntries: %v", err)
	}
	if client.req == nil {
		t.Fatal("no request was made")
	}
	if client.req.OpName != "AddLedgerEntries" {
		t.Errorf("OpName = %q, want AddLedgerEntries", client.req.OpName)
	}
	return client.body
}

// TestWireFormat is the worked example: one batch exercising everything the
// serializer decides, with the complete JSON it produces written out.
//
// It is the canonical reference for the wire format. Prose cannot pin nesting,
// version normalisation and name handling at the same time, and a diff here is the
// clearest possible statement of what changed. Written by hand rather than
// regenerated, so that altering the wire format takes a deliberate edit.
//
// Each entry earns its place:
//
//	1  a real CLI payload, with an entry-level timestamp and tags
//	2  version 2 of an entry type whose version 1 also exists
//	3  a parameter colliding with a common field, source-ordered parameters, an
//	   omitted optional one, and a non-ASCII value
//	4  parameters the operation did not type, supplied as raw JSON
//	5  an entry type that takes no parameters
//	6  a raw untyped entry, mixed into the same batch
func TestWireFormat(t *testing.T) {
	posted := "2026-01-01T00:00:00Z"
	entryType := "card_settle"
	rawParameters := json.RawMessage(`{"user_id":"user-9","order_id":"order-9","currency":"USD","amount":"25"}`)

	got := wireOf(t,
		// 1. Real CLI output. Note posted and tags are set but description,
		//    groups and conditions are not, so they do not appear at all.
		cli.OrderPlacedV1Entry{
			Ik:           "ik-1",
			LedgerIk:     "prod",
			Posted:       &posted,
			Tags:         []queries.LedgerEntryTagInput{{Key: "source", Value: "example"}},
			UserId:       "user-1",
			OrderId:      "order-1",
			OrderCost:    "1000",
			Currency:     "USD",
			PlatformFee:  "100",
			DriverFee:    "200",
			RestaurantId: "restaurant-1",
			DriverId:     "driver-1",
		},

		// 2. typeVersion is always on the wire, and it is 2 here because identity
		//    is the (type, typeVersion) pair rather than the type alone.
		edge.UserFundsV2Entry{
			Ik:        "ik-2",
			LedgerIk:  "prod",
			Amount:    "500",
			FeeAmount: "5",
		},

		// 3. Posted2 carries the "posted" parameter; the entry's own posted is
		//    left unset here, so the two are visibly independent. Memo is optional
		//    and omitted. Alpha comes after Zulu because that is the order the
		//    operation declares them in, not alphabetical.
		edge.ReservedV1Entry{
			Ik:       "ik-3",
			LedgerIk: "prod",
			Posted2:  ptr("2020-06-01"),
			Zulu:     "a&b <tag> café 日本",
			Alpha:    "first-in-source-order",
		},

		// 4. The untyped fallback. The caller's JSON goes through verbatim.
		edge.RuntimeV1Entry{
			Ik:         "ik-4",
			LedgerIk:   "prod",
			Parameters: json.RawMessage(`{"anything":"goes"}`),
		},

		// 5. An entry type with no parameters. There is no field to set, and the
		//    empty parameters object is still sent.
		edge.FixedV1Entry{
			Ik:       "ik-5",
			LedgerIk: "prod",
		},

		// 6. A raw entry. Its unset fields are omitted too, which is what lets it
		//    share a batch with the typed payloads above: the API resolves
		//    {"id":null,"ik":"prod"} as a different Ledger from {"ik":"prod"}.
		queries.RawEntry{Input: queries.AddLedgerEntryInput{
			Ik: "ik-6",
			Entry: queries.LedgerEntryInput{
				Ledger:     &queries.LedgerMatchInput{Ik: ptr("prod")},
				Type:       &entryType,
				Parameters: &rawParameters,
			},
		}},
	)

	const want = `{
  "entries": [
    {
      "ik": "ik-1",
      "entry": {
        "ledger": {"ik": "prod"},
        "type": "order_placed",
        "typeVersion": 1,
        "posted": "2026-01-01T00:00:00Z",
        "tags": [{"key": "source", "value": "example"}],
        "parameters": {
          "user_id": "user-1",
          "order_id": "order-1",
          "order_cost": "1000",
          "currency": "USD",
          "platform_fee": "100",
          "driver_fee": "200",
          "restaurant_id": "restaurant-1",
          "driver_id": "driver-1"
        }
      }
    },
    {
      "ik": "ik-2",
      "entry": {
        "ledger": {"ik": "prod"},
        "type": "user-funds",
        "typeVersion": 2,
        "parameters": {"amount": "500", "feeAmount": "5"}
      }
    },
    {
      "ik": "ik-3",
      "entry": {
        "ledger": {"ik": "prod"},
        "type": "reserved",
        "typeVersion": 1,
        "parameters": {
          "posted": "2020-06-01",
          "zulu": "a&b <tag> café 日本",
          "alpha": "first-in-source-order"
        }
      }
    },
    {
      "ik": "ik-4",
      "entry": {
        "ledger": {"ik": "prod"},
        "type": "runtime",
        "typeVersion": 1,
        "parameters": {"anything": "goes"}
      }
    },
    {
      "ik": "ik-5",
      "entry": {
        "ledger": {"ik": "prod"},
        "type": "fixed",
        "typeVersion": 1,
        "parameters": {}
      }
    },
    {
      "ik": "ik-6",
      "entry": {
        "ledger": {"ik": "prod"},
        "type": "card_settle",
        "parameters": {
          "user_id": "user-9",
          "order_id": "order-9",
          "currency": "USD",
          "amount": "25"
        }
      }
    }
  ]
}`

	assertJSONEqual(t, []byte(want), got)
}

// assertJSONEqual compares two encodings by parsed equality: the same keys,
// nesting and values, with key order unconstrained. Ordering that does matter is
// asserted on the raw bytes, in the tests below.
func assertJSONEqual(t *testing.T, wantRaw, gotRaw []byte) {
	t.Helper()

	var want, have any
	if err := json.Unmarshal(wantRaw, &want); err != nil {
		t.Fatalf("parsing expected JSON: %v\n%s", err, wantRaw)
	}
	if err := json.Unmarshal(gotRaw, &have); err != nil {
		t.Fatalf("parsing produced JSON: %v\n%s", err, gotRaw)
	}

	if !reflect.DeepEqual(want, have) {
		wantPretty, _ := json.MarshalIndent(want, "", "  ")
		havePretty, _ := json.MarshalIndent(have, "", "  ")
		t.Errorf("wire JSON does not match\n\nwant:\n%s\n\ngot:\n%s", wantPretty, havePretty)
	}
}

// TestParameterOrderIsSourceOrder checks ordering on the raw bytes, which the
// parsed comparison in TestWireFormat cannot see.
//
// Parameters keep the order the operation declares. It is not alphabetical and not
// required-first: source order is the only ordering available without inventing
// one, and reordering them changes nothing semantically but makes diffs between
// requests unreadable.
func TestParameterOrderIsSourceOrder(t *testing.T) {
	got := string(wireOf(t, edge.ReservedV1Entry{
		Ik:       "ik-1",
		LedgerIk: "prod",
		Posted2:  ptr("p"),
		Zulu:     "z",
		Memo:     ptr("m"),
		Alpha:    "a",
	}))

	want := `"parameters":{"posted":"p","zulu":"z","memo":"m","alpha":"a"}`
	if !strings.Contains(got, want) {
		t.Errorf("parameters are not in source order.\nwant substring: %s\ngot: %s", want, got)
	}
}

// TestUnsetFieldsAreOmitted asserts the distinction directly: a field the caller
// did not set is absent, not null.
//
// This is not cosmetic. encoding/json's omitempty cannot express it, since that
// also drops "", 0 and false — and a null in the wrong place changes meaning, as
// {"id":null,"ik":"prod"} does for a Ledger reference.
func TestUnsetFieldsAreOmitted(t *testing.T) {
	got := string(wireOf(t, edge.UserFundsV1Entry{
		Ik:       "ik-1",
		LedgerIk: "prod",
		Amount:   "100",
	}))

	if strings.Contains(got, "null") {
		t.Errorf("wire JSON should not contain null, got: %s", got)
	}
	for _, key := range []string{"posted", "description", "tags", "groups", "conditions", "lines"} {
		if strings.Contains(got, `"`+key+`":`) {
			t.Errorf("unset field %q should be absent, got: %s", key, got)
		}
	}
}

// TestEntryOrderIsPreserved checks that entries arrive in the order given, which is
// what lets a caller match the API's results back to its own inputs.
func TestEntryOrderIsPreserved(t *testing.T) {
	got := string(wireOf(t,
		edge.UserFundsV2Entry{Ik: "second", LedgerIk: "prod", Amount: "1", FeeAmount: "2"},
		edge.UserFundsV1Entry{Ik: "first", LedgerIk: "prod", Amount: "1"},
	))

	secondAt, firstAt := strings.Index(got, `"second"`), strings.Index(got, `"first"`)
	if secondAt < 0 || firstAt < 0 {
		t.Fatalf("both iks should appear, got: %s", got)
	}
	if secondAt > firstAt {
		t.Errorf("entries were reordered, got: %s", got)
	}
}

// TestUntypedParametersOmittedWhenNil checks that raw JSON follows the same
// omission rule as everything else, and in particular that a nil RawMessage does
// not become the literal null its []byte nature would suggest.
func TestUntypedParametersOmittedWhenNil(t *testing.T) {
	got := string(wireOf(t, edge.RuntimeV1Entry{Ik: "ik-1", LedgerIk: "prod"}))

	if strings.Contains(got, `"parameters":`) || strings.Contains(got, "null") {
		t.Errorf("unset parameters should be absent, got: %s", got)
	}
}

// TestEmptyBatch checks that a batch with no entries serializes as an empty array
// rather than null, which a nil slice would produce.
func TestEmptyBatch(t *testing.T) {
	got := string(wireOf(t))
	if got != `{"entries":[]}` {
		t.Errorf("got %s, want {\"entries\":[]}", got)
	}
}

// TestMarshalErrorReachesTheCaller checks that a failure inside a nested object
// propagates out through the batch rather than producing truncated JSON.
func TestMarshalErrorReachesTheCaller(t *testing.T) {
	client := &capturingClient{}
	if _, err := queries.AddTypedLedgerEntries(context.Background(), client, brokenEntry{}); err == nil {
		t.Error("expected the marshalling error to reach the caller")
	}
}

// brokenEntry is an Entry whose marshalling always fails.
type brokenEntry struct{}

func (brokenEntry) FragmentBatchEntry() {}

func (brokenEntry) MarshalJSON() ([]byte, error) {
	o := batch.NewObject()
	o.Set("bad", func() {}) // funcs cannot be marshalled
	return o.MarshalJSON()
}
