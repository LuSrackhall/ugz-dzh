package generator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/xuri/excelize/v2"

	"ledger/voucher"
)

// journalHeaders 日记账列标题（《会计基础工作规范》第三十五条：日期/凭证编号/摘要/对方科目/金额/余额）
var journalHeaders = []string{"日期", "凭证字号", "摘  要", "对方科目", "借  方", "贷  方", "余  额"}

// WriteJournals 生成现金/银行日记账（设计专家审查 Change 9）：
//   - 现金日记账：库存现金 逐日逐笔、日结月结
//   - 银行存款日记账：银行存款（含子科目）同上
//
// 数据全部来自当月 entries（日期/凭证号/摘要/金额），对方科目取同凭证其他分录的总账科目。
func (wb *Workbook) WriteJournals(entries []voucher.Entry, initials map[string]int64) error {
	if err := wb.writeOneJournal("现金日记账", "库存现金", entries, initials); err != nil {
		return err
	}
	if err := wb.writeOneJournal("银行存款日记账", "银行存款", entries, initials); err != nil {
		return err
	}
	return nil
}

// sumInitialsFor 汇总某科目（含子科目）的期初。
func sumInitialsFor(initials map[string]int64, general string) int64 {
	var total int64
	for k, v := range initials {
		if k == general || strings.HasPrefix(k, general+"-") {
			total += v
		}
	}
	return total
}

func (wb *Workbook) writeOneJournal(sheetName, general string, entries []voucher.Entry, initials map[string]int64) error {
	// 筛选当月分录（含子科目）
	var rows []voucher.Entry
	for _, e := range entries {
		if e.GeneralAccount == general || strings.HasPrefix(e.GeneralAccount, general+"-") {
			rows = append(rows, e)
		}
	}
	if len(rows) == 0 {
		return nil // 无分录不建 sheet
	}
	// 按日期+凭证号排序（序时）
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Date != rows[j].Date {
			return rows[i].Date < rows[j].Date
		}
		if rows[i].VoucherNum != rows[j].VoucherNum {
			return rows[i].VoucherNum < rows[j].VoucherNum
		}
		return i < j
	})

	idx, err := wb.File.NewSheet(sheetName)
	if err != nil {
		return fmt.Errorf("创建 %s: %w", sheetName, err)
	}
	wb.File.SetActiveSheet(idx)

	// 标题
	wb.File.SetCellValue(sheetName, "A1", wb.Month+" "+sheetName)
	wb.File.MergeCell(sheetName, "A1", "G1")
	titleStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 14},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	wb.File.SetCellStyle(sheetName, "A1", "G1", titleStyle)
	wb.File.SetRowHeight(sheetName, 1, 22)

	for i, h := range journalHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		wb.File.SetCellValue(sheetName, cell, h)
	}
	headerStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 10},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#D9E1F2"}, Pattern: 1},
		Border:    []excelize.Border{{Type: "bottom", Color: "#808080", Style: 1}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	wb.File.SetCellStyle(sheetName, "A2", "G2", headerStyle)
	wb.File.SetColWidth(sheetName, "A", "A", 12)
	wb.File.SetColWidth(sheetName, "B", "B", 10)
	wb.File.SetColWidth(sheetName, "C", "C", 32)
	wb.File.SetColWidth(sheetName, "D", "D", 22)
	for _, col := range []string{"E", "F", "G"} {
		wb.File.SetColWidth(sheetName, col, col, 14)
	}

	row := 3
	// 期初行
	opening := sumInitialsFor(initials, general)
	balance := opening
	if opening != 0 {
		wb.File.SetCellValue(sheetName, cellName(1, row), "期初余额")
		wb.File.SetCellValue(sheetName, cellName(7, row), centsToYuan(balance))
		wb.setMoneyStyle(sheetName, row, 7)
		row++
	}

	// 逐笔登记 + 日结
	var dayDebit, dayCredit int64
	var dayTotalDebit, dayTotalCredit int64
	prevDate := ""
	for i, e := range rows {
		date := e.Date
		// 日期切换 → 前一日日结
		if prevDate != "" && date != prevDate {
			row = wb.writeJournalDayTotal(sheetName, row, prevDate, dayDebit, dayCredit, balance)
			dayTotalDebit += dayDebit
			dayTotalCredit += dayCredit
			dayDebit, dayCredit = 0, 0
		}
		// 明细科目并入摘要行前标记（银行存款子账户）
		detail := ""
		if e.GeneralAccount != general {
			detail = "(" + strings.TrimPrefix(e.GeneralAccount, general+"-") + ")"
		}
		wb.File.SetCellValue(sheetName, cellName(1, row), date)
		wb.File.SetCellValue(sheetName, cellName(2, row), fmt.Sprintf("记%d", e.VoucherNum))
		wb.File.SetCellValue(sheetName, cellName(3, row), e.Summary+detail)
		wb.File.SetCellValue(sheetName, cellName(4, row), counterpartAccounts(entries, e.Date, e.VoucherNum, general))
		if e.DebitCents != 0 {
			wb.File.SetCellValue(sheetName, cellName(5, row), centsToYuan(e.DebitCents))
			wb.setMoneyStyle(sheetName, row, 5)
		}
		if e.CreditCents != 0 {
			wb.File.SetCellValue(sheetName, cellName(6, row), centsToYuan(e.CreditCents))
			wb.setMoneyStyle(sheetName, row, 6)
		}
		balance += e.DebitCents - e.CreditCents
		wb.File.SetCellValue(sheetName, cellName(7, row), centsToYuan(balance))
		wb.setMoneyStyle(sheetName, row, 7)
		dayDebit += e.DebitCents
		dayCredit += e.CreditCents
		prevDate = date
		row++
		_ = i
	}
	// 最后一日日结
	if prevDate != "" {
		row = wb.writeJournalDayTotal(sheetName, row, prevDate, dayDebit, dayCredit, balance)
		dayTotalDebit += dayDebit
		dayTotalCredit += dayCredit
	}

	// 月结：本月合计 / 本年累计 / 期末结存
	totalStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 10},
		Border:    []excelize.Border{{Type: "top", Color: "#808080", Style: 1}, {Type: "bottom", Color: "#808080", Style: 2}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	wb.File.SetCellValue(sheetName, cellName(1, row), "本月合计")
	wb.File.SetCellValue(sheetName, cellName(5, row), centsToYuan(dayTotalDebit))
	wb.File.SetCellValue(sheetName, cellName(6, row), centsToYuan(dayTotalCredit))
	wb.File.SetCellValue(sheetName, cellName(7, row), centsToYuan(balance))
	wb.File.SetCellStyle(sheetName, cellName(1, row), cellName(7, row), totalStyle)
	for _, c := range []int{5, 6, 7} {
		wb.setMoneyStyle(sheetName, row, c)
	}
	row++
	wb.File.SetCellValue(sheetName, cellName(1, row), "期末结存")
	wb.File.SetCellValue(sheetName, cellName(7, row), centsToYuan(balance))
	wb.File.SetCellStyle(sheetName, cellName(1, row), cellName(7, row), totalStyle)
	wb.setMoneyStyle(sheetName, row, 7)

	// 现金余额为负提示
	if general == "库存现金" && balance < 0 {
		fmt.Printf("警告: 现金日记账期末余额为负（%.2f 元），现金不应出现贷方余额，请核对凭证\n", float64(balance)/100)
	}

	return nil
}

// writeJournalDayTotal 写"本日合计"行（日结），返回下一行号。
func (wb *Workbook) writeJournalDayTotal(sheet string, row int, date string, dayDebit, dayCredit, balance int64) int {
	dayStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 10},
		Border:    []excelize.Border{{Type: "bottom", Color: "#A6A6A6", Style: 1}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	wb.File.SetCellValue(sheet, cellName(1, row), date)
	wb.File.SetCellValue(sheet, cellName(3, row), "本日合计")
	if dayDebit != 0 {
		wb.File.SetCellValue(sheet, cellName(5, row), centsToYuan(dayDebit))
		wb.setMoneyStyle(sheet, row, 5)
	}
	if dayCredit != 0 {
		wb.File.SetCellValue(sheet, cellName(6, row), centsToYuan(dayCredit))
		wb.setMoneyStyle(sheet, row, 6)
	}
	wb.File.SetCellValue(sheet, cellName(7, row), centsToYuan(balance))
	wb.setMoneyStyle(sheet, row, 7)
	wb.File.SetCellStyle(sheet, cellName(1, row), cellName(7, row), dayStyle)
	return row + 1
}

// counterpartAccounts 对方科目：同凭证其他分录的总账科目去重（《规范》对应科目栏只填总账科目）。
func counterpartAccounts(entries []voucher.Entry, date string, num int, self string) string {
	seen := make(map[string]bool)
	var list []string
	for _, e := range entries {
		if e.Date == date && e.VoucherNum == num && e.GeneralAccount != self {
			if !seen[e.GeneralAccount] {
				seen[e.GeneralAccount] = true
				list = append(list, e.GeneralAccount)
			}
		}
	}
	if len(list) == 0 {
		return ""
	}
	if len(list) > 3 {
		return strings.Join(list[:3], "、") + "等"
	}
	return strings.Join(list, "、")
}
