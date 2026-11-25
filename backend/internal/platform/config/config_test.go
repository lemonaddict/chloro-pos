package config

import (
	"slices"
	"testing"
)

func TestParseOriginsNormalizes(t *testing.T) {
	got, err := parseOrigins(" https://App.Example.com/ , http://localhost:5173,https://api.example.com:8443 ,, ")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"https://app.example.com", "http://localhost:5173", "https://api.example.com:8443"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseOriginsEmpty(t *testing.T) {
	got, err := parseOrigins("")
	if err != nil || len(got) != 0 {
		t.Errorf("an empty list means same-origin only: got %v, %v", got, err)
	}
}

func TestParseOriginsRejects(t *testing.T) {
	for _, bad := range []string{
		"*",
		"https://*.example.com",
		"app.example.com",             // no scheme
		"ftp://app.example.com",       // not http(s)
		"https://app.example.com/app", // a path is not part of an origin
		"https://app.example.com?x=1",
		"https://user:pw@app.example.com",
		"https://",
	} {
		if _, err := parseOrigins(bad); err == nil {
			t.Errorf("%q must be rejected", bad)
		}
	}
}

func TestHTTPAddr(t *testing.T) {
	cases := []struct {
		name, httpAddr, port, env, want string
	}{
		{"default", "", "", "development", ":8080"},
		{"explicit address wins", ":9000", "10000", "production", ":9000"},
		{"production follows PORT (Render)", "", "10000", "production", ":10000"},
		{"development ignores a stray PORT", "", "3000", "development", ":8080"},
		{"production without PORT", "", "", "production", ":8080"},
	}
	for _, c := range cases {
		t.Setenv("HTTP_ADDR", c.httpAddr)
		t.Setenv("PORT", c.port)
		if got := httpAddr(c.env); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
