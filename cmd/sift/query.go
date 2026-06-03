package main

import (
	"log/slog"
	"os"

	"github.com/harrisonwang/sift/internal/config"
	"github.com/harrisonwang/sift/internal/orchestrator"
	"github.com/harrisonwang/sift/internal/reporter"
	"github.com/spf13/cobra"
)

func newQueryCmd() *cobra.Command {
	var (
		ff     filterFlags
		format string
	)
	cmd := &cobra.Command{
		Use:   "query",
		Short: "查询缓存中的条目并输出到 stdout（默认 JSON）",
		RunE: func(_ *cobra.Command, _ []string) error {
			return withApp(func(o *orchestrator.Orchestrator, _ *config.Config, _ *slog.Logger) error {
				rep, err := reporter.For(format)
				if err != nil {
					return err
				}
				items, err := o.Query(ff.toFilter())
				if err != nil {
					return err
				}
				return rep.Render(os.Stdout, items)
			})
		},
	}
	ff.bind(cmd)
	cmd.Flags().StringVarP(&format, "format", "f", "json", "输出格式：json|markdown")
	return cmd
}
