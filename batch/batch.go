// Package batch provides the runtime support for typed batch Ledger Entries.
//
// The SDK's code generator derives a typed payload struct for every
// (entry type, typeVersion) pair it finds in your GraphQL operations and emits
// them into a typed_payloads package. Those payloads implement [Entry] and can
// be passed to [github.com/fragment-dev/fragment-go/v4/queries.AddTypedLedgerEntries]
// to be committed as one atomic batch.
//
// Nothing here is generated, and you should not normally need to use [Object]
// directly; the generated payloads depend on it.
package batch

import (
	"bytes"
	"encoding/json"
)

// Entry is a single element of a batch. It is implemented by the generated
// typed payloads and by
// [github.com/fragment-dev/fragment-go/v4/queries.RawEntry].
//
// FragmentBatchEntry is a marker, so that an arbitrary [encoding/json.Marshaler]
// cannot be passed to a batch by accident. It is exported only because generated
// payloads live outside this package and so cannot implement an unexported
// method.
type Entry interface {
	json.Marshaler
	FragmentBatchEntry()
}

// Object builds a JSON object while preserving the order in which fields are
// added, and omitting fields that were never set.
//
// Field order matters for parameters: the spec the generator implements requires
// them to appear in the order they occur in the source operation, which a Go map
// cannot express. Omission matters everywhere: a field the caller did not set
// must be absent rather than null, so that an explicitly-null value stays
// distinguishable from an unset one. encoding/json's omitempty cannot express
// that, since it also drops "", 0 and false.
//
// Optional and slice fields are set with [SetOpt] and [SetSlice], which are
// functions rather than methods because Go does not allow type parameters on
// methods.
//
// Errors are accumulated rather than returned per call, in the manner of
// [strings.Builder], and surfaced by [Object.MarshalJSON]. An Object is a
// [encoding/json.Marshaler], so nesting one inside another is just
// [Object.Set].
type Object struct {
	buf   bytes.Buffer
	n     int
	err   error
	ended bool
}

// NewObject returns an empty Object ready to accept fields.
func NewObject() *Object {
	o := &Object{}
	o.buf.WriteByte('{')
	return o
}

func (o *Object) key(name string) bool {
	if o.err != nil {
		return false
	}
	if o.ended {
		// The object has already been encoded, so appending would write past the
		// closing brace and silently produce invalid JSON.
		o.err = errAfterMarshal
		return false
	}
	if o.n > 0 {
		o.buf.WriteByte(',')
	}
	o.n++
	k, err := json.Marshal(name)
	if err != nil {
		o.err = err
		return false
	}
	o.buf.Write(k)
	o.buf.WriteByte(':')
	return true
}

// Set adds a field unconditionally. Use it for fields that are always present,
// such as an entry's type and typeVersion, and to nest another Object.
func (o *Object) Set(name string, v any) {
	if !o.key(name) {
		return
	}
	b, err := json.Marshal(v)
	if err != nil {
		o.err = err
		return
	}
	o.buf.Write(b)
}

// SetSlice adds a field only when the slice is non-nil. An empty non-nil slice
// is still written, as [], because the caller asked for it; a nil slice is
// treated as unset and omitted.
//
// Null members are stripped from the encoding. The element types these slices hold
// are generated without omitempty, so marshalling them directly fills in every
// field the caller left unset. That is not cosmetic: a Ledger Line's account is a
// match input, and the API resolves {"id":null,"path":"x"} as a different Account
// from {"path":"x"}.
func SetSlice[T any](o *Object, name string, v []T) {
	if v == nil {
		return
	}

	encoded, err := json.Marshal(v)
	if err != nil {
		o.err = err
		return
	}
	cleaned, err := StripNulls(encoded)
	if err != nil {
		o.err = err
		return
	}
	o.Set(name, json.RawMessage(cleaned))
}

// StripNulls removes null members from every JSON object in data, recursively. The
// value of any field named in passthrough is kept verbatim, for JSON the caller
// owns and this package has no business rewriting.
func StripNulls(data []byte, passthrough ...string) ([]byte, error) {
	keep := make(map[string]bool, len(passthrough))
	for _, name := range passthrough {
		keep[name] = true
	}
	return stripNulls(data, keep)
}

func stripNulls(data json.RawMessage, passthrough map[string]bool) (json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err == nil && object != nil {
		kept := make(map[string]json.RawMessage, len(object))
		for name, value := range object {
			if string(value) == "null" {
				continue
			}
			if passthrough[name] {
				kept[name] = value
				continue
			}
			cleaned, err := stripNulls(value, passthrough)
			if err != nil {
				return nil, err
			}
			kept[name] = cleaned
		}
		return json.Marshal(kept)
	}

	var array []json.RawMessage
	if err := json.Unmarshal(data, &array); err == nil && array != nil {
		for i, element := range array {
			cleaned, err := stripNulls(element, passthrough)
			if err != nil {
				return nil, err
			}
			array[i] = cleaned
		}
		return json.Marshal(array)
	}

	// A scalar, or JSON this function has no business rewriting.
	return data, nil
}

// SetOpt adds a field only when the pointer is non-nil. A nil pointer means the
// caller did not set the field, so it is omitted rather than written as null.
func SetOpt[T any](o *Object, name string, v *T) {
	if v == nil {
		return
	}
	o.Set(name, *v)
}

// MarshalJSON closes the object and returns its encoding, or the first error any
// field encountered.
func (o *Object) MarshalJSON() ([]byte, error) {
	if o.err != nil {
		return nil, o.err
	}
	if !o.ended {
		o.buf.WriteByte('}')
		o.ended = true
	}
	return o.buf.Bytes(), nil
}

type marshalError string

func (e marshalError) Error() string { return string(e) }

const errAfterMarshal marshalError = "batch: Object was modified after being marshalled"
