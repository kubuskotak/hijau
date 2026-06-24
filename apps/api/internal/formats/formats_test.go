package formats

import (
	"reflect"
	"testing"
)

func asMap(entries []Entry) map[string]string {
	m := map[string]string{}
	for _, e := range entries {
		m[e.Key] = e.Value
	}
	return m
}

func TestRoundTripSimple(t *testing.T) {
	entries := []Entry{
		{"app.title", "Welcome"},
		{"app.cta", "Sign up now"},
		{"greeting", "Hi there"},
		{"cart.empty", "Your cart is empty"},
	}
	want := asMap(entries)
	for _, id := range IDs() {
		f, _ := Get(id)
		data, err := f.Marshal(entries)
		if err != nil {
			t.Fatalf("%s marshal: %v", id, err)
		}
		got, err := f.Unmarshal(data)
		if err != nil {
			t.Fatalf("%s unmarshal: %v", id, err)
		}
		if !reflect.DeepEqual(want, asMap(got)) {
			t.Fatalf("%s round-trip mismatch:\n got %v\nwant %v\n---\n%s", id, asMap(got), want, data)
		}
	}
}

func TestRoundTripSpecialChars(t *testing.T) {
	// quotes, ampersand/angle brackets, comma, newline, ICU placeholder.
	entries := []Entry{
		{"a.quote", `He said "hi"`},
		{"a.amp", "Tom & Jerry <ok>"},
		{"a.comma", "one, two, three"},
		{"a.icu", "Hello {name}, you have {count} messages"},
	}
	want := asMap(entries)
	for _, id := range IDs() {
		f, _ := Get(id)
		data, err := f.Marshal(entries)
		if err != nil {
			t.Fatalf("%s marshal: %v", id, err)
		}
		got, err := f.Unmarshal(data)
		if err != nil {
			t.Fatalf("%s unmarshal: %v", id, err)
		}
		if !reflect.DeepEqual(want, asMap(got)) {
			t.Fatalf("%s special-char round-trip mismatch:\n got %v\nwant %v\n---\n%s", id, asMap(got), want, data)
		}
	}
}

func TestNestedJSONFlattens(t *testing.T) {
	f, _ := Get("json-nested")
	got, err := f.Unmarshal([]byte(`{"cart":{"checkout":{"button":"Pay"}},"home":"Home"}`))
	if err != nil {
		t.Fatal(err)
	}
	m := asMap(got)
	if m["cart.checkout.button"] != "Pay" || m["home"] != "Home" {
		t.Fatalf("flatten mismatch: %v", m)
	}
}

func TestFlatJSONRejectsNested(t *testing.T) {
	f, _ := Get("json")
	if _, err := f.Unmarshal([]byte(`{"a":{"b":"c"}}`)); err == nil {
		t.Fatal("expected flat JSON to reject a nested object")
	}
}

func TestUnknownFormat(t *testing.T) {
	if _, ok := Get("nope"); ok {
		t.Fatal("expected unknown format to be absent")
	}
}
