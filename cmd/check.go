package cmd

import (
	"fmt"
	"strings"

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

		// 期初试算平衡校验（最新月份快照，借正贷负求和，0=平衡）
		if month := balance.LatestBalanceMonth(cfg); month != "" {
			if diff := balance.CheckInitialBalanceAt(cfg, month); diff != 0 {
				return fmt.Errorf("期初借贷不平衡（%s 快照），差额 %.2f 元（借正贷负）。请核对期初设置", month, float64(diff)/100)
			}
			fmt.Printf("✓ 期初试算平衡校验通过（%s 快照）\n", month)
		}

		// 未知类别科目提示（属性默认按"借"处理，可 SetAccountProperty 修正）
		unknown := false
		for account := range cfg.Tree {
			gen := account
			if idx := strings.IndexByte(gen, '-'); idx > 0 {
				gen = gen[:idx]
			}
			if balance.IsUnknownType(gen) {
				unknown = true
				fmt.Printf("提示: 科目 %s 类别未知，属性按\"借\"处理；可用 SetAccountProperty 修正\n", account)
			}
		}
		if unknown {
			fmt.Println("（以上为未知类别提示，不影响余额计算）")
		}

		return nil
	},
}
