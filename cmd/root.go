package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     "ledger",
	Short:   "手工账电子化生成系统",
	Long:    "将手工记账凭证（Markdown 文件）自动转为每月独立、完整的累计 Excel 工作薄。",
	Version: "1.0.0",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
