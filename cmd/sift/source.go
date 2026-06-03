package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/harrisonwang/sift/internal/provider"
	"github.com/spf13/cobra"
)

func newSourceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "source",
		Short: "列出并查看可用的 provider 类型",
	}
	cmd.AddCommand(newSourceListCmd(), newSourceInfoCmd())
	return cmd
}

func newSourceListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "列出所有可用的 provider 类型",
		RunE: func(_ *cobra.Command, _ []string) error {
			for _, info := range provider.Infos() {
				fmt.Fprintf(os.Stdout, "  %-12s %s\n", info.Name, info.Summary)
			}
			return nil
		},
	}
}

func newSourceInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info <provider>",
		Short: "查看某个 provider 类型的配置项与示例",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			info, ok := provider.InfoFor(args[0])
			if !ok {
				return fmt.Errorf("未知 provider %q（可用：%s）",
					args[0], strings.Join(provider.Names(), ", "))
			}
			out := os.Stdout
			fmt.Fprintf(out, "%s\n%s\n\n", info.Name, info.Summary)
			fmt.Fprintf(out, "配置项：%s\n\n", strings.Join(info.ConfigKeys, ", "))
			if info.Example != "" {
				fmt.Fprintln(out, "示例：")
				fmt.Fprintln(out, info.Example)
			}
			return nil
		},
	}
}
