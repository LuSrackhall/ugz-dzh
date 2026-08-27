package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"ledger/balance"

	"github.com/spf13/cobra"
	"github.com/xuri/excelize/v2"
)

func init() {
	rootCmd.AddCommand(yearCloseCmd)
	yearCloseCmd.Flags().StringP("json", "j", "", "科目余额总览.json 路径（必填）")
	yearCloseCmd.Flags().StringP("output", "o", ".", "输出根目录")
	yearCloseCmd.MarkFlagRequired("json")
}

var yearCloseCmd = &cobra.Command{
	Use:   "year-close",
	Short: "跨年结转",
	Long:  "将各科目年末余额结转为新年度的期初余额，生成新年度首月 xlsx。\nJSON 所在目录即为对应年份的输出目录。",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("json")
		output, _ := cmd.Flags().GetString("output")

		cfg, err := balance.LoadConfig(configPath)
		if err != nil {
			return fmt.Errorf("加载配置: %w", err)
		}

		lastMonth := ""
		for _, node := range cfg.Tree {
			for m := range node.Balances {
				if m > lastMonth {
					lastMonth = m
				}
			}
		}

		if lastMonth == "" {
			return fmt.Errorf("科目树中无余额记录，无法结转")
		}

		// 第三轮审查 F1：跨年结转校验（告警不阻断，结转是用户主动操作）
		if lastMonth[5:] != "12" {
			fmt.Printf("警告: 最近余额月为 %s，不是 12 月——跨年结转将按 %s 期末结转，请确认\n", lastMonth, lastMonth)
		}
		if diff := balance.CheckInitialBalanceAt(cfg, lastMonth); diff != 0 {
			fmt.Printf("警告: 期初借贷不平衡（%s 快照），差额 %.2f 元（借正贷负），请核对期初设置\n", lastMonth, float64(diff)/100)
		}
		hasUnclosedPnl := false
		for account, node := range cfg.Tree {
			mb, ok := node.Balances[lastMonth]
			if !ok || mb.Final == 0 {
				continue
			}
			gen := account
			if idx := strings.IndexByte(gen, '-'); idx > 0 {
				gen = gen[:idx]
			}
			if t, ok := balance.AccountTypeOf(gen); ok && (t == "收入" || t == "费用") {
				hasUnclosedPnl = true
				fmt.Printf("警告: 损益类科目 %s 年末余额 %.2f 元非 0，疑似漏结转（收入/费用应结转归零）\n", account, float64(mb.Final)/100)
				// 结转草稿（设计专家审查 Change 9）：方向由余额符号决定（验收修正）——
				// 借余（final>0）→ 借 本年收益 / 贷 科目；贷余（final<0）→ 借 科目 / 贷 本年收益。
				// 目标科目=本年收益（权益类）。
				abs := mb.Final
				if abs < 0 {
					abs = -abs
				}
				var draft string
				if mb.Final > 0 {
					draft = fmt.Sprintf("借 本年收益 %.2f / 贷 %s %.2f", float64(abs)/100, account, float64(abs)/100)
				} else {
					draft = fmt.Sprintf("借 %s %.2f / 贷 本年收益 %.2f", account, float64(abs)/100, float64(abs)/100)
				}
				fmt.Printf("  结转草稿: %s\n", draft)
			}
		}
		if hasUnclosedPnl {
			fmt.Printf("提示: 可用 ledger gen-close -j %s -o %s 自动生成结转凭证到 closing/ 目录（不污染手工凭证目录）\n", configPath, output)
		}

		yy, _ := strconv.Atoi(lastMonth[:4])
		nextYear := fmt.Sprintf("%04d-01", yy+1)

		nextYearDir := filepath.Join(output, fmt.Sprintf("%04d", yy+1))
		newPath := filepath.Join(nextYearDir, nextYear+".xlsx")

		if err := os.MkdirAll(nextYearDir, 0o755); err != nil {
			return fmt.Errorf("创建新年度目录: %w", err)
		}

		// 创建干净的空工作薄（不带旧年度的明细数据）
		f := excelize.NewFile()
		f.DeleteSheet("Sheet1")
		defer f.Close()

		if err := f.SaveAs(newPath); err != nil {
			return fmt.Errorf("保存 %s: %w", newPath, err)
		}

		// 生产门槛（设计专家审查 Change 9）：自动生成新年度 JSON（科目树/映射/期初调整/余额历史保留）
		newJSON := filepath.Join(nextYearDir, fmt.Sprintf("%04d.json", yy+1))
		if b, err := os.ReadFile(configPath); err == nil {
			if err := os.WriteFile(newJSON, b, 0o644); err != nil {
				return fmt.Errorf("复制新年度 JSON: %w", err)
			}
			fmt.Printf("已生成新年度配置 %s（科目树/映射/期初调整保留）\n", newJSON)
		} else {
			fmt.Printf("警告: 无法读取 %s，未生成新年度 JSON（请手动复制）\n", configPath)
		}

		fmt.Printf("已生成 %s 跨年结转工作薄\n", nextYear)
		fmt.Printf("提示: 期初调整锚定建账月 %s——新年度期初=%s 期末自动结转；如需调整新年期初，请改 JSON 后用 generate -f 从建账月重建\n", cfg.Settings.StartMonth, lastMonth)
		return nil
	},
}
