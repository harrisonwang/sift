package reporter

import (
	"encoding/json"
	"io"
	"time"

	"github.com/harrisonwang/sift/internal/model"
)

// JSON renders a machine-readable report: a small envelope plus the items.
type JSON struct{}

func (JSON) Ext() string { return "json" }

type jsonReport struct {
	GeneratedAt string       `json:"generated_at"`
	Total       int          `json:"total"`
	Items       []model.Item `json:"items"`
}

func (JSON) Render(w io.Writer, items []model.Item) error {
	if items == nil {
		items = []model.Item{}
	}
	rep := jsonReport{
		GeneratedAt: time.Now().Format(time.RFC3339),
		Total:       len(items),
		Items:       items,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(rep)
}
