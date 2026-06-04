// Package resolver turns a thing the user knows — a URL, a handle, a subreddit,
// a website — into a concrete sift source (provider + key), optionally verifying
// it can be fetched. It is the engine behind `sift add`.
//
// It is fully non-interactive: ambiguity (e.g. a site exposing several feeds)
// is reported via Candidates, never resolved by prompting.
package resolver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/harrisonwang/sift/internal/feedx"
)

// Provider type names a resolved source maps to.
const (
	ProviderTwitter    = "twitter"
	ProviderReddit     = "reddit"
	ProviderHackerNews = "hackernews"
	ProviderRSSBlog    = "rssblog"
)

// nitterInstances are tried (in order) when verifying a twitter handle.
var nitterInstances = []string{
	"https://nitter.net",
	"https://nitter.privacydev.net",
	"https://nitter.poast.org",
}

// Result is the outcome of resolving one input.
type Result struct {
	Input    string `json:"input"`
	Resolved bool   `json:"resolved"`

	Provider string `json:"provider,omitempty"`
	// Key is the provider-specific identifier: account, subreddit, or feed URL.
	Key     string `json:"key,omitempty"`
	FeedURL string `json:"feed_url,omitempty"` // rssblog
	Source  string `json:"source,omitempty"`   // rssblog label

	Verified    bool   `json:"verified"`
	SampleCount int    `json:"sample_count,omitempty"`
	LatestTitle string `json:"latest_title,omitempty"`

	Ambiguous  bool     `json:"ambiguous,omitempty"`
	Candidates []string `json:"candidates,omitempty"`

	Written   bool `json:"written"`
	Duplicate bool `json:"duplicate,omitempty"`

	Suggestion string `json:"suggestion,omitempty"`
	Error      string `json:"error,omitempty"`
}

// Resolver detects and verifies sources.
type Resolver struct {
	http *http.Client
	feed *feedx.Client
}

// New builds a Resolver with sane HTTP/feed clients.
func New() *Resolver {
	return &Resolver{
		http: &http.Client{Timeout: 15 * time.Second},
		feed: feedx.NewClient("", 0),
	}
}

// Resolve identifies the input. It may hit the network for RSS auto-discovery
// on generic websites, but does not verify (see Verify).
func (r *Resolver) Resolve(ctx context.Context, input string) Result {
	res := Result{Input: input}
	in := strings.TrimSpace(input)
	if in == "" {
		res.Error = "空输入"
		return res
	}

	// 1. @handle → twitter
	if h, ok := strings.CutPrefix(in, "@"); ok {
		return twitterResult(input, h)
	}
	// 2. bare shortcuts
	switch strings.ToLower(in) {
	case "hn", "hackernews":
		return hackernewsResult(input)
	}
	// 3. r/sub (bare)
	if s, ok := strings.CutPrefix(in, "r/"); ok && s != "" {
		return redditResult(input, s)
	}

	// Need a URL from here on; add scheme if missing.
	if !strings.Contains(in, "://") {
		if !strings.Contains(in, ".") {
			res.Error = fmt.Sprintf("无法识别 %q;请用 @handle、r/subreddit 或完整 URL", input)
			return res
		}
		in = "https://" + in
	}
	u, err := url.Parse(in)
	if err != nil || u.Host == "" {
		res.Error = fmt.Sprintf("无法解析 URL:%q", input)
		return res
	}
	host := strings.ToLower(strings.TrimPrefix(u.Host, "www."))
	segs := pathSegments(u.Path)

	switch {
	case host == "x.com" || host == "twitter.com" || host == "mobile.twitter.com" ||
		strings.HasPrefix(host, "nitter."):
		if len(segs) == 0 {
			res.Error = "缺少账号名,例:x.com/karpathy"
			return res
		}
		return twitterResult(input, segs[0])

	case host == "reddit.com" || host == "old.reddit.com" || strings.HasSuffix(host, ".reddit.com"):
		if len(segs) >= 2 && segs[0] == "r" {
			return redditResult(input, segs[1])
		}
		res.Error = "缺少子版块,例:reddit.com/r/golang"
		return res

	case host == "news.ycombinator.com":
		return hackernewsResult(input)

	case host == "github.com":
		if len(segs) >= 2 {
			owner, repo := segs[0], segs[1]
			feed := fmt.Sprintf("https://github.com/%s/%s/releases.atom", owner, repo)
			return rssResult(input, feed, owner+"_"+repo)
		}
		res.Error = "缺少 owner/repo,例:github.com/cli/cli"
		return res
	}

	// 4. feed-shaped URL → it is the feed itself
	if isFeedURL(u) {
		return rssResult(input, u.String(), slugFromHost(host))
	}

	// 5. generic website → RSS/Atom auto-discovery
	feeds, derr := r.discoverFeeds(ctx, u.String())
	if derr != nil {
		res.Error = fmt.Sprintf("抓取网站失败:%v", derr)
		return res
	}
	if len(feeds) == 0 {
		res.Error = "未发现 RSS/Atom feed;请直接提供 feed 地址"
		return res
	}
	out := rssResult(input, feeds[0], slugFromHost(host))
	if len(feeds) > 1 {
		out.Ambiguous = true
		out.Candidates = feeds
	}
	return out
}

// Verify fetches the resolved source once and records a sample count / latest
// title. On failure it sets Error and leaves Verified false.
func (r *Resolver) Verify(ctx context.Context, res *Result) {
	if !res.Resolved {
		return
	}
	switch res.Provider {
	case ProviderTwitter:
		r.verifyTwitter(ctx, res)
	case ProviderReddit:
		r.verifyFeed(ctx, res, fmt.Sprintf("https://www.reddit.com/r/%s/.rss", res.Key))
	case ProviderRSSBlog:
		r.verifyFeed(ctx, res, res.FeedURL)
	case ProviderHackerNews:
		r.verifyHackerNews(ctx, res)
	}
}

func (r *Resolver) verifyFeed(ctx context.Context, res *Result, feedURL string) {
	feed, err := r.feed.Parse(ctx, feedURL)
	if err != nil {
		res.Error = fmt.Sprintf("验证失败:%v", err)
		return
	}
	res.Verified = true
	res.SampleCount = len(feed.Items)
	if len(feed.Items) > 0 {
		res.LatestTitle = strings.TrimSpace(feed.Items[0].Title)
	}
}

func (r *Resolver) verifyTwitter(ctx context.Context, res *Result) {
	for _, inst := range nitterInstances {
		feedURL := strings.TrimRight(inst, "/") + "/" + res.Key + "/rss"
		feed, err := r.feed.Parse(ctx, feedURL)
		if err != nil {
			continue
		}
		res.Verified = true
		res.SampleCount = len(feed.Items)
		if len(feed.Items) > 0 {
			res.LatestTitle = strings.TrimSpace(feed.Items[0].Title)
		}
		return
	}
	// Nitter is flaky; a failure here is more likely an instance problem than a
	// bad handle, so we say so rather than declaring the account invalid.
	res.Error = "验证失败:所有 Nitter 实例均未响应(可能是实例问题,而非账号无效)"
}

func (r *Resolver) verifyHackerNews(ctx context.Context, res *Result) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://hacker-news.firebaseio.com/v0/topstories.json", nil)
	if err != nil {
		res.Error = err.Error()
		return
	}
	resp, err := r.http.Do(req)
	if err != nil {
		res.Error = fmt.Sprintf("验证失败:%v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		res.Error = fmt.Sprintf("验证失败:HN API 状态 %d", resp.StatusCode)
		return
	}
	var ids []int
	if err := json.NewDecoder(resp.Body).Decode(&ids); err != nil {
		res.Error = fmt.Sprintf("验证失败:%v", err)
		return
	}
	res.Verified = true
	res.SampleCount = len(ids)
}

// discoverFeeds fetches a page and extracts <link rel="alternate"> feed URLs.
func (r *Resolver) discoverFeeds(ctx context.Context, pageURL string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", feedx.BrowserUA)
	resp, err := r.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("状态 %d", resp.StatusCode)
	}
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}
	base, _ := url.Parse(pageURL)

	var feeds []string
	seen := map[string]bool{}
	doc.Find(`link[rel="alternate"]`).Each(func(_ int, s *goquery.Selection) {
		typ, _ := s.Attr("type")
		typ = strings.ToLower(strings.TrimSpace(typ))
		if typ != "application/rss+xml" && typ != "application/atom+xml" {
			return
		}
		href, ok := s.Attr("href")
		if !ok || strings.TrimSpace(href) == "" {
			return
		}
		abs := href
		if ref, err := url.Parse(href); err == nil && base != nil {
			abs = base.ResolveReference(ref).String()
		}
		if !seen[abs] {
			seen[abs] = true
			feeds = append(feeds, abs)
		}
	})
	return feeds, nil
}

// --- constructors for each provider type ---

func twitterResult(input, account string) Result {
	account = strings.TrimSpace(strings.TrimPrefix(account, "@"))
	return Result{Input: input, Resolved: account != "", Provider: ProviderTwitter, Key: account,
		Error: emptyKeyErr(account, "账号名")}
}

func redditResult(input, sub string) Result {
	sub = strings.TrimSpace(strings.Trim(sub, "/"))
	return Result{Input: input, Resolved: sub != "", Provider: ProviderReddit, Key: sub,
		Error: emptyKeyErr(sub, "子版块")}
}

func hackernewsResult(input string) Result {
	return Result{Input: input, Resolved: true, Provider: ProviderHackerNews}
}

func rssResult(input, feedURL, source string) Result {
	return Result{Input: input, Resolved: feedURL != "", Provider: ProviderRSSBlog,
		Key: feedURL, FeedURL: feedURL, Source: source}
}

func emptyKeyErr(v, what string) string {
	if v == "" {
		return "缺少" + what
	}
	return ""
}

// --- small helpers ---

func pathSegments(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

func isFeedURL(u *url.URL) bool {
	p := strings.ToLower(u.Path)
	if strings.HasSuffix(p, ".rss") || strings.HasSuffix(p, ".atom") || strings.HasSuffix(p, ".xml") {
		return true
	}
	return strings.Contains(p, "/feed") || strings.Contains(p, "/rss")
}

// slugFromHost turns a host into a config-friendly source label, e.g.
// "simonwillison.net" -> "simonwillison", "news.ycombinator.com" -> "news_ycombinator".
func slugFromHost(host string) string {
	host = strings.TrimPrefix(strings.ToLower(host), "www.")
	host = strings.TrimSuffix(host, ".")
	// drop the last label (TLD) when there are at least two labels
	labels := strings.Split(host, ".")
	if len(labels) >= 2 {
		labels = labels[:len(labels)-1]
	}
	slug := strings.Join(labels, "_")
	slug = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			return r
		case r >= 'A' && r <= 'Z':
			return r + 32
		default:
			return '_'
		}
	}, slug)
	return strings.Trim(slug, "_")
}
