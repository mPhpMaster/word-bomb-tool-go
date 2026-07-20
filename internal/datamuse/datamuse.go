// Package datamuse is a small client for the Datamuse words API. It is the Go
// port of api_client.py.
package datamuse

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mphpmaster/word-bomb-tool-go/internal/config"
)

// Client talks to the Datamuse API and tracks the status of the last call using
// the same human-readable indicators as the Python app. It is safe for
// concurrent use.
type Client struct {
	http    *http.Client
	baseURL string

	mu     sync.Mutex
	status string
}

// New returns a ready-to-use client.
func New() *Client {
	return &Client{
		http:    &http.Client{Timeout: time.Duration(config.OCRTimeout) * time.Second},
		status:  config.StatusOnline,
		baseURL: config.DatamuseAPI,
	}
}

// SetBaseURL overrides the API endpoint (used in tests).
func (c *Client) SetBaseURL(u string) { c.baseURL = u }

// Status returns the status of the most recent request.
func (c *Client) Status() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.status
}

func (c *Client) setStatus(s string) {
	c.mu.Lock()
	c.status = s
	c.mu.Unlock()
}

type wordItem struct {
	Word string   `json:"word"`
	Defs []string `json:"defs"`
}

// Suggestions fetches word suggestions for the given letters and search mode.
// It returns an empty slice on any error (matching the Python behaviour) and
// updates Status accordingly.
func (c *Client) Suggestions(letters, mode string) []string {
	if len(letters) < 1 {
		return nil
	}

	q := url.Values{}
	q.Set("max", strconv.Itoa(config.MaxSuggestionsDisplay))
	switch mode {
	case "Starts With":
		q.Set("sp", letters+"*")
	case "Ends With":
		q.Set("sp", "*"+letters)
	case "Contains":
		q.Set("sp", "*"+letters+"*")
	case "Rhymes":
		q.Set("rel_rhy", letters)
	case "Related Words":
		q.Set("rel_jja", letters)
	}

	var items []wordItem
	if err := c.get(q, &items); err != nil {
		return nil
	}

	out := make([]string, 0, len(items))
	for _, it := range items {
		// Keep single-word results only (no spaces), as in the original.
		if len(strings.Fields(it.Word)) == 1 {
			out = append(out, it.Word)
		}
	}
	if len(out) > config.MaxSuggestionsDisplay {
		out = out[:config.MaxSuggestionsDisplay]
	}
	c.setStatus(config.StatusOnline)
	return out
}

// Definitions fetches the definitions for a word. Returns an empty slice on any
// error and updates Status accordingly.
func (c *Client) Definitions(word string) []string {
	if len(word) < 1 {
		return nil
	}

	q := url.Values{}
	q.Set("sp", word)
	q.Set("qe", "sp")
	q.Set("md", "d")
	q.Set("max", "1")

	var items []wordItem
	if err := c.get(q, &items); err != nil {
		return nil
	}

	c.setStatus(config.StatusOnline)
	if len(items) > 0 {
		return items[0].Defs
	}
	return nil
}

// get performs the HTTP GET, decodes the JSON body into out and maps transport
// errors onto the status string. It returns a non-nil error whenever the caller
// should treat the result as empty.
func (c *Client) get(q url.Values, out interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.OCRTimeout)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"?"+q.Encode(), nil)
	if err != nil {
		c.setStatus(config.StatusError)
		return err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			c.setStatus(config.StatusTimeout)
		} else {
			c.setStatus(config.StatusOffline)
		}
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.setStatus(config.StatusError)
		return errors.New("datamuse: unexpected status " + resp.Status)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		c.setStatus(config.StatusError)
		return err
	}
	return nil
}
