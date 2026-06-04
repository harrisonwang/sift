package resolver

import (
	"context"
	"testing"
)

func TestResolve_PatternMatched(t *testing.T) {
	rsv := New()
	cases := []struct{ in, provider, key string }{
		{"@karpathy", "twitter", "karpathy"},
		{"https://x.com/OpenAI", "twitter", "OpenAI"},
		{"twitter.com/msdev", "twitter", "msdev"},
		{"r/golang", "reddit", "golang"},
		{"https://reddit.com/r/LocalLLaMA", "reddit", "LocalLLaMA"},
		{"hn", "hackernews", ""},
		{"news.ycombinator.com", "hackernews", ""},
		{"https://example.com/index.xml", "rssblog", "https://example.com/index.xml"},
		{"https://blog.example.com/feed", "rssblog", "https://blog.example.com/feed"},
		{"github.com/cli/cli", "rssblog", "https://github.com/cli/cli/releases.atom"},
	}
	for _, c := range cases {
		r := rsv.Resolve(context.Background(), c.in)
		if !r.Resolved {
			t.Errorf("%q: not resolved (%s)", c.in, r.Error)
			continue
		}
		if r.Provider != c.provider || r.Key != c.key {
			t.Errorf("%q => provider=%q key=%q, want %q/%q", c.in, r.Provider, r.Key, c.provider, c.key)
		}
	}
}

func TestResolve_Unresolved(t *testing.T) {
	rsv := New()
	for _, in := range []string{"", "karpathy", "just words"} {
		if r := rsv.Resolve(context.Background(), in); r.Resolved {
			t.Errorf("%q should be unresolved", in)
		}
	}
}

func TestSlugFromHost(t *testing.T) {
	cases := map[string]string{
		"simonwillison.net":     "simonwillison",
		"www.simonwillison.net": "simonwillison",
		"news.ycombinator.com":  "news_ycombinator",
		"blog.google":           "blog",
	}
	for host, want := range cases {
		if got := slugFromHost(host); got != want {
			t.Errorf("slugFromHost(%q) = %q, want %q", host, got, want)
		}
	}
}
