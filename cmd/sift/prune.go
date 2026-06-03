package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/harrisonwang/sift/internal/config"
	"github.com/harrisonwang/sift/internal/orchestrator"
	"github.com/spf13/cobra"
)

func newPruneCmd() *cobra.Command {
	var (
		ff  filterFlags
		all bool
	)
	cmd := &cobra.Command{
		Use:   "prune",
		Short: "按过滤条件删除缓存中的条目",
		Long: "按给定的过滤条件删除缓存条目。\n\n" +
			"常用于收紧某个 provider 的过滤后（如设置 include_replies: false）：\n" +
			"先 prune 受影响的源，再重新 discover。\n" +
			"必须至少指定一个过滤条件，或用 --all 清空整个缓存。\n\n" +
			"注意：--limit 会被忽略，prune 删除所有匹配的行。",
		Example: "  sift prune --source twitter:OpenAI\n" +
			"  sift prune --provider twitter\n" +
			"  sift prune --all",
		RunE: func(_ *cobra.Command, _ []string) error {
			filter := ff.toFilter()
			noFilter := filter.Date == "" && filter.Provider == "" &&
				filter.Source == "" && filter.Keyword == ""
			if noFilter && !all {
				return fmt.Errorf("拒绝在无过滤条件时清空整个缓存（请加 --all，或指定过滤条件）")
			}
			return withApp(func(o *orchestrator.Orchestrator, _ *config.Config, _ *slog.Logger) error {
				n, err := o.Prune(filter)
				if err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "已删除 %d 条\n", n)
				return nil
			})
		},
	}
	ff.bind(cmd)
	cmd.Flags().BoolVar(&all, "all", false, "在未指定过滤条件时允许清空整个缓存")
	return cmd
}
