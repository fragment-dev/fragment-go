// Package batch provides the runtime support for typed batch Ledger Entries.
//
// The SDK's code generator derives a typed payload struct for every
// (entry type, typeVersion) pair it finds in your GraphQL operations and emits
// them into a typed_payloads package. Those payloads implement Entry and can be
// passed to queries.AddTypedLedgerEntries to be committed as one atomic batch.
//
// Nothing in this package is generated; the generated payloads depend on it.
package batch

import (
	"bytes"
	"encoding/json"
)

// Entry is a single element of a batch. It is implemented by the generated
// typed payloads and by queries.RawEntry.
//
// FragmentBatchEntry is a marker so that an arbitrary json.Marshaler cannot be
// passed to a batch by accident. It is exported only because generated payloads
// live outside this package and so cannot implement an unexported method.
type Entry interface {
	json.Marshaler
	FragmentBatchEntry()
}

// Object builds a JSON object while preserving the order in which fields are
// added, and omitting fields that were never set.
//
// Field order matters for parameters: the spec requires them to appear in the
// order they occur in the source operation, which a Go map cannot express.
// Omission matters everywhere: a field the caller did not set must be absent
// rather than null, so that an explicitly-null value stays distinguishable from
// an unset one. encoding/json's omitempty cannot express that, since it also
// drops "" , 0 and false.
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
// such as an entry's type and typeVersion.
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

// SetRaw adds a field whose value is already encoded JSON, such as a nested
// Object's Bytes.
func (o *Object) SetRaw(name string, raw []byte) {
	if !o.key(name) {
		return
	}
	o.buf.Write(raw)
}

// SetSlice adds a field only when the slice is non-nil. An empty non-nil slice
// is still written, as [], because the caller asked for it; a nil slice is
// treated as unset and omitted.
func SetSlice[T any](o *Object, name string, v []T) {
	if v == nil {
		return
	}
	o.Set(name, v)
}

// SetOpt adds a field only when the pointer is non-nil. A nil pointer means the
// caller did not set the field, so it is omitted rather than written as null.
func SetOpt[T any](o *Object, name string, v *T) {
	if v == nil {
		return
	}
	o.Set(name, *v)
}

// Bytes closes the object and returns its encoding.
func (o *Object) Bytes() ([]byte, error) {
	if o.err != nil {
		return nil, o.err
	}
	if !o.ended {
		o.buf.WriteByte('}')
		o.ended = true
	}
	return o.buf.Bytes(), nil
}
