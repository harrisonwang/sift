package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/harrisonwang/sift/internal/config"
	"github.com/harrisonwang/sift/internal/orchestrator"
	"github.com/harrisonwang/sift/internal/reporter"
	"github.com/spf13/cobra"
)

func newReportCmd() *cobra.Command {
	var (
		ff     filterFlags
		format string
		output string
	)
	cmd := &cobra.Command{
		Use:   "report",
		Short: "从缓存生成报告（默认输出 Markdown 到 stdout）",
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

				// Default: stream the report to stdout.
				if output == "" {
					return rep.Render(os.Stdout, items)
				}

				// -o <file>: write to the given path.
				if dir := filepath.Dir(output); dir != "" && dir != "." {
					if err := os.MkdirAll(dir, 0o755); err != nil {
						return fmt.Errorf("create output dir: %w", err)
					}
				}
				f, err := os.Create(output)
				if err != nil {
					return fmt.Errorf("create report file: %w", err)
				}
				defer f.Close()
				if err := rep.Render(f, items); err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "已写入报告：%s（%d 条）\n", output, len(items))
				return nil
			})
		},
	}
	ff.bind(cmd)
	cmd.Flags().StringVarP(&format, "format", "f", "markdown", "报告格式：markdown|json")
	cmd.Flags().StringVarP(&output, "output", "o", "", "写入到该文件，而非 stdout")
	return cmd
}
