// Package reporter renders a slice of items into a report format. Reporters are
// decoupled from sources: they consume the standardized model.Item only.
package reporter

import (
	"fmt"
	"io"

	"github.com/harrisonwang/sift/internal/model"
)

// Reporter renders items to a writer.
type Reporter interface {
	// Render writes the report for items to w.
	Render(w io.Writer, items []model.Item) error
	// Ext is the conventional file extension (without dot), e.g. "md".
	Ext() string
}

// For returns the reporter for a format name ("markdown"/"md" or "json").
func For(format string) (Reporter, error) {
	switch format {
	case "markdown", "md", "":
		return Markdown{}, nil
	case "json":
		return JSON{}, nil
	default:
		return nil, fmt.Errorf("unknown report format %q (want markdown|json)", format)
	}
}
