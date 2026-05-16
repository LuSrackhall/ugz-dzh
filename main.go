package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ledger/balance"
	"ledger/voucher"

	"github.com/xuri/excelize/v2"
)

func main() {
	voucherDir := flag.String("voucherDir", "", "凭证 .md 文件所在目录（必填）")
	output := flag.String("output", ".", "输出目录（默认当前目录）")
	month := flag.String("month", "", "按月份筛选 (YYYY-MM)，留空则输出全部")
	flag.Parse()

	if *voucherDir == "" {
		fmt.Fprintln(os.Stderr, "错误: -voucherDir 参数为必填项")
		flag.Usage()
		os.Exit(1)
	}

	entries, err := collectEntries(*voucherDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}

	if *month != "" {
		entries = filterByMonth(entries, *month)
		if len(entries) == 0 {
			fmt.Fprintf(os.Stderr, "警告: 月份 %s 没有匹配的凭证分录\n", *month)
			os.Exit(0)
		}
	}

	if err := os.MkdirAll(*output, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "错误: 创建输出目录失败: %v\n", err)
		os.Exit(1)
	}

	if err := writeCSV(*output, entries); err != nil {
		fmt.Fprintf(os.Stderr, "错误: 写入 CSV 失败: %v\n", err)
		os.Exit(1)
	}

	if err := writeXLSX(*output, entries); err != nil {
		fmt.Fprintf(os.Stderr, "错误: 写入 XLSX 失败: %v\n", err)
		os.Exit(1)
	}

	summaries := balance.ComputeSummariesWithParents(entries)
	if err := writeBalanceCSV(*output, summaries); err != nil {
		fmt.Fprintf(os.Stderr, "错误: 写入余额 CSV 失败: %v\n", err)
		os.Exit(1)
	}
	if err := writeBalanceXLSX(*output, summaries); err != nil {
		fmt.Fprintf(os.Stderr, "错误: 写入余额 XLSX 失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("已输出 %d 条分录到 %s\n", len(entries), *output)
}

func collectEntries(voucherDir string) ([]voucher.Entry, error) {
	var all []voucher.Entry
	err := filepath.WalkDir(voucherDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		entries, err := voucher.ParseFile(path)
		if err != nil {
			return fmt.Errorf("解析 %s: %w", path, err)
		}
		all = append(all, entries...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].Date != all[j].Date {
			return all[i].Date < all[j].Date
		}
		return all[i].VoucherNum < all[j].VoucherNum
	})
	return all, nil
}

func filterByMonth(entries []voucher.Entry, month string) []voucher.Entry {
	prefix := month + "-"
	var filtered []voucher.Entry
	for _, e := range entries {
		if strings.HasPrefix(e.Date, prefix) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

func writeCSV(dir string, entries []voucher.Entry) error {
	path := filepath.Join(dir, "ledger.csv")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	w.Write([]string{"日期", "凭证号", "摘要", "总账科目", "明细科目", "借方金额", "贷方金额"})
	for _, e := range entries {
		w.Write([]string{
			e.Date,
			fmt.Sprintf("%d", e.VoucherNum),
			e.Summary,
			e.GeneralAccount,
			e.DetailAccount,
			centsToYuan(e.DebitCents),
			centsToYuan(e.CreditCents),
		})
	}
	return w.Error()
}

func writeXLSX(dir string, entries []voucher.Entry) error {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Sheet1"
	f.SetSheetName("Sheet1", "凭证分录")
	sheet = "凭证分录"

	headers := []string{"日期", "凭证号", "摘要", "总账科目", "明细科目", "借方金额", "贷方金额"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 11},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#D9E1F2"}, Pattern: 1},
		Border: []excelize.Border{
			{Type: "bottom", Color: "#B0B0B0", Style: 1},
		},
	})
	f.SetCellStyle(sheet, "A1", "G1", headerStyle)

	for i, e := range entries {
		row := i + 2
		f.SetCellValue(sheet, cellName(1, row), e.Date)
		f.SetCellValue(sheet, cellName(2, row), e.VoucherNum)
		f.SetCellValue(sheet, cellName(3, row), e.Summary)
		f.SetCellValue(sheet, cellName(4, row), e.GeneralAccount)
		f.SetCellValue(sheet, cellName(5, row), e.DetailAccount)
		f.SetCellValue(sheet, cellName(6, row), centsToYuan(e.DebitCents))
		f.SetCellValue(sheet, cellName(7, row), centsToYuan(e.CreditCents))
	}

	f.SetColWidth(sheet, "A", "A", 12)
	f.SetColWidth(sheet, "B", "B", 8)
	f.SetColWidth(sheet, "C", "C", 40)
	f.SetColWidth(sheet, "D", "D", 14)
	f.SetColWidth(sheet, "E", "E", 14)
	f.SetColWidth(sheet, "F", "F", 14)
	f.SetColWidth(sheet, "G", "G", 14)

	path := filepath.Join(dir, "ledger.xlsx")
	return f.SaveAs(path)
}

func cellName(col, row int) string {
	name, _ := excelize.CoordinatesToCellName(col, row)
	return name
}

func centsToYuan(c int64) string {
	if c == 0 {
		return "0"
	}
	yuan := float64(c) / 100.0
	return fmt.Sprintf("%.2f", yuan)
}

func writeBalanceCSV(dir string, summaries []balance.LeafSummary) error {
	path := filepath.Join(dir, "balance.csv")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	w.Write([]string{"科目类别", "科目全路径", "借方合计", "贷方合计", "余额", "方向"})
	for _, s := range summaries {
		w.Write([]string{
			s.AccountType,
			s.FullPath,
			centsToYuan(s.DebitTotal),
			centsToYuan(s.CreditTotal),
			centsToYuan(s.Balance),
			s.Direction,
		})
	}
	return w.Error()
}

func writeBalanceXLSX(dir string, summaries []balance.LeafSummary) error {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "科目余额表"
	f.SetSheetName("Sheet1", sheet)

	headers := []string{"科目类别", "科目全路径", "借方合计", "贷方合计", "余额", "方向"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 11},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#E2EFDA"}, Pattern: 1},
		Border: []excelize.Border{
			{Type: "bottom", Color: "#B0B0B0", Style: 1},
		},
	})
	f.SetCellStyle(sheet, "A1", "F1", headerStyle)

	for i, s := range summaries {
		row := i + 2
		f.SetCellValue(sheet, cellName(1, row), s.AccountType)
		f.SetCellValue(sheet, cellName(2, row), s.FullPath)
		f.SetCellValue(sheet, cellName(3, row), centsToYuan(s.DebitTotal))
		f.SetCellValue(sheet, cellName(4, row), centsToYuan(s.CreditTotal))
		f.SetCellValue(sheet, cellName(5, row), centsToYuan(s.Balance))
		f.SetCellValue(sheet, cellName(6, row), s.Direction)
	}

	f.SetColWidth(sheet, "A", "A", 10)
	f.SetColWidth(sheet, "B", "B", 20)
	f.SetColWidth(sheet, "C", "C", 14)
	f.SetColWidth(sheet, "D", "D", 14)
	f.SetColWidth(sheet, "E", "E", 14)
	f.SetColWidth(sheet, "F", "F", 6)

	path := filepath.Join(dir, "balance.xlsx")
	return f.SaveAs(path)
}
