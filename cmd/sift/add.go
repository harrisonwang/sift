package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/harrisonwang/sift/internal/config"
	"github.com/harrisonwang/sift/internal/resolver"
	"github.com/spf13/cobra"
)

const addTimeout = 45 * time.Second

func newAddCmd() *cobra.Command {
	var (
		write    bool
		noVerify bool
		format   string
	)
	cmd := &cobra.Command{
		Use:   "add <input>...",
		Short: "识别信息源并加入雷达（URL / @handle / r/sub / 网站；默认 dry-run 输出 JSON）",
		Long: "把你知道的东西（一个 URL、@handle、r/subreddit、GitHub repo、或普通网站）\n" +
			"识别成具体的 sift 信息源,验证能抓到数据,并（加 --write 时）写入配置。\n\n" +
			"默认只识别 + 验证,输出 JSON,不改配置;加 --write 才写入。",
		Example: "  sift add @karpathy\n" +
			"  sift add https://simonwillison.net/\n" +
			"  sift add r/LocalLLaMA --write\n" +
			"  sift add @karpathy r/golang github.com/cli/cli",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), addTimeout)
			defer cancel()

			rsv := resolver.New()
			results := make([]resolver.Result, 0, len(args))
			for _, input := range args {
				results = append(results, resolveOne(ctx, rsv, input, write, noVerify))
			}

			if format == "text" {
				renderAddText(os.Stdout, results)
			} else if err := renderAddJSON(os.Stdout, results); err != nil {
				return err
			}
			if anyFailed(results) {
				// Non-zero exit so callers/agents can branch; details are in the output.
				return errSilent
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&write, "write", false, "写入配置（默认只识别和验证，不改配置）")
	cmd.Flags().BoolVar(&noVerify, "no-verify", false, "跳过抓取验证（离线 / CI）")
	cmd.Flags().StringVarP(&format, "format", "f", "json", "输出格式：json|text")
	return cmd
}

// errSilent signals a non-zero exit without printing an extra error line (the
// structured output already carries the detail).
var errSilent = &silentErr{}

type silentErr struct{}

func (*silentErr) Error() string { return "" }

func resolveOne(ctx context.Context, rsv *resolver.Resolver, input string, write, noVerify bool) resolver.Result {
	res := rsv.Resolve(ctx, input)
	if res.Resolved && !noVerify {
		rsv.Verify(ctx, &res)
	}
	res.Suggestion = suggestionYAML(res)

	if write && res.Resolved && (noVerify || res.Verified) {
		added, err := config.AddSource(flagConfig, res.Provider, res.Key, res.FeedURL, res.Source)
		switch {
		case err == nil:
			res.Written = added
			res.Duplicate = !added
		case os.IsNotExist(err):
			res.Error = "未找到配置文件,请先运行 `sift config init`"
		default:
			res.Error = err.Error()
		}
	}
	return res
}

func anyFailed(results []resolver.Result) bool {
	for _, r := range results {
		if !r.Resolved || r.Error != "" {
			return true
		}
	}
	return false
}

func renderAddJSON(w io.Writer, results []resolver.Result) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(struct {
		Results []resolver.Result `json:"results"`
	}{results})
}

func renderAddText(w io.Writer, results []resolver.Result) {
	for _, r := range results {
		if !r.Resolved {
			fmt.Fprintf(w, "✗ %s — %s\n\n", r.Input, r.Error)
			continue
		}
		fmt.Fprintf(w, "✓ 识别为 %s\n", describeSource(r))
		if r.Ambiguous && len(r.Candidates) > 1 {
			fmt.Fprintln(w, "  发现多个 feed，默认选第一个；其它候选：")
			for _, c := range r.Candidates[1:] {
				fmt.Fprintf(w, "    - %s\n", c)
			}
		}
		switch {
		case r.Verified:
			line := fmt.Sprintf("✓ 验证成功：%d 条", r.SampleCount)
			if r.LatestTitle != "" {
				line += "，最新：" + truncate(r.LatestTitle, 50)
			}
			fmt.Fprintln(w, line)
		case r.Error != "":
			fmt.Fprintf(w, "✗ %s\n", r.Error)
		}
		switch {
		case r.Written:
			fmt.Fprintln(w, "✓ 已写入配置")
		case r.Duplicate:
			fmt.Fprintln(w, "• 已存在，未重复添加")
		case r.Error == "" && r.Suggestion != "":
			fmt.Fprintf(w, "\n建议配置片段：\n%s\n如需写入：sift add %s --write\n", indentLines(r.Suggestion, "  "), r.Input)
		}
		fmt.Fprintln(w)
	}
}

func describeSource(r resolver.Result) string {
	switch r.Provider {
	case resolver.ProviderTwitter:
		return "Twitter/X 账号：" + r.Key
	case resolver.ProviderReddit:
		return "Reddit 子版块：r/" + r.Key
	case resolver.ProviderHackerNews:
		return "Hacker News"
	case resolver.ProviderRSSBlog:
		return "RSS/Atom 源：" + r.FeedURL
	default:
		return r.Provider
	}
}

func suggestionYAML(r resolver.Result) string {
	switch r.Provider {
	case resolver.ProviderTwitter:
		return fmt.Sprintf("providers:\n  - name: twitter\n    enabled: true\n    config:\n      accounts:\n        - %s\n", r.Key)
	case resolver.ProviderReddit:
		return fmt.Sprintf("providers:\n  - name: reddit\n    enabled: true\n    config:\n      subreddits:\n        - %s\n", r.Key)
	case resolver.ProviderHackerNews:
		return "providers:\n  - name: hackernews\n    enabled: true\n    config:\n      feeds: [top]\n"
	case resolver.ProviderRSSBlog:
		return fmt.Sprintf("providers:\n  - name: rssblog\n    enabled: true\n    config:\n      feeds:\n        - url: %s\n          source: %s\n", r.FeedURL, r.Source)
	default:
		return ""
	}
}

func indentLines(s, prefix string) string {
	var b strings.Builder
	for line := range strings.SplitSeq(strings.TrimRight(s, "\n"), "\n") {
		b.WriteString(prefix)
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
