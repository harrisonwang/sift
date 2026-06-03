package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/harrisonwang/sift/internal/config"
	"github.com/harrisonwang/sift/internal/provider"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "查看与校验配置",
	}
	cmd.AddCommand(newConfigInitCmd(), newConfigValidateCmd(), newConfigShowCmd())
	return cmd
}

func newConfigInitCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "创建默认配置文件（默认 ~/.sift/config.yaml）",
		RunE: func(_ *cobra.Command, _ []string) error {
			path, created, err := config.InitDefaultConfig(flagConfig, force)
			if err != nil {
				return err
			}
			if created {
				fmt.Fprintf(os.Stdout, "已创建配置文件：%s\n请按需编辑后运行：sift config validate\n", path)
				return nil
			}
			fmt.Fprintf(os.Stdout, "配置文件已存在，未覆盖：%s\n如需覆盖，请运行：sift config init --force\n", path)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "覆盖已存在的配置文件")
	return cmd
}

func newConfigValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "校验配置文件（含各 provider 的设置）",
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			// Build every enabled provider to surface provider-specific errors.
			log := newLogger()
			var problems []string
			for _, pc := range cfg.EnabledProviders() {
				if _, err := provider.Build(pc, log); err != nil {
					problems = append(problems, fmt.Sprintf("  %s: %v", pc.Name, err))
				}
			}
			if len(problems) > 0 {
				return fmt.Errorf("配置无效：\n%s", strings.Join(problems, "\n"))
			}
			fmt.Fprintf(os.Stdout, "配置 OK：共 %d 个 provider，%d 个已启用\n",
				len(cfg.Providers), len(cfg.EnabledProviders()))
			return nil
		},
	}
}

func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "打印当前生效的配置与 provider",
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			out := os.Stdout
			fmt.Fprintf(out, "cache.db_path: %s\n\n", cfg.Cache.DBPath)
			fmt.Fprintln(out, "providers:")
			for _, pc := range cfg.Providers {
				state := "已禁用"
				if pc.Enabled {
					state = "已启用"
				}
				fmt.Fprintf(out, "  - %s [%s]\n", pc.Name, state)
				if body := renderConfigNode(pc); body != "" {
					fmt.Fprint(out, body)
				}
			}
			return nil
		},
	}
}

// renderConfigNode re-marshals a provider's config block, indented for display.
func renderConfigNode(pc config.ProviderConfig) string {
	if pc.Config.Kind == 0 {
		return ""
	}
	raw, err := yaml.Marshal(&pc.Config)
	if err != nil {
		return ""
	}
	var b strings.Builder
	for line := range strings.SplitSeq(strings.TrimRight(string(raw), "\n"), "\n") {
		b.WriteString("      ")
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}
