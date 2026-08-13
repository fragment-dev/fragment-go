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

func TestSetSliceStripsNulls(t *testing.T) {
	// Stands in for a generated input struct: no omitempty, so every unset field
	// marshals as null unless something removes it.
	type account struct {
		Id   *string `json:"id"`
		Path *string `json:"path"`
	}
	type line struct {
		Account  account `json:"account"`
		Amount   *string `json:"amount"`
		Currency *string `json:"currency"`
	}

	path, amount := "assets/cash", "100"

	o := NewObject()
	SetSlice(o, "lines", []line{{Account: account{Path: &path}, Amount: &amount}})

	got, err := o.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	want := `{"lines":[{"account":{"path":"assets/cash"},"amount":"100"}]}`
	if string(got) != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestStripNulls(t *testing.T) {
	cases := map[string]struct {
		in, want    string
		passthrough []string
	}{
		"removes null members": {
			in:   `{"a":1,"b":null}`,
			want: `{"a":1}`,
		},
		"recurses into objects": {
			in:   `{"outer":{"a":null,"b":2}}`,
			want: `{"outer":{"b":2}}`,
		},
		"recurses into arrays": {
			in:   `[{"a":null,"b":1},{"c":null}]`,
			want: `[{"b":1},{}]`,
		},
		"keeps zero values": {
			in:   `{"empty":"","zero":0,"false":false,"list":[]}`,
			want: `{"empty":"","false":false,"list":[],"zero":0}`,
		},
		// A null the caller put inside their own JSON is theirs to keep.
		"passthrough is untouched": {
			in:          `{"a":null,"parameters":{"memo":null}}`,
			want:        `{"parameters":{"memo":null}}`,
			passthrough: []string{"parameters"},
		},
		"a bare null is left alone": {
			// Nothing names it, so there is no member to remove.
			in:   `null`,
			want: `null`,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := StripNulls([]byte(tc.in), tc.passthrough...)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}
