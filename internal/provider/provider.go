// Package provider defines the data-source abstraction. Each source (Twitter,
// HackerNews, Reddit, RSS blogs, ...) implements Provider; the registry turns
// configuration into a set of activated providers.
package provider

import (
	"context"

	"github.com/harrisonwang/sift/internal/model"
)

// Provider is a single data source. Implementations are constructed from
// configuration by a Factory registered in the registry.
type Provider interface {
	// Name returns the provider type name, e.g. "twitter". It matches the
	// `name` field in the YAML config and the registry key.
	Name() string

	// Fetch retrieves the current batch of items from the source. It should
	// honor ctx cancellation/deadline and return a non-nil error only on a
	// hard failure; partial results with a soft warning are returned via the
	// items slice with err == nil where reasonable.
	Fetch(ctx context.Context) ([]model.Item, error)
}

// Info describes a provider type for the `source list` / `source info`
// commands. It is static metadata, independent of any configured instance.
type Info struct {
	Name       string   // provider type name
	Summary    string   // one-line description
	ConfigKeys []string // recognized keys under `config:`
	Example    string   // short YAML example snippet
}

// Describer is an optional interface a provider may implement to expose its
// Info. The registry can also supply Info for types that aren't instantiated.
type Describer interface {
	Info() Info
}
