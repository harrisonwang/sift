package feedx

import (
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
)

func TestCleanHTML(t *testing.T) {
	cases := map[string]struct{ in, want string }{
		"paragraphs": {"<p>Hello</p><p>World</p>", "Hello World"},
		"breaks":     {"line1<br>line2<br/>line3", "line1 line2 line3"},
		"tags":       {`<a href="x">link</a> text`, "link text"},
		"whitespace": {"  too   many\n\nspaces ", "too many spaces"},
		"empty":      {"", ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := CleanHTML(tc.in); got != tc.want {
				t.Errorf("CleanHTML(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestToItem_MapsFields(t *testing.T) {
	pub := time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC)
	fi := &gofeed.Item{
		GUID:            "guid-1",
		Title:           "  Title  ",
		Link:            "https://example.com/a",
		Description:     "<p>desc</p>",
		Categories:      []string{"go", "news"},
		Author:          &gofeed.Person{Name: "@alice"},
		PublishedParsed: &pub,
	}
	it := ToItem("rssblog", "blog", fi)
	if it.Provider != "rssblog" || it.Source != "blog" {
		t.Errorf("provider/source: %+v", it)
	}
	if it.ExternalID != "guid-1" {
		t.Errorf("external_id: %q", it.ExternalID)
	}
	if it.Title != "Title" {
		t.Errorf("title not trimmed: %q", it.Title)
	}
	if it.Summary != "desc" {
		t.Errorf("summary not cleaned: %q", it.Summary)
	}
	if it.Author != "alice" {
		t.Errorf("author @ not stripped: %q", it.Author)
	}
	if !it.PublishedAt.Equal(pub) {
		t.Errorf("published: %v", it.PublishedAt)
	}
}

func TestToItem_GUIDFallsBackToLink(t *testing.T) {
	fi := &gofeed.Item{Link: "https://example.com/x"}
	if got := ToItem("p", "s", fi).ExternalID; got != "https://example.com/x" {
		t.Errorf("expected link fallback, got %q", got)
	}
}
