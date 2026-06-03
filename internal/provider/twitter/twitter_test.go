package twitter

import (
	"testing"

	"github.com/mmcdole/gofeed"
)

func TestParseStatusLink(t *testing.T) {
	cases := []struct {
		link, author, id string
	}{
		{"https://nitter.net/msdev/status/1234567890#m", "msdev", "1234567890"},
		{"https://nitter.net/msdev/status/1234567890", "msdev", "1234567890"},
		{"https://x.com/OpenAI/status/999", "OpenAI", "999"},
		{"https://nitter.net/msdev", "", ""},                  // no status segment
		{"", "", ""},                                          // empty
		{"https://nitter.net/a/status/12ab34#m", "a", "1234"}, // strips non-digits
	}
	for _, tc := range cases {
		author, id := parseStatusLink(tc.link)
		if author != tc.author || id != tc.id {
			t.Errorf("parseStatusLink(%q) = (%q,%q), want (%q,%q)",
				tc.link, author, id, tc.author, tc.id)
		}
	}
}

func TestToTweet(t *testing.T) {
	// Link carries the canonical author + numeric id.
	fi := &gofeed.Item{
		Link:        "https://nitter.net/msdev/status/2061891029754904654#m",
		Title:       "Hello world",
		Description: "<p>Hello world</p>",
	}
	it, ok := toTweet("msdev", fi)
	if !ok {
		t.Fatal("expected ok")
	}
	if it.ExternalID != "2061891029754904654" {
		t.Errorf("external_id: %q", it.ExternalID)
	}
	if it.Author != "msdev" {
		t.Errorf("author: %q", it.Author)
	}
	if it.URL != "https://x.com/msdev/status/2061891029754904654" {
		t.Errorf("url: %q", it.URL)
	}
	if it.Source != "twitter:msdev" {
		t.Errorf("source: %q", it.Source)
	}
}

func TestToTweet_DropsNonStatusEntries(t *testing.T) {
	// A pinned/profile entry with no numeric status id is dropped.
	fi := &gofeed.Item{Link: "https://nitter.net/msdev", GUID: "not-a-number"}
	if _, ok := toTweet("msdev", fi); ok {
		t.Fatal("expected entry without a tweet id to be dropped")
	}
}

func TestToTweet_NumericGUIDFallback(t *testing.T) {
	// Some instances only provide a bare numeric guid.
	fi := &gofeed.Item{GUID: "12345", Title: "x"}
	it, ok := toTweet("acct", fi)
	if !ok || it.ExternalID != "12345" {
		t.Fatalf("guid fallback failed: ok=%v id=%q", ok, it.ExternalID)
	}
	if it.Author != "acct" {
		t.Errorf("author should default to account: %q", it.Author)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		name, account, title, author string
		want                         tweetKind
	}{
		{"original", "msdev", "We shipped a thing", "msdev", kindOriginal},
		{"retweet by title", "msdev", "RT by @msdev: cool news", "msdev", kindRetweet},
		{"retweet by author", "msdev", "cool news", "Microsoft", kindRetweet},
		{"reply", "msdev", "R to @someone: thanks", "msdev", kindReply},
		{"original case-insensitive author", "MSDev", "hello", "msdev", kindOriginal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classify(tc.account, tc.title, tc.author); got != tc.want {
				t.Errorf("classify(%q,%q,%q) = %d, want %d",
					tc.account, tc.title, tc.author, got, tc.want)
			}
		})
	}
}

func TestDigitsOnly(t *testing.T) {
	if got := digitsOnly("12ab34"); got != "1234" {
		t.Errorf("digitsOnly = %q", got)
	}
	if got := digitsOnly("none"); got != "" {
		t.Errorf("digitsOnly = %q", got)
	}
}
