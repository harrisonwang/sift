package orchestrator

import (
	"testing"
	"time"

	"github.com/harrisonwang/sift/internal/model"
)

func TestPublishedToday(t *testing.T) {
	loc := time.FixedZone("CST", 8*60*60)
	now := time.Date(2026, 6, 4, 10, 0, 0, 0, loc)

	tests := []struct {
		name      string
		published time.Time
		want      bool
	}{
		{
			name:      "same local day",
			published: time.Date(2026, 6, 4, 8, 30, 0, 0, loc),
			want:      true,
		},
		{
			name:      "utc timestamp on same local day",
			published: time.Date(2026, 6, 3, 18, 30, 0, 0, time.UTC),
			want:      true,
		},
		{
			name:      "previous local day",
			published: time.Date(2026, 6, 3, 15, 59, 0, 0, time.UTC),
			want:      false,
		},
		{
			name:      "missing published time",
			published: time.Time{},
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := publishedToday(model.Item{PublishedAt: tt.published}, now)
			if got != tt.want {
				t.Fatalf("publishedToday() = %v, want %v", got, tt.want)
			}
		})
	}
}
