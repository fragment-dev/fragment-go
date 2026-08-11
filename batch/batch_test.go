package batch

import "testing"

func TestObjectPreservesInsertionOrder(t *testing.T) {
	o := NewObject()
	o.Set("zulu", "1")
	o.Set("alpha", "2")
	o.Set("mike", "3")

	got, err := o.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	want := `{"zulu":"1","alpha":"2","mike":"3"}`
	if string(got) != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestSetOptOmitsNilAndKeepsZeroValues(t *testing.T) {
	// The distinction encoding/json's omitempty cannot make: an empty string, a
	// zero and a false are values the caller chose, while nil is the absence of
	// a choice.
	empty := ""
	zero := 0
	no := false

	o := NewObject()
	SetOpt[string](o, "unset", nil)
	SetOpt(o, "empty", &empty)
	SetOpt(o, "zero", &zero)
	SetOpt(o, "false", &no)

	got, err := o.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	want := `{"empty":"","zero":0,"false":false}`
	if string(got) != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestSetSliceDistinguishesNilFromEmpty(t *testing.T) {
	// A nil slice is unset and omitted; an empty one was explicitly provided and
	// is sent as [].
	o := NewObject()
	SetSlice[string](o, "unset", nil)
	SetSlice(o, "empty", []string{})
	SetSlice(o, "full", []string{"a"})

	got, err := o.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	want := `{"empty":[],"full":["a"]}`
	if string(got) != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestEmptyObject(t *testing.T) {
	got, err := NewObject().MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "{}" {
		t.Errorf("got %s, want {}", got)
	}
}

func TestSetEmbedsNestedObject(t *testing.T) {
	// An Object is a json.Marshaler, so nesting is just Set.
	inner := NewObject()
	inner.Set("ik", "prod")

	outer := NewObject()
	outer.Set("ledger", inner)
	outer.Set("type", "t")

	got, err := outer.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	want := `{"ledger":{"ik":"prod"},"type":"t"}`
	if string(got) != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestMarshalJSONIsIdempotent(t *testing.T) {
	// Closing the brace twice would produce invalid JSON.
	o := NewObject()
	o.Set("a", 1)

	first, err := o.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	second, err := o.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Errorf("second call returned %s, first returned %s", second, first)
	}
}

func TestMarshalErrorIsReported(t *testing.T) {
	o := NewObject()
	o.Set("bad", func() {}) // channels and funcs cannot be marshalled
	if _, err := o.MarshalJSON(); err == nil {
		t.Error("expected an error for an unmarshallable value")
	}
}

func TestSetAfterMarshalIsAnError(t *testing.T) {
	// Appending after the closing brace has been written would produce invalid
	// JSON, so it must be reported rather than silently corrupt the output.
	o := NewObject()
	o.Set("a", 1)
	if _, err := o.MarshalJSON(); err != nil {
		t.Fatal(err)
	}

	o.Set("b", 2)
	got, err := o.MarshalJSON()
	if err == nil {
		t.Errorf("expected an error, got %s", got)
	}
}

func TestErrorSuppressesLaterFields(t *testing.T) {
	// Once a field has failed the buffer is not trustworthy, so MarshalJSON must
	// report the error rather than return partial JSON.
	o := NewObject()
	o.Set("bad", func() {})
	o.Set("good", "value")
	if _, err := o.MarshalJSON(); err == nil {
		t.Error("expected the earlier error to persist")
	}
}
