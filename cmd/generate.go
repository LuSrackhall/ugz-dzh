package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"ledger/balance"
	"ledger/generator"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(generateCmd)
	generateCmd.Flags().StringP("voucherDir", "v", "", "凭证 .md 文件所在目录（必填）")
	generateCmd.Flags().StringP("output", "o", ".", "输出目录")
	generateCmd.Flags().StringP("month", "m", "", "按月份筛选 (YYYY-MM)")
	generateCmd.Flags().StringP("json", "j", "", "科目余额总览.json 路径")
	generateCmd.Flags().BoolP("force", "f", false, "覆盖已有 xlsx")
	generateCmd.Flags().BoolP("verbose", "V", false, "输出详细日志")
	generateCmd.MarkFlagRequired("voucherDir")
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "生成月度账本",
	Long:  "解析凭证文件，生成 CSV、XLSX 分录表和完整的累计 Excel 工作薄。",
	RunE: func(cmd *cobra.Command, args []string) error {
		voucherDir, _ := cmd.Flags().GetString("voucherDir")
		output, _ := cmd.Flags().GetString("output")
		month, _ := cmd.Flags().GetString("month")
		configJSON, _ := cmd.Flags().GetString("json")
		force, _ := cmd.Flags().GetBool("force")
		verbose, _ := cmd.Flags().GetBool("verbose")

		if verbose {
			fmt.Printf("凭证目录: %s\n输出目录: %s\n月份: %s\n配置: %s\n", voucherDir, output, month, configJSON)
		}

		entries, err := CollectEntries(voucherDir)
		if err != nil {
			return fmt.Errorf("收集凭证: %w", err)
		}

		if verbose {
			fmt.Printf("收集到 %d 条原始分录\n", len(entries))
		}

		if month != "" {
			entries = FilterByMonth(entries, month)
			if verbose {
				fmt.Printf("按月份 %s 筛选后剩余 %d 条分录\n", month, len(entries))
			}
			if len(entries) == 0 {
				fmt.Printf("月份 %s 没有匹配的凭证分录\n", month)
				return nil
			}
		}

		if err := os.MkdirAll(output, 0o755); err != nil {
			return fmt.Errorf("创建输出目录: %w", err)
		}

		if err := WriteCSV(output, entries); err != nil {
			return fmt.Errorf("写入 CSV: %w", err)
		}
		if err := WriteXLSX(output, entries); err != nil {
			return fmt.Errorf("写入 XLSX: %w", err)
		}

		summaries := balance.ComputeSummariesWithParents(entries)
		if err := WriteBalanceCSV(output, summaries); err != nil {
			return fmt.Errorf("写入余额 CSV: %w", err)
		}
		if err := WriteBalanceXLSX(output, summaries); err != nil {
			return fmt.Errorf("写入余额 XLSX: %w", err)
		}

		if configJSON != "" && month != "" {
			xlsxPath := filepath.Join(output, month+".xlsx")
			if !force {
				if _, err := os.Stat(xlsxPath); err == nil {
					return fmt.Errorf("%s 已存在，使用 -f 覆盖已有 xlsx", xlsxPath)
				}
			}
			if err := generator.GenerateWorkbook(configJSON, voucherDir, month, output); err != nil {
				return fmt.Errorf("生成工作薄: %w", err)
			}
			fmt.Printf("已生成 %s 工作薄\n", month)
		}

		fmt.Printf("已输出 %d 条分录到 %s\n", len(entries), output)
		return nil
	},
}
