package generator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/xuri/excelize/v2"

	"ledger/balance"
	"ledger/voucher"
)

// WriteReportSheets 生成报表套件（Change 11）：
//   - 资产负债表：资产/费用→左，负债/权益/收入→右，金额=|期末|（方向相反显负），合计+差额
//   - 收支结余表：收入类累计 - 费用类累计 = 本年收益
//   - 科目汇总表：本月各科目借/贷发生额（试算平衡）
//   - 凭证序时簿：按日期/凭证号列示每张凭证借贷合计
func (wb *Workbook) WriteReportSheets(entries []voucher.Entry, activity map[string]Activity, initials map[string]int64, ytdDebit, ytdCredit map[string]int64) error {
	if err := wb.writeBalanceSheet(initials, activity); err != nil {
		return err
	}
	if err := wb.writeIncomeStatement(ytdDebit, ytdCredit, activity); err != nil {
		return err
	}
	if err := wb.writeSubjectSummary(activity); err != nil {
		return err
	}
	if err := wb.writeVoucherRegister(entries); err != nil {
		return err
	}
	return nil
}

// reportStyle 报表通用样式（创建前清除同名旧 sheet——跨月残留，验收发现同 Change 9 日记账缺陷）。
func (wb *Workbook) reportSheet(sheet, title string, headers []string, widths []float64) (*excelize.File, int, error) {
	for _, s := range wb.File.GetSheetList() {
		if s == sheet {
			wb.File.DeleteSheet(s)
		}
	}
	idx, err := wb.File.NewSheet(sheet)
	if err != nil {
		return nil, 0, fmt.Errorf("创建 %s: %w", sheet, err)
	}
	wb.File.SetActiveSheet(idx)
	wb.File.SetCellValue(sheet, "A1", wb.Month+" "+title)
	lastCol := string(rune('A' + len(headers) - 1))
	wb.File.MergeCell(sheet, "A1", lastCol+"1")
	titleStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 14},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	wb.File.SetCellStyle(sheet, "A1", lastCol+"1", titleStyle)
	wb.File.SetRowHeight(sheet, 1, 22)
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		wb.File.SetCellValue(sheet, cell, h)
	}
	headerStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 10},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#D9E1F2"}, Pattern: 1},
		Border:    []excelize.Border{{Type: "bottom", Color: "#808080", Style: 1}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	wb.File.SetCellStyle(sheet, "A2", lastCol+"2", headerStyle)
	for i, w := range widths {
		col, _ := excelize.ColumnNumberToName(i + 1)
		wb.File.SetColWidth(sheet, col, col, w)
	}
	return wb.File, 3, nil
}

// writeBalanceSheet 资产负债表（期末数）。
func (wb *Workbook) writeBalanceSheet(initials map[string]int64, activity map[string]Activity) error {
	sheet := "资产负债表"
	// 期末余额（含无活动科目）
	type repRow struct {
		account string
		gen     string
		final   int64
	}
	var rows []repRow
	for k, init := range initials {
		act := activity[k]
		final := init + act.Debit - act.Credit
		gen := k
		if idx := strings.IndexByte(gen, '-'); idx > 0 {
			gen = gen[:idx]
		}
		rows = append(rows, repRow{account: k, gen: gen, final: final})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].account < rows[j].account })

	_, startRow, err := wb.reportSheet(sheet, "资产负债表（期末）", []string{"资产", "金额", "负债及所有者权益", "金额"}, []float64{32, 16, 32, 16})
	if err != nil {
		return err
	}
	row := startRow
	var leftTotal, rightTotal int64
	for _, r := range rows {
		t, ok := balance.AccountTypeOf(r.gen)
		left := ok && (t == "资产" || t == "费用")
		if !ok {
			continue // 未分类科目不列入资产负债表
		}
		amt := r.final
		if amt < 0 {
			amt = -amt
		}
		if left {
			wb.File.SetCellValue(sheet, cellName(1, row), r.account)
			if r.final < 0 {
				amt = -amt // 方向相反显负（红字科目）
			}
			wb.File.SetCellValue(sheet, cellName(2, row), centsToYuan(amt))
			wb.setMoneyStyle(sheet, row, 2)
			leftTotal += amt
		} else {
			wb.File.SetCellValue(sheet, cellName(3, row), r.account)
			if r.final > 0 {
				amt = -amt
			}
			wb.File.SetCellValue(sheet, cellName(4, row), centsToYuan(amt))
			wb.setMoneyStyle(sheet, row, 4)
			rightTotal += amt
		}
		row++
	}
	// 合计
	wb.File.SetCellValue(sheet, cellName(1, row), "资产总计")
	wb.File.SetCellValue(sheet, cellName(2, row), centsToYuan(leftTotal))
	wb.File.SetCellValue(sheet, cellName(3, row), "负债及所有者权益总计")
	wb.File.SetCellValue(sheet, cellName(4, row), centsToYuan(rightTotal))
	totalStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:   &excelize.Font{Bold: true, Size: 10},
		Border: []excelize.Border{{Type: "top", Color: "#808080", Style: 2}},
	})
	wb.File.SetCellStyle(sheet, cellName(1, row), cellName(4, row), totalStyle)
	wb.setMoneyStyle(sheet, row, 2)
	wb.setMoneyStyle(sheet, row, 4)
	if leftTotal != rightTotal {
		wb.File.SetCellValue(sheet, cellName(1, row+1), fmt.Sprintf("差额（左-右）: %.2f 元——请检查红字/异常科目", float64(leftTotal-rightTotal)/100))
	}
	return nil
}

// writeIncomeStatement 收支结余表（本年累计口径）。
func (wb *Workbook) writeIncomeStatement(ytdDebit, ytdCredit map[string]int64, activity map[string]Activity) error {
	sheet := "收支结余表"
	// 收入类：贷方累计；费用类：借方累计
	type repRow struct {
		name  string
		gen   string
		side  string // 收入/支出
		cents int64
	}
	var rows []repRow
	seen := make(map[string]bool)
	add := func(k string, side string) {
		if seen[k] {
			return
		}
		seen[k] = true
		gen := k
		if idx := strings.IndexByte(gen, '-'); idx > 0 {
			gen = gen[:idx]
		}
		t, ok := balance.AccountTypeOf(gen)
		if !ok {
			return
		}
		if t == "收入" || t == "费用" {
			var v int64
			if t == "收入" {
				v = ytdCredit[k] + activity[k].Credit
			} else {
				v = ytdDebit[k] + activity[k].Debit
			}
			if v != 0 {
				rows = append(rows, repRow{name: k, gen: gen, side: side, cents: v})
			}
		}
	}
	for k := range activity {
		add(k, "")
	}
	for k := range ytdDebit {
		add(k, "")
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })

	_, startRow, err := wb.reportSheet(sheet, "收支结余表（本年累计）", []string{"收入项目", "金额", "支出项目", "金额"}, []float64{32, 16, 32, 16})
	if err != nil {
		return err
	}
	row := startRow
	var incomeTotal, expenseTotal int64
	for _, r := range rows {
		t, _ := balance.AccountTypeOf(r.gen)
		if t == "收入" {
			wb.File.SetCellValue(sheet, cellName(1, row), r.name)
			wb.File.SetCellValue(sheet, cellName(2, row), centsToYuan(r.cents))
			wb.setMoneyStyle(sheet, row, 2)
			incomeTotal += r.cents
		} else {
			wb.File.SetCellValue(sheet, cellName(3, row), r.name)
			wb.File.SetCellValue(sheet, cellName(4, row), centsToYuan(r.cents))
			wb.setMoneyStyle(sheet, row, 4)
			expenseTotal += r.cents
		}
		row++
	}
	wb.File.SetCellValue(sheet, cellName(1, row), "收入合计")
	wb.File.SetCellValue(sheet, cellName(2, row), centsToYuan(incomeTotal))
	wb.File.SetCellValue(sheet, cellName(3, row), "支出合计")
	wb.File.SetCellValue(sheet, cellName(4, row), centsToYuan(expenseTotal))
	totalStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:   &excelize.Font{Bold: true, Size: 10},
		Border: []excelize.Border{{Type: "top", Color: "#808080", Style: 2}},
	})
	wb.File.SetCellStyle(sheet, cellName(1, row), cellName(4, row), totalStyle)
	wb.setMoneyStyle(sheet, row, 2)
	wb.setMoneyStyle(sheet, row, 4)
	row++
	wb.File.SetCellValue(sheet, cellName(1, row), "本年收益（收入-支出）")
	wb.File.SetCellValue(sheet, cellName(2, row), centsToYuan(incomeTotal-expenseTotal))
	wb.setMoneyStyle(sheet, row, 2)
	wb.File.SetCellStyle(sheet, cellName(1, row), cellName(4, row), totalStyle)
	return nil
}

// writeSubjectSummary 科目汇总表（本月发生额试算平衡）。
func (wb *Workbook) writeSubjectSummary(activity map[string]Activity) error {
	sheet := "科目汇总表"
	_, startRow, err := wb.reportSheet(sheet, "科目汇总表（本月发生额）", []string{"科目", "借方发生额", "贷方发生额"}, []float64{40, 16, 16})
	if err != nil {
		return err
	}
	accounts := make([]string, 0, len(activity))
	for k := range activity {
		accounts = append(accounts, k)
	}
	sort.Strings(accounts)
	row := startRow
	var totalDebit, totalCredit int64
	for _, k := range accounts {
		act := activity[k]
		if act.Debit == 0 && act.Credit == 0 {
			continue
		}
		wb.File.SetCellValue(sheet, cellName(1, row), k)
		if act.Debit != 0 {
			wb.File.SetCellValue(sheet, cellName(2, row), centsToYuan(act.Debit))
			wb.setMoneyStyle(sheet, row, 2)
		}
		if act.Credit != 0 {
			wb.File.SetCellValue(sheet, cellName(3, row), centsToYuan(act.Credit))
			wb.setMoneyStyle(sheet, row, 3)
		}
		totalDebit += act.Debit
		totalCredit += act.Credit
		row++
	}
	wb.File.SetCellValue(sheet, cellName(1, row), "合计")
	wb.File.SetCellValue(sheet, cellName(2, row), centsToYuan(totalDebit))
	wb.File.SetCellValue(sheet, cellName(3, row), centsToYuan(totalCredit))
	totalStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:   &excelize.Font{Bold: true, Size: 10},
		Border: []excelize.Border{{Type: "top", Color: "#808080", Style: 2}},
	})
	wb.File.SetCellStyle(sheet, cellName(1, row), cellName(3, row), totalStyle)
	wb.setMoneyStyle(sheet, row, 2)
	wb.setMoneyStyle(sheet, row, 3)
	return nil
}

// writeVoucherRegister 凭证序时簿（每凭证一行，借贷合计）。
func (wb *Workbook) writeVoucherRegister(entries []voucher.Entry) error {
	sheet := "凭证序时簿"
	_, startRow, err := wb.reportSheet(sheet, "凭证序时簿", []string{"日期", "凭证字号", "摘要", "借方金额", "贷方金额"}, []float64{12, 10, 44, 16, 16})
	if err != nil {
		return err
	}
	type vk struct {
		date string
		num  int
	}
	order := make([]vk, 0)
	groups := make(map[vk][]voucher.Entry)
	for _, e := range entries {
		key := vk{date: e.Date, num: e.VoucherNum}
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], e)
	}
	sort.Slice(order, func(i, j int) bool {
		if order[i].date != order[j].date {
			return order[i].date < order[j].date
		}
		return order[i].num < order[j].num
	})
	row := startRow
	var totalDebit, totalCredit int64
	for _, key := range order {
		es := groups[key]
		var db, cr int64
		summary := ""
		for _, e := range es {
			db += e.DebitCents
			cr += e.CreditCents
			if summary == "" {
				summary = e.Summary
			}
		}
		wb.File.SetCellValue(sheet, cellName(1, row), key.date)
		wb.File.SetCellValue(sheet, cellName(2, row), fmt.Sprintf("记%d", key.num))
		wb.File.SetCellValue(sheet, cellName(3, row), summary)
		if db != 0 {
			wb.File.SetCellValue(sheet, cellName(4, row), centsToYuan(db))
			wb.setMoneyStyle(sheet, row, 4)
		}
		if cr != 0 {
			wb.File.SetCellValue(sheet, cellName(5, row), centsToYuan(cr))
			wb.setMoneyStyle(sheet, row, 5)
		}
		totalDebit += db
		totalCredit += cr
		row++
	}
	wb.File.SetCellValue(sheet, cellName(3, row), "合计")
	wb.File.SetCellValue(sheet, cellName(4, row), centsToYuan(totalDebit))
	wb.File.SetCellValue(sheet, cellName(5, row), centsToYuan(totalCredit))
	totalStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:   &excelize.Font{Bold: true, Size: 10},
		Border: []excelize.Border{{Type: "top", Color: "#808080", Style: 2}},
	})
	wb.File.SetCellStyle(sheet, cellName(1, row), cellName(5, row), totalStyle)
	wb.setMoneyStyle(sheet, row, 4)
	wb.setMoneyStyle(sheet, row, 5)
	return nil
}
