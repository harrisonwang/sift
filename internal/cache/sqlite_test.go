package cache

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/harrisonwang/sift/internal/model"
)

func newTestCache(t *testing.T) *SQLite {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	c, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func sampleItem() model.Item {
	return model.Item{
		Provider:    "hackernews",
		Source:      "hackernews",
		ExternalID:  "123",
		Title:       "Go 1.26 released",
		URL:         "https://example.com/go126",
		Author:      "gopher",
		Summary:     "A new Go version about generics.",
		PublishedAt: time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC),
		Score:       42,
		Tags:        []string{"top"},
		Extra:       map[string]string{"replies": "10"},
	}
}

func TestInsertIfNew_DeduplicatesByProviderAndExternalID(t *testing.T) {
	c := newTestCache(t)
	it := sampleItem()

	isNew, err := c.InsertIfNew(it)
	if err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if !isNew {
		t.Fatal("first insert should report new=true")
	}

	// Same identity, different fetched time -> not new.
	isNew, err = c.InsertIfNew(it)
	if err != nil {
		t.Fatalf("second insert: %v", err)
	}
	if isNew {
		t.Fatal("re-inserting same item should report new=false")
	}

	// Different external_id -> new again.
	other := it
	other.ExternalID = "456"
	isNew, _ = c.InsertIfNew(other)
	if !isNew {
		t.Fatal("different external_id should be new")
	}
}

func TestQuery_RoundTripsFieldsAndFilters(t *testing.T) {
	c := newTestCache(t)
	a := sampleItem()
	b := sampleItem()
	b.ExternalID = "456"
	b.Provider = "reddit"
	b.Source = "reddit:programming"
	b.Title = "Rust vs Go"
	if _, err := c.InsertIfNew(a); err != nil {
		t.Fatal(err)
	}
	if _, err := c.InsertIfNew(b); err != nil {
		t.Fatal(err)
	}

	// Filter by provider.
	got, err := c.Query(Filter{Provider: "hackernews"})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(got) != 1 || got[0].ExternalID != "123" {
		t.Fatalf("provider filter: got %d items %+v", len(got), got)
	}

	// Round-trip of structured fields.
	r := got[0]
	if r.Title != a.Title || r.Score != 42 || len(r.Tags) != 1 || r.Tags[0] != "top" {
		t.Fatalf("field round-trip mismatch: %+v", r)
	}
	if r.Extra["replies"] != "10" {
		t.Fatalf("extra round-trip mismatch: %+v", r.Extra)
	}
	if !r.PublishedAt.Equal(a.PublishedAt) {
		t.Fatalf("published_at mismatch: %v != %v", r.PublishedAt, a.PublishedAt)
	}

	// Keyword filter is case-insensitive over title/summary.
	got, _ = c.Query(Filter{Keyword: "rust"})
	if len(got) != 1 || got[0].Provider != "reddit" {
		t.Fatalf("keyword filter: got %+v", got)
	}

	// Date filter uses the local fetched day.
	today := time.Now().Local().Format("2006-01-02")
	got, _ = c.Query(Filter{Date: today})
	if len(got) != 2 {
		t.Fatalf("date filter: expected 2 today, got %d", len(got))
	}
	got, _ = c.Query(Filter{Date: "1999-01-01"})
	if len(got) != 0 {
		t.Fatalf("date filter: expected 0 for old date, got %d", len(got))
	}
}

func TestDelete_ByFilterAndAll(t *testing.T) {
	c := newTestCache(t)
	a := sampleItem()
	b := sampleItem()
	b.ExternalID = "456"
	b.Provider = "reddit"
	b.Source = "reddit:programming"
	for _, it := range []model.Item{a, b} {
		if _, err := c.InsertIfNew(it); err != nil {
			t.Fatal(err)
		}
	}

	// Delete by source removes only the match.
	n, err := c.Delete(Filter{Source: "reddit:programming"})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 deleted, got %d", n)
	}
	got, _ := c.Query(Filter{})
	if len(got) != 1 || got[0].Provider != "hackernews" {
		t.Fatalf("after delete: %+v", got)
	}

	// Empty filter deletes everything (caller is responsible for gating).
	n, _ = c.Delete(Filter{})
	if n != 1 {
		t.Fatalf("expected 1 remaining deleted, got %d", n)
	}
	got, _ = c.Query(Filter{})
	if len(got) != 0 {
		t.Fatalf("expected empty cache, got %d", len(got))
	}
}

func TestQuery_LimitAndSources(t *testing.T) {
	c := newTestCache(t)
	for i := range 5 {
		it := sampleItem()
		it.ExternalID = string(rune('a' + i))
		if _, err := c.InsertIfNew(it); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := c.Query(Filter{Limit: 3})
	if len(got) != 3 {
		t.Fatalf("limit: expected 3, got %d", len(got))
	}
	srcs, err := c.Sources()
	if err != nil {
		t.Fatal(err)
	}
	if len(srcs) != 1 || srcs[0] != "hackernews" {
		t.Fatalf("sources: %+v", srcs)
	}
}
