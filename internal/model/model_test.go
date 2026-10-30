package model

import "testing"

func TestSameContentIgnoresBlankRowsAndIDs(t *testing.T) {
	saved := Request{
		ID: "a", Name: "Things", Method: "GET", URL: "https://example.com",
		Headers: []KV{{ID: "h1", Enabled: true, Key: "Accept", Value: "*/*"}},
	}
	draft := saved.Clone()
	draft.ID = "b"
	draft.Params = append(draft.Params, NewKV())
	draft.Headers = append(draft.Headers, NewKV())
	if !draft.SameContent(saved) {
		t.Error("a blank row marked the request as edited")
	}

	draft.Headers[0].Enabled = false
	if draft.SameContent(saved) {
		t.Error("turning a header off went unnoticed")
	}
	draft.Headers[0].Enabled = true

	draft.Name = "Other things"
	if draft.SameContent(saved) {
		t.Error("a new name went unnoticed")
	}
	draft.Name = saved.Name

	draft.Form = []KV{{ID: "f", Enabled: true, Key: "a", Value: "1"}}
	if draft.SameContent(saved) {
		t.Error("a form field went unnoticed")
	}
}

func TestNormalizeTurnsAFormWrittenAsTextIntoRows(t *testing.T) {
	r := Request{BodyKind: BodyForm, Body: "name=two+words&a%26b=x%3Dy&flag"}
	r.Normalize()
	if r.Body != "" {
		t.Errorf("body %q stayed beside the rows", r.Body)
	}
	want := []KV{
		{Enabled: true, Key: "name", Value: "two words"},
		{Enabled: true, Key: "a&b", Value: "x=y"},
		{Enabled: true, Key: "flag", Value: ""},
	}
	if len(r.Form) != len(want) {
		t.Fatalf("rows %+v, want %+v", r.Form, want)
	}
	for i, kv := range r.Form {
		if kv.ID == "" || kv.Enabled != want[i].Enabled || kv.Key != want[i].Key || kv.Value != want[i].Value {
			t.Errorf("row %d is %+v, want %+v", i, kv, want[i])
		}
	}

	// Anything else is left alone: a JSON body, a form that has rows, and
	// text that is not a form.
	for _, r := range []Request{
		{BodyKind: BodyJSON, Body: "a=1"},
		{BodyKind: BodyForm, Body: "a=1", Form: []KV{{Enabled: true, Key: "b", Value: "2"}}},
		{BodyKind: BodyForm, Body: "a=%zz"},
	} {
		before := r.Clone()
		r.Normalize()
		if r.Body != before.Body || len(r.Form) != len(before.Form) {
			t.Errorf("Normalize changed %+v into %+v", before, r)
		}
	}
}
