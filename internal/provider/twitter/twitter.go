// Package twitter implements a Twitter/X provider backed by Nitter RSS feeds,
// with multi-instance fallback. It is the Go port of the Python NitterFetcher:
// for each watched account it fetches /{account}/rss from the first healthy
// Nitter instance and normalizes entries into model.Item.
package twitter

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/harrisonwang/sift/internal/config"
	"github.com/harrisonwang/sift/internal/feedx"
	"github.com/harrisonwang/sift/internal/model"
	"github.com/harrisonwang/sift/internal/provider"
	"github.com/mmcdole/gofeed"
)

const name = "twitter"

// defaultInstances mirrors the Python config's NITTER_INSTANCES list.
var defaultInstances = []string{
	"https://nitter.net",
	"https://nitter.privacydev.net",
	"https://nitter.poast.org",
	"https://nitter.space",
}

// maxInstanceFailures is the failure budget before an instance is skipped for
// the remainder of the run (mirrors Python's MAX_RETRIES*2 heuristic).
const maxInstanceFailures = 6

func init() { provider.Register(Info(), New) }

// Settings is the twitter provider config block.
//
// A Nitter account feed is the account's full timeline: original posts plus
// retweets ("RT by @acct:") and replies ("R to @x:"). IncludeRetweets and
// IncludeReplies control which of those are kept. They are *bool so an omitted
// key takes the default (retweets off, replies on) rather than Go's zero false.
type Settings struct {
	Accounts        []string `yaml:"accounts"`
	NitterInstances []string `yaml:"nitter_instances"`
	MaxItems        int      `yaml:"max_items"`
	IncludeRetweets *bool    `yaml:"include_retweets"`
	IncludeReplies  *bool    `yaml:"include_replies"`
}

// Provider fetches tweets for watched accounts via Nitter RSS.
type Provider struct {
	accounts        []string
	instances       []string
	maxItems        int
	includeRetweets bool
	includeReplies  bool
	client          *feedx.Client
	log             *slog.Logger

	failures map[string]int // per-instance failure counter
}

// New constructs a twitter provider from its config block.
func New(pc config.ProviderConfig, log *slog.Logger) (provider.Provider, error) {
	var s Settings
	if err := pc.Decode(&s); err != nil {
		return nil, err
	}
	if len(s.Accounts) == 0 {
		return nil, fmt.Errorf("twitter：未配置 accounts")
	}
	instances := s.NitterInstances
	if len(instances) == 0 {
		instances = defaultInstances
	}

	// Defaults: drop retweets (other accounts' content surfacing in the feed),
	// keep the account's own replies. Both are per-account configurable.
	includeRetweets := false
	if s.IncludeRetweets != nil {
		includeRetweets = *s.IncludeRetweets
	}
	includeReplies := true
	if s.IncludeReplies != nil {
		includeReplies = *s.IncludeReplies
	}

	return &Provider{
		accounts:        s.Accounts,
		instances:       instances,
		maxItems:        s.MaxItems,
		includeRetweets: includeRetweets,
		includeReplies:  includeReplies,
		client:          feedx.NewClient("", 0),
		log:             log.With("provider", name),
		failures:        map[string]int{},
	}, nil
}

func (p *Provider) Name() string { return name }

func (p *Provider) Fetch(ctx context.Context) ([]model.Item, error) {
	var items []model.Item
	for _, account := range p.accounts {
		select {
		case <-ctx.Done():
			return items, ctx.Err()
		default:
		}

		account = strings.TrimPrefix(strings.TrimSpace(account), "@")
		got := p.fetchAccount(ctx, account)
		p.log.Info("已抓取账号", "account", account, "items", len(got))
		items = append(items, got...)
		time.Sleep(time.Second) // pace requests, like the Python REQUEST_DELAY
	}
	return items, nil
}

// fetchAccount tries each instance in order until one returns a parseable feed.
func (p *Provider) fetchAccount(ctx context.Context, account string) []model.Item {
	path := "/" + account + "/rss"
	for _, inst := range p.instances {
		if p.failures[inst] >= maxInstanceFailures {
			continue
		}
		feedURL := strings.TrimRight(inst, "/") + path
		feed, err := p.client.Parse(ctx, feedURL)
		if err != nil {
			p.failures[inst]++
			p.log.Warn("实例失败", "instance", inst, "account", account, "err", err)
			continue
		}
		// success: decay this instance's failure count
		if p.failures[inst] > 0 {
			p.failures[inst]--
		}

		var out []model.Item
		for _, fi := range feed.Items {
			if p.maxItems > 0 && len(out) >= p.maxItems {
				break
			}
			it, ok := toTweet(account, fi)
			if !ok {
				continue
			}
			switch classify(account, it.Title, it.Author) {
			case kindRetweet:
				if !p.includeRetweets {
					continue
				}
			case kindReply:
				if !p.includeReplies {
					continue
				}
			}
			out = append(out, it)
		}
		return out
	}
	p.log.Warn("所有实例均失败", "account", account)
	return nil
}

// tweetKind classifies a Nitter timeline entry.
type tweetKind int

const (
	kindOriginal tweetKind = iota
	kindRetweet
	kindReply
)

// classify determines whether an entry is an original post, a retweet, or a
// reply. Nitter prefixes retweet titles with "RT by @" and reply titles with
// "R to @"; a resolved author different from the feed's account is also a
// reliable retweet signal (the creator is the original poster).
func classify(account, title, author string) tweetKind {
	switch {
	case strings.HasPrefix(title, "RT by @"):
		return kindRetweet
	case author != "" && !strings.EqualFold(author, account):
		return kindRetweet
	case strings.HasPrefix(title, "R to @"):
		return kindReply
	default:
		return kindOriginal
	}
}

// toTweet maps a Nitter RSS entry to an Item. It extracts the numeric status id
// and real author from the entry link (handling the "/user/status/123#m" form
// that the Python version's isdigit() check would have dropped).
func toTweet(account string, fi *gofeed.Item) (model.Item, bool) {
	author, tweetID := parseStatusLink(fi.Link)
	if author == "" {
		author = account
	}
	if fi.Author != nil && fi.Author.Name != "" {
		author = strings.TrimPrefix(strings.TrimSpace(fi.Author.Name), "@")
	}
	if tweetID == "" {
		// Some instances put a bare numeric id in <guid>.
		tweetID = digitsOnly(fi.GUID)
	}
	if tweetID == "" {
		return model.Item{}, false
	}

	it := feedx.ToItem(name, name+":"+account, fi)
	it.ExternalID = tweetID
	it.Author = author
	it.URL = fmt.Sprintf("https://x.com/%s/status/%s", author, tweetID)
	return it, true
}

// parseStatusLink extracts (author, tweetID) from a Nitter status URL.
func parseStatusLink(link string) (author, tweetID string) {
	if link == "" {
		return "", ""
	}
	if i := strings.IndexByte(link, '#'); i >= 0 {
		link = link[:i]
	}
	u, err := url.Parse(link)
	if err != nil {
		return "", ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i, seg := range parts {
		if seg == "status" && i+1 < len(parts) {
			tweetID = digitsOnly(parts[i+1])
			if i-1 >= 0 {
				author = parts[i-1]
			}
			break
		}
	}
	return author, tweetID
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Info implements provider.Describer.
func (p *Provider) Info() provider.Info { return Info() }

// Info returns static metadata for the twitter provider type.
func Info() provider.Info {
	return provider.Info{
		Name:       name,
		Summary:    "Twitter/X 账号（经 Nitter RSS），支持多实例 fallback。",
		ConfigKeys: []string{"accounts", "nitter_instances", "max_items", "include_retweets", "include_replies"},
		Example: `  - name: twitter
    enabled: true
    config:
      accounts: [msdev, openai]
      nitter_instances:
        - https://nitter.net
        - https://nitter.privacydev.net
      include_retweets: false   # 默认：丢弃 "RT by @" 转推
      include_replies: true     # 默认：保留账号自己的回复`,
	}
}
