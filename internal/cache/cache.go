// Package cache persists discovered items and provides de-duplication.
package cache

import "github.com/harrisonwang/sift/internal/model"

// Filter selects items for querying. Zero-valued fields are ignored.
type Filter struct {
	// Date filters by the local calendar day an item was first stored
	// (FetchedAt), formatted "2006-01-02". Empty means no date filter.
	Date string
	// Provider filters by provider type (e.g. "twitter").
	Provider string
	// Source filters by concrete source (e.g. "twitter:msdev").
	Source string
	// Keyword does a case-insensitive substring match over title + summary.
	Keyword string
	// Limit caps the number of returned rows; <= 0 means a sensible default.
	Limit int
}

// Cache is the persistence layer. Implementations must be safe for concurrent
// use by multiple goroutines (the orchestrator fetches sources in parallel).
type Cache interface {
	// InsertIfNew stores item only if its ID is not already present. It
	// returns true when a new row was written, false when it already existed.
	// FetchedAt is set by the cache when the row is created.
	InsertIfNew(item model.Item) (bool, error)

	// Query returns items matching the filter, newest first.
	Query(filter Filter) ([]model.Item, error)

	// Delete removes items matching the filter and returns the count deleted.
	// A zero-valued filter matches everything, so callers must gate that case.
	Delete(filter Filter) (int64, error)

	// Sources returns the distinct sources stored, for diagnostics.
	Sources() ([]string, error)

	// Close releases the underlying handle.
	Close() error
}
