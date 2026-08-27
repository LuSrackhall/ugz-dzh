package cmd

import (
	"fmt"

	"ledger/balance"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(lockCmd)
	lockCmd.Flags().StringP("json", "j", "", "科目余额总览.json 路径（必填）")
	lockCmd.Flags().StringP("month", "m", "", "结账月（如 2025-12）——该月及之前月份默认拒绝再次生成（-f 例外）")
	lockCmd.MarkFlagRequired("json")
	lockCmd.MarkFlagRequired("month")
}

var lockCmd = &cobra.Command{
	Use:   "lock",
	Short: "设置结账月（已结账月份默认拒绝无 -f 生成）",
	Long: "设置结账月后，generate 对 <= 结账月的月份默认报错（防误改已结账账本）；" +
		"确需修改时用 -f 强制重建。置空结账月（-m ''）可解锁。",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("json")
		month, _ := cmd.Flags().GetString("month")

		cfg, err := balance.LoadConfig(configPath)
		if err != nil {
			return fmt.Errorf("加载配置: %w", err)
		}
		cfg.Settings.ClosingMonth = month
		if err := balance.SaveConfig(configPath, cfg); err != nil {
			return fmt.Errorf("保存配置: %w", err)
		}
		if month == "" {
			fmt.Printf("已解锁：结账月清空\n")
		} else {
			fmt.Printf("已设置结账月 %s——%s 及之前月份默认拒绝无 -f 生成（防误改）\n", month, month)
		}
		return nil
	},
}
