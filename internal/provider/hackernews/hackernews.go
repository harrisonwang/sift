// Package hackernews implements a Hacker News provider backed by the official
// Firebase API. For each configured feed (top/new/show/ask/best) it fetches the
// story id list and then the item details, concurrently and bounded.
package hackernews

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/harrisonwang/sift/internal/config"
	"github.com/harrisonwang/sift/internal/model"
	"github.com/harrisonwang/sift/internal/provider"
	"golang.org/x/sync/errgroup"
)

const (
	name    = "hackernews"
	apiBase = "https://hacker-news.firebaseio.com/v0"
)

// validFeeds maps a feed name to its Firebase endpoint.
var validFeeds = map[string]string{
	"top":  "topstories",
	"new":  "newstories",
	"show": "showstories",
	"ask":  "askstories",
	"best": "beststories",
}

// fetchConcurrency bounds simultaneous item lookups.
const fetchConcurrency = 8

func init() { provider.Register(Info(), New) }

// Settings is the hackernews provider config block.
type Settings struct {
	Feeds    []string `yaml:"feeds"`
	MaxItems int      `yaml:"max_items"`
}

// Provider fetches Hacker News stories.
type Provider struct {
	feeds    []string
	maxItems int
	client   *http.Client
	log      *slog.Logger
}

// hnItem is the subset of the HN item schema we consume.
type hnItem struct {
	ID          int    `json:"id"`
	Type        string `json:"type"`
	By          string `json:"by"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Text        string `json:"text"`
	Score       int    `json:"score"`
	Time        int64  `json:"time"`
	Descendants int    `json:"descendants"`
}

// New constructs a hackernews provider from its config block.
func New(pc config.ProviderConfig, log *slog.Logger) (provider.Provider, error) {
	var s Settings
	if err := pc.Decode(&s); err != nil {
		return nil, err
	}
	if len(s.Feeds) == 0 {
		s.Feeds = []string{"top"}
	}
	for _, f := range s.Feeds {
		if _, ok := validFeeds[f]; !ok {
			return nil, fmt.Errorf("hackernews：未知 feed %q（可选 top|new|show|ask|best）", f)
		}
	}
	if s.MaxItems <= 0 {
		s.MaxItems = 30
	}
	return &Provider{
		feeds:    s.Feeds,
		maxItems: s.MaxItems,
		client:   &http.Client{Timeout: 15 * time.Second},
		log:      log.With("provider", name),
	}, nil
}

func (p *Provider) Name() string { return name }

func (p *Provider) Fetch(ctx context.Context) ([]model.Item, error) {
	var (
		mu    sync.Mutex
		items []model.Item
	)
	for _, feed := range p.feeds {
		ids, err := p.storyIDs(ctx, feed)
		if err != nil {
			p.log.Warn("拉取 feed 列表失败", "feed", feed, "err", err)
			continue
		}
		if len(ids) > p.maxItems {
			ids = ids[:p.maxItems]
		}

		g, gctx := errgroup.WithContext(ctx)
		g.SetLimit(fetchConcurrency)
		for _, id := range ids {
			g.Go(func() error {
				hi, err := p.story(gctx, id)
				if err != nil {
					p.log.Warn("抓取条目失败", "id", id, "err", err)
					return nil // soft-fail individual items
				}
				if hi == nil || hi.Title == "" {
					return nil
				}
				it := toItem(feed, hi)
				mu.Lock()
				items = append(items, it)
				mu.Unlock()
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			return items, err // hard failure (context cancelled)
		}
		p.log.Info("已抓取 feed", "feed", feed, "ids", len(ids))
	}
	return items, nil
}

func (p *Provider) storyIDs(ctx context.Context, feed string) ([]int, error) {
	var ids []int
	if err := p.getJSON(ctx, apiBase+"/"+validFeeds[feed]+".json", &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

func (p *Provider) story(ctx context.Context, id int) (*hnItem, error) {
	var hi hnItem
	if err := p.getJSON(ctx, fmt.Sprintf("%s/item/%d.json", apiBase, id), &hi); err != nil {
		return nil, err
	}
	return &hi, nil
}

func (p *Provider) getJSON(ctx context.Context, url string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: status %d", url, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

func toItem(feed string, hi *hnItem) model.Item {
	url := hi.URL
	if url == "" {
		url = fmt.Sprintf("https://news.ycombinator.com/item?id=%d", hi.ID)
	}
	it := model.Item{
		Provider:   name,
		Source:     name,
		ExternalID: strconv.Itoa(hi.ID),
		Title:      hi.Title,
		URL:        url,
		Author:     hi.By,
		Score:      hi.Score,
		Tags:       []string{feed},
		Extra: map[string]string{
			"comments": fmt.Sprintf("https://news.ycombinator.com/item?id=%d", hi.ID),
			"replies":  strconv.Itoa(hi.Descendants),
		},
	}
	if hi.Time > 0 {
		it.PublishedAt = time.Unix(hi.Time, 0).UTC()
	}
	return it
}

// Info implements provider.Describer.
func (p *Provider) Info() provider.Info { return Info() }

// Info returns static metadata for the hackernews provider type.
func Info() provider.Info {
	return provider.Info{
		Name:       name,
		Summary:    "Hacker News 故事（经官方 Firebase API，支持 top/new/show/ask/best）。",
		ConfigKeys: []string{"feeds", "max_items"},
		Example: `  - name: hackernews
    enabled: true
    config:
      feeds: [top, show]
      max_items: 50`,
	}
}
