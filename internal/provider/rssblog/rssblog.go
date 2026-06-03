// Package rssblog implements a generic RSS/Atom provider. It fetches a list of
// configured feeds and normalizes each entry into a model.Item. Twitter and
// Reddit are specialized variants of the same idea; this is the plain one.
package rssblog

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/harrisonwang/sift/internal/config"
	"github.com/harrisonwang/sift/internal/feedx"
	"github.com/harrisonwang/sift/internal/model"
	"github.com/harrisonwang/sift/internal/provider"
)

const name = "rssblog"

func init() { provider.Register(Info(), New) }

// Feed is a single configured feed.
type Feed struct {
	URL    string `yaml:"url"`
	Source string `yaml:"source"`
}

// Settings is the rssblog provider config block.
type Settings struct {
	Feeds    []Feed `yaml:"feeds"`
	MaxItems int    `yaml:"max_items"`
}

// Provider fetches a set of RSS/Atom feeds.
type Provider struct {
	feeds    []Feed
	maxItems int
	client   *feedx.Client
	log      *slog.Logger
}

// New constructs an rssblog provider from its config block.
func New(pc config.ProviderConfig, log *slog.Logger) (provider.Provider, error) {
	var s Settings
	if err := pc.Decode(&s); err != nil {
		return nil, err
	}
	if len(s.Feeds) == 0 {
		return nil, fmt.Errorf("rssblog：未配置 feeds")
	}
	for i, f := range s.Feeds {
		if f.URL == "" {
			return nil, fmt.Errorf("rssblog：feeds[%d] 缺少 url", i)
		}
	}
	return &Provider{
		feeds:    s.Feeds,
		maxItems: s.MaxItems,
		client:   feedx.NewClient("", 0),
		log:      log.With("provider", name),
	}, nil
}

func (p *Provider) Name() string { return name }

func (p *Provider) Fetch(ctx context.Context) ([]model.Item, error) {
	var items []model.Item
	for _, f := range p.feeds {
		select {
		case <-ctx.Done():
			return items, ctx.Err()
		default:
		}

		src := f.Source
		if src == "" {
			src = name
		}
		feed, err := p.client.Parse(ctx, f.URL)
		if err != nil {
			p.log.Warn("抓取 feed 失败", "source", src, "url", f.URL, "err", err)
			continue
		}
		count := 0
		for _, fi := range feed.Items {
			if p.maxItems > 0 && count >= p.maxItems {
				break
			}
			it := feedx.ToItem(name, src, fi)
			if it.ExternalID == "" {
				continue
			}
			items = append(items, it)
			count++
		}
		p.log.Info("已抓取 feed", "source", src, "items", count)
		time.Sleep(300 * time.Millisecond) // be polite between feeds
	}
	return items, nil
}

// Info implements provider.Describer.
func (p *Provider) Info() provider.Info { return Info() }

// Info returns static metadata for the rssblog provider type.
func Info() provider.Info {
	return provider.Info{
		Name:       name,
		Summary:    "通用 RSS/Atom 源（博客、release notes、论文 feed 等）。",
		ConfigKeys: []string{"feeds[].url", "feeds[].source", "max_items"},
		Example: `  - name: rssblog
    enabled: true
    config:
      feeds:
        - url: https://openai.com/blog/rss.xml
          source: openai_blog
      max_items: 50`,
	}
}
