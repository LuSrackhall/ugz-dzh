package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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

		// 自动并入自动结转凭证（output/{year}/closing/，Change 10——系统生成的结转凭证，不污染手工目录）
		closingDir := filepath.Join(output, year, "closing")
		if closingFiles, _ := filepath.Glob(filepath.Join(closingDir, "*.md")); len(closingFiles) > 0 {
			for _, f := range closingFiles {
				ces, err := voucher.ParseFile(f)
				if err != nil {
					return fmt.Errorf("解析自动结转凭证 %s: %w", f, err)
				}
				entries = append(entries, ces...)
			}
			fmt.Printf("已并入 %d 张自动结转凭证（%s）\n", len(closingFiles), closingDir)
		}

		// 借贷平衡校验（审计 H2，CLI 内建安全：不平拒绝生成；含自动结转凭证）
		warnings, err := voucher.ValidateVoucherBalance(entries)
		if err != nil {
			return fmt.Errorf("凭证借贷平衡校验失败: %w", err)
		}
		for _, w := range warnings {
			fmt.Printf("提示: %s\n", w)
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
		if force {
			// 级联删除当月及之后所有月份的账本 xlsx（当月旧文件必须删除，否则 NewWorkbook 会加载旧当月文件
			// 并把旧版当月期末当作"上月期末"，导致 -f 重建期初被污染）
			// 仅删除账本文件（YYYY-MM.xlsx），绝不误删 ledger.xlsx / balance.xlsx 汇总文件
			entries, err := os.ReadDir(yearDir)
			if err == nil {
				for _, entry := range entries {
					if entry.IsDir() {
						continue
					}
					name := entry.Name()
					if isMonthXlsxName(name) && strings.TrimSuffix(name, ".xlsx") >= month {
						path := filepath.Join(yearDir, name)
						if err := os.Remove(path); err != nil {
							return fmt.Errorf("删除 %s: %w", path, err)
						}
						if verbose {
							fmt.Printf("已删除: %s\n", path)
						}
					}
				}
			}
			// 同步清理 print/ 子目录中当月及之后月份的打印版 xlsx
			printDir := filepath.Join(yearDir, "print")
			if pentries, err := os.ReadDir(printDir); err == nil {
				for _, entry := range pentries {
					if entry.IsDir() {
						continue
					}
					name := entry.Name()
					if isMonthXlsxName(name) && strings.TrimSuffix(name, ".xlsx") >= month {
						path := filepath.Join(printDir, name)
						if err := os.Remove(path); err != nil {
							return fmt.Errorf("删除 %s: %w", path, err)
						}
						if verbose {
							fmt.Printf("已删除: %s\n", path)
						}
					}
				}
			}
		} else {
			if _, err := os.Stat(xlsxPath); err == nil {
				// M3 幂等保护：已含当月"本月合计"行视为已生成，要求 -f；
				// year-close 预生成的空工作薄（无本月合计行）放行直接生成
				if done, err := generator.AlreadyGenerated(xlsxPath); err == nil && done {
					return fmt.Errorf("%s 已生成过（含本月合计行），使用 -f 覆盖重建", xlsxPath)
				}
			}
		}
		if err := generator.GenerateWorkbook(configJSON, month, yearDir, entries); err != nil {
			return fmt.Errorf("生成工作薄: %w", err)
		}

		// 生成打印版位格 xlsx（失败仅告警，不影响已落盘的查看版）
		printPath := filepath.Join(yearDir, "print", month+".xlsx")
		if err := generator.TransformToPrint(xlsxPath, printPath); err != nil {
			fmt.Fprintf(os.Stderr, "警告: 生成打印版失败（查看版已成功）: %v\n", err)
		} else if verbose {
			fmt.Printf("已生成打印版: %s\n", printPath)
		}

		fmt.Printf("已生成 %s/%s 工作薄，共 %d 条分录\n", year, month, len(entries))
		return nil
	},
}

// isMonthXlsxName 判断文件名是否为账本文件（YYYY-MM.xlsx），避免 -f 级联删除误伤 ledger.xlsx / balance.xlsx 汇总文件。
func isMonthXlsxName(name string) bool {
	trimmed := strings.TrimSuffix(name, ".xlsx")
	if len(trimmed) != 7 || trimmed[4] != '-' {
		return false
	}
	if _, err := strconv.Atoi(trimmed[:4]); err != nil {
		return false
	}
	if _, err := strconv.Atoi(trimmed[5:]); err != nil {
		return false
	}
	return true
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
