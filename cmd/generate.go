package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ledger/balance"
	"ledger/generator"
	"ledger/voucher"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(generateCmd)
	generateCmd.Flags().StringP("voucherDir", "v", "", "凭证 .md 文件所在目录（必填）")
	generateCmd.Flags().StringP("output", "o", ".", "输出根目录")
	generateCmd.Flags().BoolP("force", "f", false, "覆盖已有 xlsx")
	generateCmd.Flags().BoolP("verbose", "V", false, "输出详细日志")
	generateCmd.MarkFlagRequired("voucherDir")
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "生成月度账本",
	Long:  "解析凭证文件（必须全部来自同一年同一月），自动推导年份和月份，生成 CSV、XLSX 分录表和完整的累计 Excel 工作薄。",
	RunE: func(cmd *cobra.Command, args []string) error {
		voucherDir, _ := cmd.Flags().GetString("voucherDir")
		output, _ := cmd.Flags().GetString("output")
		force, _ := cmd.Flags().GetBool("force")
		verbose, _ := cmd.Flags().GetBool("verbose")

		// 收集所有凭证
		entries, err := CollectEntries(voucherDir)
		if err != nil {
			return fmt.Errorf("收集凭证: %w", err)
		}
		if len(entries) == 0 {
			return fmt.Errorf("目录 %s 中没有解析到任何凭证分录", voucherDir)
		}

		// 同年同月校验 + 推导年份月份
		year, month, err := validateSameMonth(entries)
		if err != nil {
			return err
		}

		// 推导 JSON 路径: {output}/{year}/{year}.json
		yearDir := filepath.Join(output, year)
		configJSON := filepath.Join(yearDir, year+".json")

		if verbose {
			fmt.Printf("凭证目录: %s\n输出目录: %s/%s/\n月份: %s\n配置: %s\n", voucherDir, output, year, month, configJSON)
		}

		// 加载配置并应用映射
		cfg, err := balance.LoadConfig(configJSON)
		if err != nil {
			return fmt.Errorf("加载配置 %s: %w", configJSON, err)
		}
		if len(cfg.Settings.AccountMap) > 0 {
			ApplyAccountMap(entries, cfg.Settings.AccountMap)
			if verbose {
				fmt.Printf("已应用 %d 条科目名称映射\n", len(cfg.Settings.AccountMap))
			}
		}

		if verbose {
			fmt.Printf("收集到 %d 条原始分录\n", len(entries))
		}

		// 筛选当月分录
		entries = FilterByMonth(entries, month)
		if verbose {
			fmt.Printf("按月份 %s 筛选后剩余 %d 条分录\n", month, len(entries))
		}
		if len(entries) == 0 {
			return fmt.Errorf("月份 %s 没有匹配的凭证分录", month)
		}

		if err := os.MkdirAll(yearDir, 0o755); err != nil {
			return fmt.Errorf("创建输出目录: %w", err)
		}

		// 写入 CSV/XLSX 分录汇总
		if err := WriteCSV(yearDir, entries); err != nil {
			return fmt.Errorf("写入 CSV: %w", err)
		}
		if err := WriteXLSX(yearDir, entries); err != nil {
			return fmt.Errorf("写入 XLSX: %w", err)
		}

		summaries := balance.ComputeSummariesWithParents(entries)
		if err := WriteBalanceCSV(yearDir, summaries); err != nil {
			return fmt.Errorf("写入余额 CSV: %w", err)
		}
		if err := WriteBalanceXLSX(yearDir, summaries); err != nil {
			return fmt.Errorf("写入余额 XLSX: %w", err)
		}

		// 生成月度累计工作薄
		xlsxPath := filepath.Join(yearDir, month+".xlsx")
		if !force {
			if _, err := os.Stat(xlsxPath); err == nil {
				return fmt.Errorf("%s 已存在，使用 -f 覆盖已有 xlsx", xlsxPath)
			}
		}
		if err := generator.GenerateWorkbook(configJSON, voucherDir, month, yearDir); err != nil {
			return fmt.Errorf("生成工作薄: %w", err)
		}

		fmt.Printf("已生成 %s/%s 工作薄，共 %d 条分录\n", year, month, len(entries))
		return nil
	},
}

// validateSameMonth 校验所有凭证是否来自同一年同一月，返回推导的年份和月份。
func validateSameMonth(entries []voucher.Entry) (year, month string, err error) {
	if len(entries) == 0 {
		return "", "", fmt.Errorf("没有分录可校验")
	}

	expected := ""
	for _, e := range entries {
		if len(e.Date) < 7 {
			return "", "", fmt.Errorf("分录日期格式无效: %q", e.Date)
		}
		m := e.Date[:7]
		if expected == "" {
			expected = m
		} else if m != expected {
			return "", "", fmt.Errorf("凭证目录中包含不同月份的分录: %s 与 %s。请确保所有凭证来自同一年同一月", expected, m)
		}
	}

	year = strings.Split(expected, "-")[0]
	month = expected
	return year, month, nil
}
