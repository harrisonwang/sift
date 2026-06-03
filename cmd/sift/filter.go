package main

import (
	"github.com/harrisonwang/sift/internal/cache"
	"github.com/spf13/cobra"
)

// filterFlags holds the common selection flags shared by query and report.
type filterFlags struct {
	date     string
	source   string
	provider string
	keyword  string
	limit    int
}

// bind registers the filter flags on cmd.
func (f *filterFlags) bind(cmd *cobra.Command) {
	fl := cmd.Flags()
	fl.StringVar(&f.date, "date", "", `按日期过滤：YYYY-MM-DD、"today" 或 "yesterday"`)
	fl.StringVar(&f.source, "source", "", "按具体来源过滤（如 twitter:msdev）")
	fl.StringVar(&f.provider, "provider", "", "按 provider 类型过滤（如 hackernews）")
	fl.StringVar(&f.keyword, "keyword", "", "标题/摘要的大小写不敏感子串匹配")
	fl.IntVar(&f.limit, "limit", 0, "返回条数上限（0 表示默认值）")
}

// toFilter converts the flags into a cache.Filter.
func (f *filterFlags) toFilter() cache.Filter {
	return cache.Filter{
		Date:     normalizeDate(f.date),
		Source:   f.source,
		Provider: f.provider,
		Keyword:  f.keyword,
		Limit:    f.limit,
	}
}
