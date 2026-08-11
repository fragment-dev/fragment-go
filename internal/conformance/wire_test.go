package conformance

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Khan/genqlient/graphql"

	"github.com/fragment-dev/fragment-go/v4/batch"
	"github.com/fragment-dev/fragment-go/v4/queries"

	f001 "github.com/fragment-dev/fragment-go/v4/internal/conformance/f001/typed_payloads"
	f002 "github.com/fragment-dev/fragment-go/v4/internal/conformance/f002/typed_payloads"
	f003 "github.com/fragment-dev/fragment-go/v4/internal/conformance/f003/typed_payloads"
	f004 "github.com/fragment-dev/fragment-go/v4/internal/conformance/f004/typed_payloads"
	f005 "github.com/fragment-dev/fragment-go/v4/internal/conformance/f005/typed_payloads"
	f006 "github.com/fragment-dev/fragment-go/v4/internal/conformance/f006/typed_payloads"
)

// capturingClient records the request instead of sending it, so a test can
// inspect exactly what would go on the wire.
type capturingClient struct {
	req *graphql.Request
}

func (c *capturingClient) MakeRequest(_ context.Context, req *graphql.Request, _ *graphql.Response) error {
	c.req = req
	return nil
}

func ptr[T any](v T) *T { return &v }

// wireOf posts a batch through the real entrypoint and returns the variables it
// would have sent. Going through AddTypedLedgerEntries rather than calling
// MarshalJSON directly means the test covers the batch variables and the
// operation wiring too, not just the generated payloads.
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

	encoded, err := json.Marshal(client.req.Variables)
	if err != nil {
		t.Fatalf("marshalling variables: %v", err)
	}
	return encoded
}

// assertMatchesFixture compares against the fixture's expected.json under the
// specification's baseline profile, which is parsed-JSON equality: the same keys,
// nesting and values, with unset fields absent. Key order is not constrained.
func assertMatchesFixture(t *testing.T, fixture string, got []byte) {
	t.Helper()

	path := filepath.Join("testdata", fixture+".expected.json")
	wantRaw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	var want, have any
	if err := json.Unmarshal(wantRaw, &want); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	if err := json.Unmarshal(got, &have); err != nil {
		t.Fatalf("parsing produced JSON: %v\n%s", err, got)
	}

	if !reflect.DeepEqual(want, have) {
		wantPretty, _ := json.MarshalIndent(want, "", "  ")
		havePretty, _ := json.MarshalIndent(have, "", "  ")
		t.Errorf("wire JSON does not match %s\n\nwant:\n%s\n\ngot:\n%s", path, wantPretty, havePretty)
	}
}

// TestFixtures covers the shared specification fixtures. Each case is written out
// by hand rather than driven from case.json, because Go has to name the
// generated types at compile time; the payloads it names are the committed
// output of the generator, so this still exercises real generated code.
func TestFixtures(t *testing.T) {
	t.Run("001-basic", func(t *testing.T) {
		// Recognition, nesting, and parameter extraction. The operation also
		// fixes a non-variable parameter, which must not become a field.
		got := wireOf(t, f001.AuthCaptureV1Entry{
			Ik:            "ik-1",
			LedgerIk:      "prod",
			UserId:        "user-1",
			CaptureAmount: "100",
		})
		assertMatchesFixture(t, "001-basic", got)
	})

	t.Run("002-type-versions", func(t *testing.T) {
		// Identity is (type, typeVersion), so the same entry type at two
		// versions produces two distinct payloads with different parameters.
		got := wireOf(t,
			f002.UserFundsAccountV1Entry{
				Ik:       "v1",
				LedgerIk: "prod",
				Amount:   "100",
			},
			f002.UserFundsAccountV2Entry{
				Ik:        "v2",
				LedgerIk:  "prod",
				Amount:    "100",
				FeeAmount: "3",
			},
		)
		assertMatchesFixture(t, "002-type-versions", got)
	})

	t.Run("003-reserved-names", func(t *testing.T) {
		// Escaping is local and never reaches the wire. Posted2 carries the
		// "posted" parameter, which collided with the common Posted field; the
		// wire still shows "posted" inside parameters and no entry-level posted.
		got := wireOf(t, f003.ReservedV1Entry{
			Ik:       "r-1",
			LedgerIk: "prod",
			Type:     "t",
			Class:    "c",
			Json:     "j",
			Posted2:  "p",
			UserId:   "u",
		})
		assertMatchesFixture(t, "003-reserved-names", got)
	})

	t.Run("004-param-order", func(t *testing.T) {
		// Parameters keep their source order rather than being sorted or
		// reordered required-first. bravo is optional but sits in the middle.
		got := wireOf(t, f004.OrderedV1Entry{
			Ik:       "o-1",
			LedgerIk: "prod",
			Alpha:    "1",
			Bravo:    ptr("2"),
			Charlie:  "3",
		})
		assertMatchesFixture(t, "004-param-order", got)
	})

	t.Run("005-unset-omitted", func(t *testing.T) {
		// Everything the caller left alone is absent rather than null: the
		// optional parameter, and every unset common field.
		got := wireOf(t, f005.OptionalV1Entry{
			Ik:       "u-1",
			LedgerIk: "prod",
			Required: "yes",
		})
		assertMatchesFixture(t, "005-unset-omitted", got)
	})

	t.Run("006-non-ascii", func(t *testing.T) {
		// Encoding agreement. Under the baseline profile the comparison is on
		// parsed JSON, so Go's escaping of & and <> is not observable, but the
		// decoded value must match exactly.
		got := wireOf(t, f006.TextV1Entry{
			Ik:       "t-1",
			LedgerIk: "prod",
			Memo:     "a&b <tag> café 日本",
		})
		assertMatchesFixture(t, "006-non-ascii", got)
	})
}

// TestParameterOrderIsSourceOrder checks source order on the raw bytes. The
// parsed-JSON comparison in TestFixtures cannot see key order, so without this
// the ordering requirement would be untested.
func TestParameterOrderIsSourceOrder(t *testing.T) {
	got := string(wireOf(t, f004.OrderedV1Entry{
		Ik:       "o-1",
		LedgerIk: "prod",
		Alpha:    "1",
		Bravo:    ptr("2"),
		Charlie:  "3",
	}))

	want := `"parameters":{"alpha":"1","bravo":"2","charlie":"3"}`
	if !contains(got, want) {
		t.Errorf("parameters are not in source order.\nwant substring: %s\ngot: %s", want, got)
	}
}

// TestUnsetIsOmittedNotNull asserts the distinction the specification cares
// about directly: an unset field must not appear at all.
func TestUnsetIsOmittedNotNull(t *testing.T) {
	got := string(wireOf(t, f005.OptionalV1Entry{
		Ik:       "u-1",
		LedgerIk: "prod",
		Required: "yes",
	}))

	if contains(got, "null") {
		t.Errorf("wire JSON should not contain null, got: %s", got)
	}
	// Matched as JSON keys, not bare substrings: this fixture's entry type is
	// itself named "optional", so a substring check would match its value.
	for _, key := range []string{"optional", "posted", "description", "tags", "groups", "conditions", "lines"} {
		if contains(got, `"`+key+`":`) {
			t.Errorf("unset field %q should be absent, got: %s", key, got)
		}
	}
}

// TestEntryOrderPreserved checks that entries arrive in the order given, which
// is what lets a caller match the API's results back to its own inputs.
func TestEntryOrderPreserved(t *testing.T) {
	got := string(wireOf(t,
		f002.UserFundsAccountV2Entry{Ik: "second", LedgerIk: "prod", Amount: "1", FeeAmount: "2"},
		f002.UserFundsAccountV1Entry{Ik: "first", LedgerIk: "prod", Amount: "1"},
	))

	secondAt, firstAt := index(got, `"second"`), index(got, `"first"`)
	if secondAt < 0 || firstAt < 0 {
		t.Fatalf("both iks should appear, got: %s", got)
	}
	if secondAt > firstAt {
		t.Errorf("entries were reordered, got: %s", got)
	}
}

// TestMixingRawAndTypedEntries covers the requirement that a batch may hold both
// kinds of entry. Raw entries go through the generated input type, so unlike
// typed payloads their unset fields are sent as null; that asymmetry is
// deliberate and is what makes a raw entry the way to send an explicit null.
func TestMixingRawAndTypedEntries(t *testing.T) {
	got := string(wireOf(t,
		f001.AuthCaptureV1Entry{Ik: "typed", LedgerIk: "prod", UserId: "u", CaptureAmount: "1"},
		queries.RawEntry{Input: queries.AddLedgerEntryInput{
			Ik: "raw",
			Entry: queries.LedgerEntryInput{
				Ledger: &queries.LedgerMatchInput{Ik: ptr("prod")},
				Type:   ptr("auth_capture"),
			},
		}},
	))

	if !contains(got, `"typed"`) || !contains(got, `"raw"`) {
		t.Errorf("both entries should be present, got: %s", got)
	}
	if !contains(got, "null") {
		t.Errorf("a raw entry is expected to serialize its unset fields as null, got: %s", got)
	}
}

func contains(haystack, needle string) bool { return index(haystack, needle) >= 0 }

func index(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
