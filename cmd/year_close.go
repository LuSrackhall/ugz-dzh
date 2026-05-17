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

		yy, _ := strconv.Atoi(lastMonth[:4])
		prevDec := fmt.Sprintf("%04d-12", yy)
		nextYear := fmt.Sprintf("%04d-01", yy+1)

		prevYearDir := filepath.Join(output, fmt.Sprintf("%04d", yy))
		nextYearDir := filepath.Join(output, fmt.Sprintf("%04d", yy+1))

		prevPath := filepath.Join(prevYearDir, prevDec+".xlsx")
		newPath := filepath.Join(nextYearDir, nextYear+".xlsx")

		if err := os.MkdirAll(nextYearDir, 0o755); err != nil {
			return fmt.Errorf("创建新年度目录: %w", err)
		}

		var f *excelize.File
		if src, err := excelize.OpenFile(prevPath); err == nil {
			f = src
		} else {
			f = excelize.NewFile()
			f.DeleteSheet("Sheet1")
		}
		defer f.Close()

		for _, name := range f.GetSheetList() {
			if !strings.HasPrefix(name, "总分类账-") {
				continue
			}
			account := strings.TrimPrefix(name, "总分类账-")
			node, ok := cfg.Tree[account]
			if !ok {
				continue
			}
			if bal, ok := node.Balances[prevDec]; ok && bal.Final != 0 {
				f.SetCellValue(name, "A1", "上年结转")
				f.SetCellValue(name, "G1", CentsToYuan(bal.Final))
			}
		}

		if err := f.SaveAs(newPath); err != nil {
			return fmt.Errorf("保存 %s: %w", newPath, err)
		}

		fmt.Printf("已生成 %s 跨年结转工作薄\n", nextYear)
		return nil
	},
}
