package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version 构建时注入（goreleaser ldflags: -X ledger/cmd.version={{.Version}}）。
// 本地 go build 为 "dev"；发布包为 tag 版本（如 v0.7.2）。
var version = "dev"

var rootCmd = &cobra.Command{
	Use:     "ledger",
	Short:   "手工账电子化生成系统",
	Long:    "将手工记账凭证（Markdown 文件）自动转为每月独立、完整的累计 Excel 工作薄。",
	Version: version,
	Run: func(cmd *cobra.Command, args []string) {
		printGuide(cmd.Version)
	},
}

// printGuide — 无参数运行 ledger 时的 agent 使用引导。
// 用户画像=agent：首次接触即可按此理解完整体系，无需人工教学。
func printGuide(version string) {
	fmt.Printf(`ledger 手工账电子化系统 v%s — agent 使用引导
============================================
本系统由 agent 通过技能驱动（ledger install-skill 安装操作手册）。首次使用：

  1) ledger install-skill                  安装 ledger-accounting 技能（agent 操作手册，自包含）
  2) ledger init -s <YYYY-MM> -o <输出根>    建账，自动建好 vouchers/ 凭证目录 + print-config.json + README
  3) 凭证放 <输出根>/vouchers/YYYY_MM/*.md
  4) ledger generate -v <凭证目录> -o <输出根>  生成账本（自动发现 print-config.json）

常用命令：
  ledger doctor             环境自检（版本/skill/配置/账本，排障先跑这个）
  ledger generate ...       月度账本
  ledger year-close ...     跨年结转
  ledger check -j <json>    账本检查

查看全部命令: ledger --help    详细用法: ledger <命令> --help
`, version)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
