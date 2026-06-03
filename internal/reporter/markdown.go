package reporter

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/harrisonwang/sift/internal/model"
)

// Markdown renders a human-readable report grouped by source.
type Markdown struct{}

func (Markdown) Ext() string { return "md" }

func (Markdown) Render(w io.Writer, items []model.Item) error {
	var b strings.Builder

	b.WriteString("# Tech Radar Digest\n\n")
	fmt.Fprintf(&b, "> Generated: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "> Items: %d\n\n", len(items))

	if len(items) == 0 {
		b.WriteString("_No items for the selected filter._\n")
		_, err := io.WriteString(w, b.String())
		return err
	}

	groups, order := groupBySource(items)
	for _, src := range order {
		group := groups[src]
		fmt.Fprintf(&b, "## %s  _(%d)_\n\n", src, len(group))
		for i, it := range group {
			fmt.Fprintf(&b, "### %d. %s\n\n", i+1, mdEscape(it.TitleOrSummary()))
			if it.Author != "" {
				fmt.Fprintf(&b, "- **Author**: %s\n", it.Author)
			}
			if it.URL != "" {
				fmt.Fprintf(&b, "- **Link**: %s\n", it.URL)
			}
			if !it.PublishedAt.IsZero() {
				fmt.Fprintf(&b, "- **Published**: %s\n", it.PublishedAt.Format("2006-01-02 15:04"))
			}
			if it.Score > 0 {
				fmt.Fprintf(&b, "- **Score**: %d\n", it.Score)
			}
			if len(it.Tags) > 0 {
				fmt.Fprintf(&b, "- **Tags**: %s\n", strings.Join(it.Tags, ", "))
			}
			if s := strings.TrimSpace(it.Summary); s != "" {
				b.WriteString("\n")
				fmt.Fprintf(&b, "> %s\n", truncate(collapseWS(s), 500))
			}
			b.WriteString("\n---\n\n")
		}
	}

	_, err := io.WriteString(w, b.String())
	return err
}

// groupBySource buckets items by Source and returns groups plus a stable order
// (sources sorted alphabetically; items within a group newest-first).
func groupBySource(items []model.Item) (map[string][]model.Item, []string) {
	groups := map[string][]model.Item{}
	for _, it := range items {
		src := it.Source
		if src == "" {
			src = it.Provider
		}
		groups[src] = append(groups[src], it)
	}
	order := make([]string, 0, len(groups))
	for src := range groups {
		order = append(order, src)
	}
	sort.Strings(order)
	for _, src := range order {
		g := groups[src]
		sort.SliceStable(g, func(i, j int) bool {
			return g[i].PublishedAt.After(g[j].PublishedAt)
		})
		groups[src] = g
	}
	return groups, order
}

func mdEscape(s string) string {
	// Keep titles single-line; escape leading markdown that would break headings.
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

func collapseWS(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
