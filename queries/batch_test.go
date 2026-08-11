package queries

import (
	"encoding/json"
	"strings"
	"testing"
)

func ptr[T any](v T) *T { return &v }

// TestRawEntryOmitsUnsetFields covers the reason RawEntry does not simply marshal
// the generated input type.
//
// A LedgerMatchInput with only ik set encodes as {"id":null,"ik":"..."}, which the
// API resolves as a different Ledger from the {"ik":"..."} a typed payload sends.
// A batch mixing the two is then rejected for targeting different Ledgers, even
// though both name the same one. Confirmed against a live API.
func TestRawEntryOmitsUnsetFields(t *testing.T) {
	entryType := "auth_capture"
	raw := RawEntry{Input: AddLedgerEntryInput{
		Ik: "raw-1",
		Entry: LedgerEntryInput{
			Ledger: &LedgerMatchInput{Ik: ptr("prod")},
			Type:   &entryType,
		},
	}}

	got, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(got), "null") {
		t.Errorf("unset fields should be omitted, got: %s", got)
	}

	var decoded struct {
		Ik    string `json:"ik"`
		Entry struct {
			Ledger map[string]any `json:"ledger"`
			Type   string         `json:"type"`
		} `json:"entry"`
	}
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("parsing %s: %v", got, err)
	}
	if decoded.Ik != "raw-1" || decoded.Entry.Type != entryType {
		t.Errorf("fields that were set went missing: %s", got)
	}
	if len(decoded.Entry.Ledger) != 1 || decoded.Entry.Ledger["ik"] != "prod" {
		t.Errorf("ledger should be exactly {\"ik\":\"prod\"}, got %v", decoded.Entry.Ledger)
	}
}

// TestRawEntryPreservesParameters checks that the caller's own JSON is passed
// through untouched. Stripping inside parameters would be rewriting a value the
// SDK does not own, where a null may well be meaningful.
func TestRawEntryPreservesParameters(t *testing.T) {
	parameters := json.RawMessage(`{"amount":"100","memo":null,"nested":{"a":null}}`)
	entryType := "t"
	raw := RawEntry{Input: AddLedgerEntryInput{
		Ik: "raw-1",
		Entry: LedgerEntryInput{
			Ledger:     &LedgerMatchInput{Ik: ptr("prod")},
			Type:       &entryType,
			Parameters: &parameters,
		},
	}}

	got, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}

	var decoded struct {
		Entry struct {
			Parameters map[string]any `json:"parameters"`
		} `json:"entry"`
	}
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("parsing %s: %v", got, err)
	}

	if _, ok := decoded.Entry.Parameters["memo"]; !ok {
		t.Errorf("a null the caller put in parameters was stripped: %s", got)
	}
	nested, ok := decoded.Entry.Parameters["nested"].(map[string]any)
	if !ok {
		t.Fatalf("nested parameters went missing: %s", got)
	}
	if _, ok := nested["a"]; !ok {
		t.Errorf("a null nested inside parameters was stripped: %s", got)
	}
}

// TestRawEntryKeepsZeroValues checks that stripping targets null and nothing else.
// An empty string, a zero and an empty list are values the caller chose.
func TestRawEntryKeepsZeroValues(t *testing.T) {
	entryType := ""
	raw := RawEntry{Input: AddLedgerEntryInput{
		Ik: "",
		Entry: LedgerEntryInput{
			Ledger:      &LedgerMatchInput{Ik: ptr("")},
			Type:        &entryType,
			TypeVersion: ptr(0),
			Tags:        []LedgerEntryTagInput{},
		},
	}}

	got, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{`"ik":""`, `"type":""`, `"typeVersion":0`, `"tags":[]`} {
		if !strings.Contains(string(got), want) {
			t.Errorf("expected %s to survive, got: %s", want, got)
		}
	}
}

// TestRawEntryWithLines covers the case RawEntry exists for: an entry with lines
// instead of a type, which no typed payload can express.
func TestRawEntryWithLines(t *testing.T) {
	raw := RawEntry{Input: AddLedgerEntryInput{
		Ik: "raw-1",
		Entry: LedgerEntryInput{
			Ledger: &LedgerMatchInput{Ik: ptr("prod")},
			Lines: []LedgerLineInput{{
				Account: LedgerAccountMatchInput{Path: ptr("assets/cash")},
				Amount:  ptr("100"),
			}},
		},
	}}

	got, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "null") {
		t.Errorf("unset fields should be omitted, got: %s", got)
	}
	if !strings.Contains(string(got), `"amount":"100"`) {
		t.Errorf("the line's amount went missing: %s", got)
	}
	if !strings.Contains(string(got), `"path":"assets/cash"`) {
		t.Errorf("the line's account went missing: %s", got)
	}
}
