package batch

import "testing"

func TestObjectPreservesInsertionOrder(t *testing.T) {
	o := NewObject()
	o.Set("zulu", "1")
	o.Set("alpha", "2")
	o.Set("mike", "3")

	got, err := o.Bytes()
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

	got, err := o.Bytes()
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

	got, err := o.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	want := `{"empty":[],"full":["a"]}`
	if string(got) != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestEmptyObject(t *testing.T) {
	got, err := NewObject().Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "{}" {
		t.Errorf("got %s, want {}", got)
	}
}

func TestSetRawEmbedsNestedObject(t *testing.T) {
	inner := NewObject()
	inner.Set("ik", "prod")
	innerJSON, err := inner.Bytes()
	if err != nil {
		t.Fatal(err)
	}

	outer := NewObject()
	outer.SetRaw("ledger", innerJSON)
	outer.Set("type", "t")

	got, err := outer.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	want := `{"ledger":{"ik":"prod"},"type":"t"}`
	if string(got) != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestBytesIsIdempotent(t *testing.T) {
	// A generated MarshalJSON may read Bytes more than once via a nested object;
	// closing the brace twice would produce invalid JSON.
	o := NewObject()
	o.Set("a", 1)

	first, err := o.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	second, err := o.Bytes()
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
	if _, err := o.Bytes(); err == nil {
		t.Error("expected an error for an unmarshallable value")
	}
}

func TestErrorSuppressesLaterFields(t *testing.T) {
	// Once a field has failed the buffer is not trustworthy, so Bytes must
	// report the error rather than return partial JSON.
	o := NewObject()
	o.Set("bad", func() {})
	o.Set("good", "value")
	if _, err := o.Bytes(); err == nil {
		t.Error("expected the earlier error to persist")
	}
}
