// Package orchestrator is the business layer that wires providers, the cache,
// and reporters together. It implements the three core operations exposed by
// the CLI: discover, query, and report.
package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"

	"github.com/harrisonwang/sift/internal/cache"
	"github.com/harrisonwang/sift/internal/config"
	"github.com/harrisonwang/sift/internal/model"
	"github.com/harrisonwang/sift/internal/provider"
	"golang.org/x/sync/errgroup"
)

// providerConcurrency bounds how many providers fetch simultaneously.
const providerConcurrency = 4

// Orchestrator coordinates discovery, querying, and reporting.
type Orchestrator struct {
	cfg   *config.Config
	cache cache.Cache
	log   *slog.Logger
}

// New builds an orchestrator.
func New(cfg *config.Config, c cache.Cache, log *slog.Logger) *Orchestrator {
	return &Orchestrator{cfg: cfg, cache: c, log: log}
}

// SourceResult summarizes one provider's discover run.
type SourceResult struct {
	Provider string
	Fetched  int
	New      int
	Err      error
}

// DiscoverStats aggregates a discover run across providers.
type DiscoverStats struct {
	Results      []SourceResult
	TotalFetched int
	TotalNew     int
}

// Discover fetches all enabled providers concurrently and stores new items.
// A single provider failure is recorded in its SourceResult but never aborts
// the others; only context cancellation stops the run.
func (o *Orchestrator) Discover(ctx context.Context) (DiscoverStats, error) {
	enabled := o.cfg.EnabledProviders()
	if len(enabled) == 0 {
		return DiscoverStats{}, fmt.Errorf("no enabled providers in config")
	}

	var (
		mu      sync.Mutex
		results []SourceResult
	)
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(providerConcurrency)

	for _, pc := range enabled {
		g.Go(func() error {
			res := SourceResult{Provider: pc.Name}

			p, err := provider.Build(pc, o.log)
			if err != nil {
				res.Err = err
				o.log.Warn("provider 构建失败", "provider", pc.Name, "err", err)
				mu.Lock()
				results = append(results, res)
				mu.Unlock()
				return nil
			}

			items, ferr := p.Fetch(gctx)
			res.Fetched = len(items)
			res.Err = ferr
			for _, it := range items {
				isNew, ierr := o.cache.InsertIfNew(it)
				if ierr != nil {
					o.log.Warn("写入缓存失败", "provider", pc.Name, "err", ierr)
					continue
				}
				if isNew {
					res.New++
				}
			}
			o.log.Info("provider 完成", "provider", pc.Name, "fetched", res.Fetched, "new", res.New)

			mu.Lock()
			results = append(results, res)
			mu.Unlock()
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return DiscoverStats{Results: results}, err
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Provider < results[j].Provider })
	stats := DiscoverStats{Results: results}
	for _, r := range results {
		stats.TotalFetched += r.Fetched
		stats.TotalNew += r.New
	}
	return stats, nil
}

// Query returns cached items matching the filter.
func (o *Orchestrator) Query(filter cache.Filter) ([]model.Item, error) {
	return o.cache.Query(filter)
}

// Prune deletes cached items matching the filter and returns the count removed.
// It's the way to clear stale rows after tightening a provider's filtering
// (e.g. flipping include_replies): prune the affected source, then re-discover.
func (o *Orchestrator) Prune(filter cache.Filter) (int64, error) {
	return o.cache.Delete(filter)
}
