package provider

import (
	"fmt"
	"log/slog"
	"sort"

	"github.com/harrisonwang/sift/internal/config"
)

// Factory constructs a configured provider instance from its config block.
type Factory func(pc config.ProviderConfig, log *slog.Logger) (Provider, error)

type registration struct {
	factory Factory
	info    Info
}

// registry holds all known provider *types*, keyed by name. Providers register
// themselves from their package init() (see the provider/all aggregator), the
// same self-registration pattern as database/sql drivers.
var registry = map[string]registration{}

// Register adds a provider type. It panics on a duplicate name, which can only
// happen from a programming error at init time.
func Register(info Info, f Factory) {
	if _, dup := registry[info.Name]; dup {
		panic("provider: duplicate registration for " + info.Name)
	}
	registry[info.Name] = registration{factory: f, info: info}
}

// Build instantiates the provider for the given config block.
func Build(pc config.ProviderConfig, log *slog.Logger) (Provider, error) {
	reg, ok := registry[pc.Name]
	if !ok {
		return nil, fmt.Errorf("unknown provider %q (known: %v)", pc.Name, Names())
	}
	return reg.factory(pc, log)
}

// Names returns all registered provider type names, sorted.
func Names() []string {
	out := make([]string, 0, len(registry))
	for n := range registry {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Infos returns metadata for all registered provider types, sorted by name.
func Infos() []Info {
	out := make([]Info, 0, len(registry))
	for _, n := range Names() {
		out = append(out, registry[n].info)
	}
	return out
}

// InfoFor returns the metadata for a single provider type.
func InfoFor(name string) (Info, bool) {
	reg, ok := registry[name]
	return reg.info, ok
}
