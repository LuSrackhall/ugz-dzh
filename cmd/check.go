package cmd

import (
	"fmt"

	"ledger/balance"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(checkCmd)
	checkCmd.Flags().StringP("json", "j", "", "科目余额总览.json 路径（必填）")
	checkCmd.MarkFlagRequired("json")
}

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "检测 JSON 科目树与余额完整性",
	Long:  "验证科目余额总览.json 中科目树的一致性，确保自动识别和手动调整科目与科目树一一对应。",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("json")

		cfg, err := balance.LoadConfig(configPath)
		if err != nil {
			return fmt.Errorf("加载配置: %w", err)
		}

		if err := balance.ValidateAccountTree(cfg); err != nil {
			return fmt.Errorf("科目树验证失败: %w", err)
		}

		fmt.Println("✓ 科目树验证通过")
		return nil
	},
}
