// Package api is the client for the vedabase.cc read-only JSON API.
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Item is one readable text. Missing parts arrive as JSON null and decode to "".
type Item struct {
	Ref         string      `json:"ref"`
	Lang        string      `json:"lang"`
	Book        string      `json:"book"`
	URL         string      `json:"url"`
	Devanagari  string      `json:"devanagari"`
	VerseText   string      `json:"verse_text"`
	Synonyms    string      `json:"synonyms"`
	Translation string      `json:"translation"`
	Purport     string      `json:"purport"`
	Format      string      `json:"format"`
	Prev        string      `json:"prev"`
	Next        string      `json:"next"`
	References  []Reference `json:"references"`
	Document    *Document   `json:"document"`
}

// Reference is one citation found in a text. Text is the exact string as it
// appears in Field; Ref is empty when the work is not on the site.
type Reference struct {
	Text  string `json:"text"`
	Ref   string `json:"ref"`
	Field string `json:"field"`
}

// Document is the extra metadata carried by letters and transcripts.
type Document struct {
	Type       string          `json:"type"`
	Date       string          `json:"date"`
	IsoDate    string          `json:"iso_date"` // 1968-03-17, 1968-03 or 1966
	Year       int             `json:"year"`     // null decodes to 0
	Location   string          `json:"location"`
	Recipients []string        `json:"recipients"`
	AudioURL   string          `json:"audio_url"`
	Address    string          `json:"address"`
	Covers     json.RawMessage `json:"covers"`
}

type Book struct {
	Code      string   `json:"code"`
	Title     string   `json:"title"`
	Type      string   `json:"type"`
	Languages []string `json:"languages"`
}

// SearchResult snippets contain <mark>…</mark>, the only HTML in the API.
type SearchResult struct {
	Ref                string `json:"ref"`
	Book               string `json:"book"`
	BookTitle          string `json:"book_title"`
	URL                string `json:"url"`
	Translation        string `json:"translation"`
	SnippetTranslation string `json:"snippet_translation"`
	SnippetPurport     string `json:"snippet_purport"`
}

type SearchPage struct {
	Query   string         `json:"query"`
	Book    string         `json:"book"`
	Page    int            `json:"page"`
	PerPage int            `json:"per_page"`
	HasMore bool           `json:"has_more"`
	Results []SearchResult `json:"results"`
}

// Parts selects which sections of a verse to show or export.
type Parts struct {
	Devanagari  bool `json:"devanagari"`
	Verse       bool `json:"verse"`
	Synonyms    bool `json:"synonyms"`
	Translation bool `json:"translation"`
	Purport     bool `json:"purport"`
}

func AllParts() Parts {
	return Parts{Devanagari: true, Verse: true, Synonyms: true, Translation: true, Purport: true}
}

// APIError is an error reported by the server (or an unexpected HTTP status).
type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("HTTP %d", e.Status)
}

// RateLimitedError is returned on a 429; callers decide how to wait.
type RateLimitedError struct{ RetryAfter time.Duration }

func (e *RateLimitedError) Error() string {
	return fmt.Sprintf("rate limited, retry in %s", e.RetryAfter)
}

// ErrOffline wraps any transport-level failure.
var ErrOffline = errors.New("offline")

func IsNotFound(err error) bool {
	var ae *APIError
	return errors.As(err, &ae) && (ae.Status == 404 || ae.Code == "not_found")
}

// TOC is a book's table of contents in site order.
type TOC struct {
	Book     string    `json:"book"`
	Lang     string    `json:"lang"`
	Title    string    `json:"title"`
	Children []TOCNode `json:"children,omitempty"`
}

// TOCNode is one entry: a container (Title, Children) or a leaf (Label, and
// sometimes Title, as in front matter). JSON null decodes to "" and nil.
type TOCNode struct {
	Ref      string    `json:"ref"`
	Title    string    `json:"title,omitempty"`
	Label    string    `json:"label,omitempty"`
	Children []TOCNode `json:"children,omitempty"`
}

// YearCount is one entry of a listing's year index.
type YearCount struct {
	Year  int `json:"year"`
	Count int `json:"count"`
}

// ListingEntry is one letter or transcript in a listing.
type ListingEntry struct {
	Ref        string   `json:"ref"`
	Title      string   `json:"title"`
	IsoDate    string   `json:"iso_date"`
	Year       int      `json:"year"`
	Location   string   `json:"location"`
	Recipients []string `json:"recipients,omitempty"`
}

// Listing is one page of /letters or /transcripts. Years and Undated describe
// the whole collection, whatever the page.
type Listing struct {
	Total   int            `json:"total"`
	Page    int            `json:"page"`
	PerPage int            `json:"per_page"`
	HasMore bool           `json:"has_more"`
	Undated int            `json:"undated"`
	Years   []YearCount    `json:"years"`
	Items   []ListingEntry `json:"items"`
}

func sanitizeTOCNodes(ns []TOCNode) {
	for i := range ns {
		n := &ns[i]
		n.Ref, n.Title, n.Label = SanitizeText(n.Ref), SanitizeText(n.Title), SanitizeText(n.Label)
		sanitizeTOCNodes(n.Children)
	}
}

func (t *TOC) sanitize() {
	t.Book, t.Lang, t.Title = SanitizeText(t.Book), SanitizeText(t.Lang), SanitizeText(t.Title)
	sanitizeTOCNodes(t.Children)
}

func (l *Listing) sanitize() {
	for i := range l.Items {
		e := &l.Items[i]
		e.Ref, e.Title = SanitizeText(e.Ref), SanitizeText(e.Title)
		e.IsoDate, e.Location = SanitizeText(e.IsoDate), SanitizeText(e.Location)
		for j := range e.Recipients {
			e.Recipients[j] = SanitizeText(e.Recipients[j])
		}
	}
}
