package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/harrisonwang/sift/internal/cache"
	"github.com/harrisonwang/sift/internal/config"
	"github.com/harrisonwang/sift/internal/orchestrator"
	"github.com/spf13/cobra"
)

// Persistent flags shared by all commands.
var (
	flagConfig  string
	flagDBPath  string
	flagVerbose bool
	flagQuiet   bool
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "sift",
		Short: "sift — provider 驱动的技术资讯雷达",
		Long: "sift 从配置的数据源（Twitter/Nitter、Hacker News、Reddit、RSS 博客）抓取条目，\n" +
			"用 SQLite 缓存去重，并生成 Markdown/JSON 报告。\n\n" +
			"首次使用请先运行 `sift config init` 创建默认配置文件，\n" +
			"然后编辑 ~/.sift/config.yaml 并运行 `sift config validate` 校验。\n\n" +
			"所有命令的结果输出到 stdout，日志输出到 stderr。",
		Example: "  sift config init                     # 首次使用：创建 ~/.sift/config.yaml\n" +
			"  sift config validate                 # 校验配置与各 provider 设置\n" +
			"  sift discover                        # 抓取新条目并写入缓存\n" +
			"  sift query --date today              # 查询今天的条目（JSON）\n" +
			"  sift report --date today -o today.md # 生成 Markdown 报告文件",
		Version:       versionString(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	// 隐藏自动生成的 completion 子命令，让帮助更干净。
	root.CompletionOptions.HiddenDefaultCmd = true
	// 用中文 usage 模板替换 cobra 默认的英文结构标题（root 设置，子命令继承）。
	root.SetUsageTemplate(usageTemplateZh)

	pf := root.PersistentFlags()
	pf.StringVarP(&flagConfig, "config", "c", "", "配置文件路径（默认 ~/.sift/config.yaml）")
	pf.StringVar(&flagDBPath, "db", "", "覆盖配置中的 cache db_path（默认 ~/.sift/sift.db）")
	pf.BoolVarP(&flagVerbose, "verbose", "v", false, "输出调试日志（debug 级别）")
	pf.BoolVarP(&flagQuiet, "quiet", "q", false, "静默：仅输出错误日志")

	// 本地化 cobra 内置的 -h/--help 与 --version 文案（pith 风格）。
	pf.BoolP("help", "h", false, "显示帮助")
	root.Flags().BoolP("version", "V", false, "显示版本")
	root.SetVersionTemplate("{{.Name}} {{.Version}}\n")

	root.AddCommand(
		newDiscoverCmd(),
		newQueryCmd(),
		newReportCmd(),
		newPruneCmd(),
		newConfigCmd(),
		newSourceCmd(),
	)

	// 本地化自动生成的 help 子命令文案。
	root.InitDefaultHelpCmd()
	for _, c := range root.Commands() {
		if c.Name() == "help" {
			c.Short = "查看任意命令的帮助"
		}
	}
	return root
}

// versionString returns the release version injected by ldflags.
func versionString() string {
	return version
}

// newLogger returns a stderr slog logger (stdout is reserved for command
// output). --quiet raises the threshold to errors only; --verbose lowers it to
// debug; --quiet wins if both are set.
func newLogger() *slog.Logger {
	level := slog.LevelInfo
	switch {
	case flagQuiet:
		level = slog.LevelError
	case flagVerbose:
		level = slog.LevelDebug
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	}))
}

// loadConfig loads the config file and applies CLI overrides.
func loadConfig() (*config.Config, error) {
	cfg, err := config.Load(flagConfig)
	if err != nil {
		return nil, err
	}
	if flagDBPath != "" {
		cfg.Cache.DBPath = flagDBPath
	}
	return cfg, nil
}

// withApp loads config, ensures directories, opens the cache, builds the
// orchestrator, runs fn, then cleans up.
func withApp(fn func(o *orchestrator.Orchestrator, cfg *config.Config, log *slog.Logger) error) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if err := cfg.EnsureDirs(); err != nil {
		return err
	}
	log := newLogger()
	c, err := cache.Open(cfg.Cache.DBPath)
	if err != nil {
		return err
	}
	defer c.Close()
	return fn(orchestrator.New(cfg, c, log), cfg, log)
}

// normalizeDate maps "today"/"yesterday" to a YYYY-MM-DD string; other values
// pass through unchanged (empty means "no date filter").
func normalizeDate(s string) string {
	switch s {
	case "today":
		return time.Now().Format("2006-01-02")
	case "yesterday":
		return time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	default:
		return s
	}
}

// usageTemplateZh is cobra's default usage template with the structural labels
// translated to Chinese. Only the literal labels change; the template logic and
// the registered helpers (rpad / trimTrailingWhitespaces) are preserved.
const usageTemplateZh = `用法：{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [命令]{{end}}{{if gt (len .Aliases) 0}}

别名：
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

示例：
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

可用命令：{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

参数：
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

全局参数：
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

其它帮助主题：{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

使用 "{{.CommandPath}} [命令] --help" 查看某个子命令的更多信息。{{end}}
`
