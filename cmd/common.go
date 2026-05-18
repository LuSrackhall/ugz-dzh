package cmd

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ledger/balance"
	"ledger/voucher"

	"github.com/xuri/excelize/v2"
)

// CentsToYuan converts cents (int64) to yuan display string.
func CentsToYuan(c int64) string {
	if c == 0 {
		return "0"
	}
	return fmt.Sprintf("%.2f", float64(c)/100.0)
}

// CellName returns the Excel cell name for a column and row (1-indexed).
func CellName(col, row int) string {
	name, _ := excelize.CoordinatesToCellName(col, row)
	return name
}

// CollectEntries walks voucherDir and parses all .md files into sorted entries.
func CollectEntries(voucherDir string) ([]voucher.Entry, error) {
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

// ApplyAccountMap applies OCR→standard name mapping to entries' GeneralAccount and DetailAccount.
// Mapping keys are checked against individual field values AND the combined full path.
func ApplyAccountMap(entries []voucher.Entry, accountMap map[string]string) {
	if len(accountMap) == 0 {
		return
	}
	for i := range entries {
		fullPath := entries[i].GeneralAccount
		if entries[i].DetailAccount != "" {
			fullPath += "-" + entries[i].DetailAccount
		}
		if mapped, ok := accountMap[fullPath]; ok {
			parts := strings.SplitN(mapped, "-", 2)
			entries[i].GeneralAccount = parts[0]
			if len(parts) > 1 {
				entries[i].DetailAccount = parts[1]
			} else {
				entries[i].DetailAccount = ""
			}
			continue
		}
		if mapped, ok := accountMap[entries[i].GeneralAccount]; ok {
			entries[i].GeneralAccount = mapped
		}
		if mapped, ok := accountMap[entries[i].DetailAccount]; ok {
			entries[i].DetailAccount = mapped
		}
	}
}

// FilterByMonth filters entries to those whose date starts with the given month prefix.
func FilterByMonth(entries []voucher.Entry, month string) []voucher.Entry {
	prefix := month + "-"
	var filtered []voucher.Entry
	for _, e := range entries {
		if strings.HasPrefix(e.Date, prefix) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// WriteCSV writes ledger entries to a CSV file.
func WriteCSV(dir string, entries []voucher.Entry) error {
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
			CentsToYuan(e.DebitCents),
			CentsToYuan(e.CreditCents),
		})
	}
	return w.Error()
}

// WriteXLSX writes ledger entries to an XLSX file.
func WriteXLSX(dir string, entries []voucher.Entry) error {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "凭证分录"
	f.SetSheetName("Sheet1", sheet)

	headers := []string{"日期", "凭证号", "摘要", "总账科目", "明细科目", "借方金额", "贷方金额"}
	for i, h := range headers {
		f.SetCellValue(sheet, CellName(i+1, 1), h)
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
		f.SetCellValue(sheet, CellName(1, row), e.Date)
		f.SetCellValue(sheet, CellName(2, row), e.VoucherNum)
		f.SetCellValue(sheet, CellName(3, row), e.Summary)
		f.SetCellValue(sheet, CellName(4, row), e.GeneralAccount)
		f.SetCellValue(sheet, CellName(5, row), e.DetailAccount)
		f.SetCellValue(sheet, CellName(6, row), CentsToYuan(e.DebitCents))
		f.SetCellValue(sheet, CellName(7, row), CentsToYuan(e.CreditCents))
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

// WriteBalanceCSV writes balance summaries to a CSV file.
func WriteBalanceCSV(dir string, summaries []balance.LeafSummary) error {
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
			CentsToYuan(s.DebitTotal),
			CentsToYuan(s.CreditTotal),
			CentsToYuan(s.Balance),
			s.Direction,
		})
	}
	return w.Error()
}

// WriteBalanceXLSX writes balance summaries to an XLSX file.
func WriteBalanceXLSX(dir string, summaries []balance.LeafSummary) error {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "科目余额表"
	f.SetSheetName("Sheet1", sheet)

	headers := []string{"科目类别", "科目全路径", "借方合计", "贷方合计", "余额", "方向"}
	for i, h := range headers {
		f.SetCellValue(sheet, CellName(i+1, 1), h)
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
		f.SetCellValue(sheet, CellName(1, row), s.AccountType)
		f.SetCellValue(sheet, CellName(2, row), s.FullPath)
		f.SetCellValue(sheet, CellName(3, row), CentsToYuan(s.DebitTotal))
		f.SetCellValue(sheet, CellName(4, row), CentsToYuan(s.CreditTotal))
		f.SetCellValue(sheet, CellName(5, row), CentsToYuan(s.Balance))
		f.SetCellValue(sheet, CellName(6, row), s.Direction)
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
