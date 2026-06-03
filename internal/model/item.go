// Package model defines the standardized data model shared across all
// providers and the reporting layer. Every source normalizes its raw output
// into an Item so that reporting and storage stay decoupled from the source.
package model

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

// Item is the unified entry emitted by every provider.
//
// Identity (used for de-duplication) is the pair (Provider, ExternalID).
// Provider is the source *type* (e.g. "twitter", "hackernews"); Source is the
// concrete origin within that type (e.g. "twitter:msdev", "openai_blog") and
// is what users filter and group reports by.
type Item struct {
	// Provider is the provider type that produced this item, e.g. "twitter".
	Provider string `json:"provider"`
	// Source is the concrete origin, e.g. "twitter:msdev" or "openai_blog".
	Source string `json:"source"`
	// ExternalID uniquely identifies the item within its provider (tweet id,
	// HN item id, feed entry guid, ...).
	ExternalID string `json:"external_id"`

	Title   string `json:"title"`
	URL     string `json:"url"`
	Author  string `json:"author,omitempty"`
	Summary string `json:"summary,omitempty"`

	// PublishedAt is the item's own timestamp. Zero if the source omits it.
	PublishedAt time.Time `json:"published_at,omitzero"`
	// FetchedAt is when sift first stored the item.
	FetchedAt time.Time `json:"fetched_at,omitzero"`

	// Score is an optional popularity signal (HN points, Reddit upvotes).
	Score int `json:"score,omitempty"`

	Tags []string `json:"tags,omitempty"`

	// Extra carries provider-specific metadata that doesn't fit the common
	// fields. Kept as strings so it serializes cleanly to JSON and SQLite.
	Extra map[string]string `json:"extra,omitempty"`
}

// ID returns a stable identifier for the item, derived from
// (Provider, ExternalID). It is used as the cache primary key.
func (it Item) ID() string {
	key := it.Provider + "\x00" + it.ExternalID
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:16])
}

// TitleOrSummary returns a non-empty display string, falling back to the
// summary (truncated) when the title is blank — mirrors the Python report's
// "(无标题)" handling but prefers real content when available.
func (it Item) TitleOrSummary() string {
	t := strings.TrimSpace(it.Title)
	if t != "" {
		return t
	}
	s := strings.TrimSpace(it.Summary)
	if s == "" {
		return "(untitled)"
	}
	if len(s) > 120 {
		return s[:120] + "…"
	}
	return s
}
