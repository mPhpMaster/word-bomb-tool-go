package datamuse

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mphpmaster/word-bomb-tool-go/internal/config"
)

func TestSuggestionsParsingAndSingleWordFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("sp"); got != "cat*" {
			t.Errorf("expected sp=cat*, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		// "cat nap" has a space and must be filtered out.
		_, _ = w.Write([]byte(`[{"word":"cat"},{"word":"cats"},{"word":"cat nap"},{"word":"catalog"}]`))
	}))
	defer srv.Close()

	c := New()
	c.SetBaseURL(srv.URL)
	got := c.Suggestions("cat", "Starts With")
	want := []string{"cat", "cats", "catalog"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	if c.Status() != config.StatusOnline {
		t.Errorf("status = %q, want online", c.Status())
	}
}

func TestDefinitions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"word":"puzzle","defs":["n\tsomething confusing","v\tto perplex"]}]`))
	}))
	defer srv.Close()

	c := New()
	c.SetBaseURL(srv.URL)
	defs := c.Definitions("puzzle")
	if len(defs) != 2 || defs[0] != "n\tsomething confusing" {
		t.Fatalf("unexpected defs: %v", defs)
	}
}

func TestSuggestionsServerErrorSetsStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := New()
	c.SetBaseURL(srv.URL)
	if got := c.Suggestions("cat", "Contains"); got != nil {
		t.Errorf("expected nil on server error, got %v", got)
	}
	if c.Status() != config.StatusError {
		t.Errorf("status = %q, want error", c.Status())
	}
}
