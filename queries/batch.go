package queries

import (
	"context"
	"encoding/json"

	"github.com/Khan/genqlient/graphql"

	"github.com/fragment-dev/fragment-go/v4/batch"
)

// RawEntry adapts an untyped AddLedgerEntryInput so it can be mixed into a
// batch alongside generated typed payloads.
//
// Raw entries serialize through the generated input type, so unlike typed
// payloads their unset fields are sent as null rather than omitted. That is the
// escape hatch: use a raw entry when you need to send an explicit null, and a
// typed payload otherwise.
type RawEntry struct {
	Input AddLedgerEntryInput
}

// FragmentBatchEntry marks RawEntry as usable in a batch.
func (RawEntry) FragmentBatchEntry() {}

// MarshalJSON encodes the wrapped input as-is.
func (r RawEntry) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.Input)
}

type typedBatchVariables struct {
	Entries []batch.Entry `json:"entries"`
}

// AddTypedLedgerEntries commits a batch of Ledger Entries atomically.
//
// Pass generated typed payloads, RawEntry values, or a mix of both. Entries are
// posted in the order given and the API returns results in that same order.
//
// The batch is all-or-nothing: on error nothing was written, so there is no
// partial state to reconcile. Idempotency keys are per entry rather than per
// batch, so retrying a batch that partly succeeded reports IsIkReplay against
// the individual entries that had already committed.
//
// The response is a union. Narrow it before reading results:
//
//	resp, err := queries.AddTypedLedgerEntries(ctx, client, first, second)
//	if err != nil {
//		return err
//	}
//	switch r := resp.GetAddLedgerEntries().(type) {
//	case *queries.AddLedgerEntriesAddLedgerEntriesAddLedgerEntriesResult:
//		for _, result := range r.Results {
//			fmt.Println(result.Entry.Ik, result.IsIkReplay)
//		}
//	case *queries.AddLedgerEntriesAddLedgerEntriesAddLedgerEntriesError:
//		// Errors carries one element per failing entry, each with the ik that
//		// identifies which entry to fix.
//		for _, e := range r.Errors {
//			fmt.Println(e.Ik, e.Message)
//		}
//	}
func AddTypedLedgerEntries(
	ctx_ context.Context,
	client_ graphql.Client,
	entries ...batch.Entry,
) (data_ *AddLedgerEntriesResponse, err_ error) {
	if entries == nil {
		entries = []batch.Entry{}
	}

	req_ := &graphql.Request{
		OpName: "AddLedgerEntries",
		Query:  AddLedgerEntries_Operation,
		Variables: &typedBatchVariables{
			Entries: entries,
		},
	}

	data_ = &AddLedgerEntriesResponse{}
	resp_ := &graphql.Response{Data: data_}

	err_ = client_.MakeRequest(
		ctx_,
		req_,
		resp_,
	)

	return data_, err_
}
