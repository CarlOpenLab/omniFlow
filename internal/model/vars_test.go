package model

import (
	"regexp"
	"slices"
	"testing"
)

func TestResolvedFillsWhatIsSent(t *testing.T) {
	env := Environment{Vars: []KV{
		{Enabled: true, Key: "host", Value: "api.staging.example.com"},
		{Enabled: true, Key: "token", Value: "s3cret"},
		{Enabled: false, Key: "off", Value: "never"},
		{Enabled: true, Key: "host", Value: "api.example.com"}, // a later row wins
	}}
	r := Request{
		Method: "POST", URL: "https://{{ host }}/v1/{{missing}}",
		Params:   []KV{{Enabled: true, Key: "id", Value: "{{$uuid}}"}, {Enabled: false, Key: "x", Value: "{{off_row}}"}},
		Headers:  []KV{{Enabled: true, Key: "X-Off", Value: "{{off}}"}},
		BodyKind: BodyJSON, Body: `{"at":"{{$timestamp}}"}`,
		AuthType: AuthBearer, Token: "{{token}}",
		User: "{{not_sent}}", // Basic is not the auth in use
	}
	res := Resolver{Values: env.Values()}
	got := r.Resolved(&res)

	if got.URL != "https://api.example.com/v1/{{missing}}" {
		t.Errorf("URL %q", got.URL)
	}
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(got.Params[0].Value) {
		t.Errorf("$uuid became %q", got.Params[0].Value)
	}
	if got.Params[1].Value != "{{off_row}}" {
		t.Errorf("a row that is off was filled in: %q", got.Params[1].Value)
	}
	if !regexp.MustCompile(`^\{"at":"\d+"\}$`).MatchString(got.Body) {
		t.Errorf("body %q", got.Body)
	}
	if got.Token != "s3cret" || got.User != "{{not_sent}}" {
		t.Errorf("auth %q / %q", got.Token, got.User)
	}
	// "off" is a variable whose row is off: it is missing, as an unknown one is.
	if want := []string{"missing", "off"}; !slices.Equal(res.Missing, want) {
		t.Errorf("missing %q, want %q", res.Missing, want)
	}
	if r.URL != "https://{{ host }}/v1/{{missing}}" || r.Params[0].Value != "{{$uuid}}" {
		t.Error("Resolved changed the request it was given")
	}
}

func TestPlaceholdersAreCountedInRunes(t *testing.T) {
	got := Placeholders("é/{{a}}/{{ b.c }}/{{")
	want := []Placeholder{{Name: "a", Start: 2, End: 7}, {Name: "b.c", Start: 8, End: 17}}
	if !slices.Equal(got, want) {
		t.Errorf("Placeholders = %+v, want %+v", got, want)
	}
	if Placeholders("no braces") != nil {
		t.Error("a text without braces has placeholders")
	}
}

func TestVariablesListsEachNameOnce(t *testing.T) {
	r := Request{
		URL:      "{{base}}/users/{{id}}",
		Headers:  []KV{{Enabled: true, Key: "Authorization", Value: "Bearer {{token}}"}},
		BodyKind: BodyText, Body: "{{id}} {{$uuid}}",
	}
	if got, want := r.Variables(), []string{"base", "id", "token", "$uuid"}; !slices.Equal(got, want) {
		t.Errorf("Variables = %q, want %q", got, want)
	}
}
