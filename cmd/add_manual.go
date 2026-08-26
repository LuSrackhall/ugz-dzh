package cmd

import (
	"fmt"

	"ledger/balance"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(addManualCmd)
	addManualCmd.Flags().StringP("account", "a", "", "科目全路径（必填，如 银行存款-工商银行）")
	addManualCmd.Flags().StringP("month", "m", "", "补录时点 YYYY-MM（信息性记录，期初锚定建账月，不参与计算）")
	addManualCmd.Flags().Float64P("amount", "n", 0, "期初调整额（元，作用于建账月）")
	addManualCmd.Flags().StringP("note", "t", "", "说明")
	addManualCmd.Flags().StringP("json", "j", "", "科目余额总览.json 路径（必填）")
	addManualCmd.MarkFlagRequired("account")
	addManualCmd.MarkFlagRequired("month")
	addManualCmd.MarkFlagRequired("json")
}

var addManualCmd = &cobra.Command{
	Use:   "add-manual",
	Short: "设置/修改手动调整科目的建账月期初值",
	Long:  "向科目余额总览.json 中添加或更新手动调整科目条目，期初调整额作用于建账月（init 启动月）；同科目重复调用更新期初值。",
	RunE: func(cmd *cobra.Command, args []string) error {
		account, _ := cmd.Flags().GetString("account")
		month, _ := cmd.Flags().GetString("month")
		amount, _ := cmd.Flags().GetFloat64("amount")
		note, _ := cmd.Flags().GetString("note")
		configPath, _ := cmd.Flags().GetString("json")

		cfg, err := balance.LoadConfig(configPath)
		if err != nil {
			return fmt.Errorf("加载配置: %w", err)
		}

		if err := balance.AddManualAdjustment(cfg, account, month, amount, note); err != nil {
			return fmt.Errorf("添加手动调整: %w", err)
		}

		if err := balance.SaveConfig(configPath, cfg); err != nil {
			return fmt.Errorf("保存配置: %w", err)
		}

		fmt.Printf("已设置手动调整科目: %s (期初调整额 %.2f，作用于建账月)\n", account, amount)
		return nil
	},
}
