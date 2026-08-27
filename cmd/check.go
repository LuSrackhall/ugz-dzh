package cmd

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"ledger/balance"

	"github.com/spf13/cobra"
	"github.com/xuri/excelize/v2"
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

		// 未知类别科目提示（属性=未分类，不影响余额计算；N6：删除误导的 SetAccountProperty 提示——无此命令）
		unknown := false
		for account := range cfg.Tree {
			gen := account
			if idx := strings.IndexByte(gen, '-'); idx > 0 {
				gen = gen[:idx]
			}
			if balance.IsUnknownType(gen) {
				unknown = true
				fmt.Printf("提示: 科目 %s 类别未知（属性=未分类）——如需进入资产负债表/收支结余表，请在 JSON 科目顺序或类别中补充\n", account)
			}
		}
		if unknown {
			fmt.Println("（以上为未知类别提示，不影响余额计算）")
		}

		// xlsx 漂移比对（Change 12）：读最新月 xlsx 期末表，与 JSON Balances 对比（防手工改表漂移）
		if month := balance.LatestBalanceMonth(cfg); month != "" {
			xlsxPath := filepath.Join(filepath.Dir(configPath), month+".xlsx")
			f, err := excelize.OpenFile(xlsxPath)
			if err != nil {
				fmt.Printf("提示: 未找到 %s，跳过 xlsx 漂移比对\n", xlsxPath)
			} else {
				rows, _ := f.GetRows(month + "期末")
				f.Close()
				drift := 0
				checked := 0
				for _, row := range rows {
					if len(row) < 4 {
						continue
					}
					account := strings.TrimSpace(row[0])
					if account == "" || account == "科目" || account == "合计" {
						continue
					}
					node, ok := cfg.Tree[account]
					if !ok {
						continue
					}
					jsonFinal := node.Balances[month].Final
					xFinal := int64((parseYuanFloat(row[2]) - parseYuanFloat(row[3])) * 100)
					// 四舍五入到分
					if xFinal != jsonFinal {
						drift++
						fmt.Printf("⚠ 漂移: %s JSON=%.2f xlsx=%.2f\n", account, float64(jsonFinal)/100, float64(xFinal)/100)
					}
					checked++
				}
				if checked == 0 {
					fmt.Printf("提示: %s 期末表无可比对科目（可能未生成），跳过漂移比对\n", month)
				} else if drift == 0 {
					fmt.Printf("✓ xlsx 期末与 JSON 一致（%s，%d 个科目）\n", month, checked)
				} else {
					fmt.Printf("⚠ %d/%d 个科目 xlsx 与 JSON 漂移——以 JSON 为准，generate -f 可重建修复\n", drift, checked)
				}
			}
		}

		return nil
	},
}

// parseYuanFloat 解析金额字符串（如 "1,234.56" / "30000"）为元。
func parseYuanFloat(s string) float64 {
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}
