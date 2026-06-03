// Package reddit implements a Reddit provider backed by per-subreddit RSS/Atom
// feeds. It reuses the shared feedx toolkit; Reddit's feeds are Atom, which
// gofeed handles transparently.
package reddit

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/harrisonwang/sift/internal/config"
	"github.com/harrisonwang/sift/internal/feedx"
	"github.com/harrisonwang/sift/internal/model"
	"github.com/harrisonwang/sift/internal/provider"
)

const name = "reddit"

// validSorts are the sort modes Reddit exposes as RSS.
var validSorts = map[string]bool{"hot": true, "new": true, "top": true, "rising": true, "best": true}

func init() { provider.Register(Info(), New) }

// Settings is the reddit provider config block.
type Settings struct {
	Subreddits []string `yaml:"subreddits"`
	Sort       string   `yaml:"sort"`
	Limit      int      `yaml:"limit"`
}

// Provider fetches posts for configured subreddits.
type Provider struct {
	subreddits []string
	sort       string
	limit      int
	client     *feedx.Client
	log        *slog.Logger
}

// New constructs a reddit provider from its config block.
func New(pc config.ProviderConfig, log *slog.Logger) (provider.Provider, error) {
	var s Settings
	if err := pc.Decode(&s); err != nil {
		return nil, err
	}
	if len(s.Subreddits) == 0 {
		return nil, fmt.Errorf("reddit：未配置 subreddits")
	}
	if s.Sort == "" {
		s.Sort = "hot"
	}
	if !validSorts[s.Sort] {
		return nil, fmt.Errorf("reddit：未知 sort %q（可选 hot|new|top|rising|best）", s.Sort)
	}
	return &Provider{
		subreddits: s.Subreddits,
		sort:       s.Sort,
		limit:      s.Limit,
		client:     feedx.NewClient("", 0),
		log:        log.With("provider", name),
	}, nil
}

func (p *Provider) Name() string { return name }

func (p *Provider) Fetch(ctx context.Context) ([]model.Item, error) {
	var items []model.Item
	for _, sub := range p.subreddits {
		select {
		case <-ctx.Done():
			return items, ctx.Err()
		default:
		}

		sub = strings.TrimPrefix(strings.TrimSpace(sub), "r/")
		feed, err := p.client.Parse(ctx, p.feedURL(sub))
		if err != nil {
			p.log.Warn("抓取子版块失败", "subreddit", sub, "err", err)
			continue
		}
		count := 0
		for _, fi := range feed.Items {
			if p.limit > 0 && count >= p.limit {
				break
			}
			it := feedx.ToItem(name, name+":"+sub, fi)
			if it.ExternalID == "" {
				continue
			}
			it.Author = strings.TrimPrefix(it.Author, "/u/")
			items = append(items, it)
			count++
		}
		p.log.Info("已抓取子版块", "subreddit", sub, "items", count)
		time.Sleep(time.Second) // Reddit rate-limits aggressively
	}
	return items, nil
}

// feedURL builds the RSS URL for a subreddit; "hot" uses the bare feed.
func (p *Provider) feedURL(sub string) string {
	if p.sort == "hot" {
		return fmt.Sprintf("https://www.reddit.com/r/%s/.rss", sub)
	}
	return fmt.Sprintf("https://www.reddit.com/r/%s/%s/.rss", sub, p.sort)
}

// Info implements provider.Describer.
func (p *Provider) Info() provider.Info { return Info() }

// Info returns static metadata for the reddit provider type.
func Info() provider.Info {
	return provider.Info{
		Name:       name,
		Summary:    "Reddit 子版块（经 RSS，支持 hot/new/top/rising/best）。",
		ConfigKeys: []string{"subreddits", "sort", "limit"},
		Example: `  - name: reddit
    enabled: true
    config:
      subreddits: [programming, MachineLearning]
      sort: hot
      limit: 30`,
	}
}
