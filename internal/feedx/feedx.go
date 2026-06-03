// Package feedx is a thin shared toolkit over gofeed for the RSS-based
// providers (rssblog, twitter/nitter, reddit). It centralizes the HTTP client
// configuration and the gofeed-item -> model.Item mapping so each provider only
// has to deal with its source-specific details.
package feedx

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/harrisonwang/sift/internal/model"
	"github.com/mmcdole/gofeed"
)

// BrowserUA mimics a real browser. Several feed hosts (notably Nitter
// instances) reject requests without a browser-like User-Agent.
const BrowserUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) " +
	"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// DefaultTimeout is the per-request timeout for feed fetches.
const DefaultTimeout = 15 * time.Second

// Client wraps a configured gofeed parser.
type Client struct {
	parser *gofeed.Parser
}

// NewClient builds a feed client. A zero timeout uses DefaultTimeout, and an
// empty userAgent uses BrowserUA.
func NewClient(userAgent string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	if userAgent == "" {
		userAgent = BrowserUA
	}
	p := gofeed.NewParser()
	p.UserAgent = userAgent
	p.Client = &http.Client{Timeout: timeout}
	return &Client{parser: p}
}

// Parse fetches and parses the feed at url, honoring ctx.
func (c *Client) Parse(ctx context.Context, url string) (*gofeed.Feed, error) {
	return c.parser.ParseURLWithContext(url, ctx)
}

// ToItem maps a gofeed item into the standardized model.Item for the given
// provider type and concrete source. ExternalID falls back from GUID to Link.
func ToItem(provider, source string, fi *gofeed.Item) model.Item {
	it := model.Item{
		Provider:   provider,
		Source:     source,
		ExternalID: firstNonEmpty(fi.GUID, fi.Link),
		Title:      strings.TrimSpace(fi.Title),
		URL:        fi.Link,
		Summary:    CleanHTML(firstNonEmpty(fi.Description, fi.Content)),
		Tags:       fi.Categories,
	}
	if fi.Author != nil && fi.Author.Name != "" {
		it.Author = strings.TrimPrefix(strings.TrimSpace(fi.Author.Name), "@")
	}
	if fi.PublishedParsed != nil {
		it.PublishedAt = *fi.PublishedParsed
	} else if fi.UpdatedParsed != nil {
		it.PublishedAt = *fi.UpdatedParsed
	}
	return it
}

// CleanHTML strips the small set of HTML tags commonly found in RSS
// descriptions and collapses whitespace, mirroring the Python fetcher's
// description sanitization.
func CleanHTML(s string) string {
	if s == "" {
		return ""
	}
	r := strings.NewReplacer(
		"<p>", "", "</p>", "\n",
		"<br>", "\n", "<br/>", "\n", "<br />", "\n",
	)
	s = r.Replace(s)
	s = stripTags(s)
	return strings.Join(strings.Fields(s), " ")
}

// stripTags removes any remaining "<...>" tags without a full HTML parser.
func stripTags(s string) string {
	var b strings.Builder
	depth := 0
	for _, r := range s {
		switch r {
		case '<':
			depth++
		case '>':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
