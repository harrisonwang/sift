package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"time"

	"github.com/harrisonwang/sift/internal/config"
	"github.com/harrisonwang/sift/internal/orchestrator"
	"github.com/spf13/cobra"
)

// discoverTimeout caps a full discover run.
const discoverTimeout = 5 * time.Minute

// discoverSourceJSON / discoverResultJSON are the machine-readable summary
// emitted to stdout, so automations can check per-source success and counts.
type discoverSourceJSON struct {
	Provider string `json:"provider"`
	Fetched  int    `json:"fetched"`
	New      int    `json:"new"`
	OK       bool   `json:"ok"`
	Error    string `json:"error,omitempty"`
}

type discoverResultJSON struct {
	Fetched int                  `json:"fetched"`
	New     int                  `json:"new"`
	Sources []discoverSourceJSON `json:"sources"`
}

func newDiscoverCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "discover",
		Short: "抓取所有启用 provider 的新条目并写入缓存（向 stdout 输出 JSON 摘要）",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return withApp(func(o *orchestrator.Orchestrator, _ *config.Config, _ *slog.Logger) error {
				ctx, cancel := context.WithTimeout(cmd.Context(), discoverTimeout)
				defer cancel()

				stats, discErr := o.Discover(ctx)

				out := discoverResultJSON{
					Fetched: stats.TotalFetched,
					New:     stats.TotalNew,
					Sources: make([]discoverSourceJSON, 0, len(stats.Results)),
				}
				for _, r := range stats.Results {
					s := discoverSourceJSON{
						Provider: r.Provider,
						Fetched:  r.Fetched,
						New:      r.New,
						OK:       r.Err == nil,
					}
					if r.Err != nil {
						s.Error = r.Err.Error()
					}
					out.Sources = append(out.Sources, s)
				}

				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				enc.SetEscapeHTML(false)
				if err := enc.Encode(out); err != nil {
					return err
				}
				return discErr
			})
		},
	}
}
