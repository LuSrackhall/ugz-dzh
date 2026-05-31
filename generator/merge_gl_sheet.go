package generator

import (
	"fmt"
	"sort"

	"ledger/voucher"

	"github.com/xuri/excelize/v2"
)

// AppendMergeEntries 为配置中"合并总账科目"指定的父级科目生成合并 GL Sheet。
// 该父级科目下所有子科目分录按发生时间序归入同一帐页，摘要前缀 [子科目名]。
// 此方法纯增量，不影响原有叶子 GL 的生成。
func (wb *Workbook) AppendMergeEntries(entries []voucher.Entry, initials map[string]int64) error {
	if len(wb.Config.Settings.MergeGLAccounts) == 0 {
		return nil
	}

	// 构建合并科目集合
	mergeSet := make(map[string]bool)
	for _, a := range wb.Config.Settings.MergeGLAccounts {
		mergeSet[a] = true
	}

	// 按父级科目分组
	type mergeGroup struct {
		entries []voucher.Entry
	}
	groups := make(map[string]*mergeGroup)

	for _, e := range entries {
		if !mergeSet[e.GeneralAccount] {
			continue
		}
		g, ok := groups[e.GeneralAccount]
		if !ok {
			g = &mergeGroup{}
			groups[e.GeneralAccount] = g
		}
		g.entries = append(g.entries, e)
	}

	for general, g := range groups {
		if len(g.entries) == 0 {
			continue
		}
		if err := wb.appendToMergeGLSheet(general, g.entries, initials); err != nil {
			return fmt.Errorf("合并总分类账 %s: %w", general, err)
		}
	}

	return nil
}

// ensureMergeGLSheet 确保合并 GL Sheet 存在并已初始化标题。
func (wb *Workbook) ensureMergeGLSheet(general string) (string, error) {
	name := sheetNameGL(general)
	if idx, err := wb.File.GetSheetIndex(name); err == nil && idx >= 0 {
		return name, nil
	}

	idx, err := wb.File.NewSheet(name)
	if err != nil {
		return "", fmt.Errorf("创建 Sheet %s: %w", name, err)
	}
	wb.File.SetActiveSheet(idx)

	if err := wb.writeGLTitle(name); err != nil {
		return "", err
	}
	return name, nil
}

// appendToMergeGLSheet 将分录追加到指定父级科目的合并 GL Sheet。
// 摘要列格式: [子科目名] 原摘要；余额按父级汇总累计。
func (wb *Workbook) appendToMergeGLSheet(general string, entries []voucher.Entry, initials map[string]int64) error {
	sheet, err := wb.ensureMergeGLSheet(general)
	if err != nil {
		return err
	}

	rows, _ := wb.File.GetRows(sheet)
	isNew := len(rows) <= 2

	// 计算父级期初余额 = 各子科目期初之和
	var parentInitial int64
	for k, v := range initials {
		if isChildOf(k, general) {
			parentInitial += v
		}
	}

	if isNew && parentInitial != 0 {
		if err := wb.insertCarryForward(sheet, parentInitial); err != nil {
			return err
		}
	}

	// 按日期+凭证号排序
	sortEntries(entries)

	balance := parentInitial
	var pageDebit, pageCredit int64
	if !isNew {
		balance = wb.lastPageBalance(sheet)
		if !wb.pageHasBreakRow(sheet) {
			wb.markExistingPageForPrint(sheet)
		}
	}

	for _, e := range entries {
		row, err := wb.nextDataRow(sheet)
		if err != nil {
			return err
		}

		// 补承前页
		if wb.lastRowIsOrphanBreak(sheet) {
			pbDebit, pbCredit := wb.lastBreakTotals(sheet)
			wb.writeCarryForwardRow(sheet, row, balance, pbDebit, pbCredit)
			row++
			pageDebit = 0
			pageCredit = 0
		}

		// 页满 → 过次页 + 承前页
		if wb.rowIsPageBreak(sheet, row) {
			wb.writePageBreakRow(sheet, row, balance, pageDebit, pageCredit)
			row++
			wb.writeCarryForwardRow(sheet, row, balance, pageDebit, pageCredit)
			row++
			pageDebit = 0
			pageCredit = 0
		}

		balance = balance + e.DebitCents - e.CreditCents
		pageDebit += e.DebitCents
		pageCredit += e.CreditCents

		dir, dispBal := directionFor(balance, 0)

		// 摘要: [子科目] 原摘要
		summary := e.Summary
		if e.DetailAccount != "" {
			summary = fmt.Sprintf("[%s] %s", e.DetailAccount, e.Summary)
		}

		wb.File.SetCellValue(sheet, cellName(1, row), e.Date)
		wb.File.SetCellValue(sheet, cellName(2, row), e.VoucherNum)
		wb.File.SetCellValue(sheet, cellName(3, row), summary)
		wb.File.SetCellValue(sheet, cellName(4, row), centsToYuan(e.DebitCents))
		wb.File.SetCellValue(sheet, cellName(5, row), centsToYuan(e.CreditCents))
		wb.File.SetCellValue(sheet, cellName(6, row), dir)
		wb.File.SetCellValue(sheet, cellName(7, row), centsToYuan(dispBal))

		wb.setMoneyStyle(sheet, row, 4)
		wb.setMoneyStyle(sheet, row, 5)
		wb.setMoneyStyle(sheet, row, 7)

		wb.markRowForPrint(sheet, row)
		row++
	}

	return nil
}

// isChildOf 判断 account 是否为 parent 的子科目（account 以 "parent-" 开头）。
func isChildOf(account, parent string) bool {
	return len(account) > len(parent) && account[:len(parent)] == parent && account[len(parent)] == '-'
}

// sortEntries 按日期、凭证号排序分录。
func sortEntries(entries []voucher.Entry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Date != entries[j].Date {
			return entries[i].Date < entries[j].Date
		}
		return entries[i].VoucherNum < entries[j].VoucherNum
	})
}

// WriteMergeGLClosings 为所有合并 GL Sheet 追加月结行。
// activity 包含当月各科目的借/贷合计；合并科目的 activity 由其子科目汇总得出。
func (wb *Workbook) WriteMergeGLClosings(activity map[string]Activity, ytdDebit, ytdCredit, qtdDebit, qtdCredit map[string]int64, initials map[string]int64) error {
	if len(wb.Config.Settings.MergeGLAccounts) == 0 {
		return nil
	}

	for _, general := range wb.Config.Settings.MergeGLAccounts {
		sheet := sheetNameGL(general)
		// 若 Sheet 不存在（无分录），跳过
		if idx, err := wb.File.GetSheetIndex(sheet); err != nil || idx < 0 {
			continue
		}

		// 汇总该父级下所有子科目的月度活动
		var mtdDebit, mtdCredit int64
		for k, a := range activity {
			if isChildOf(k, general) {
				mtdDebit += a.Debit
				mtdCredit += a.Credit
			}
		}

		// 也包含父级自身的直接分录（无明细科目的）
		if a, ok := activity[general]; ok {
			mtdDebit += a.Debit
			mtdCredit += a.Credit
		}

		// 汇总期初
		var parentInitial int64
		for k, v := range initials {
			if isChildOf(k, general) {
				parentInitial += v
			}
		}
		parentInitial += initials[general]

		// 汇总本年累计 = 截至上月的 ytd + 当月 activity
		var cumDebit, cumCredit int64
		for k := range activity {
			if isChildOf(k, general) {
				cumDebit += ytdDebit[k]
				cumCredit += ytdCredit[k]
				cumDebit += activity[k].Debit
				cumCredit += activity[k].Credit
			}
		}
		if a, ok := activity[general]; ok {
			cumDebit += ytdDebit[general] + a.Debit
			cumCredit += ytdCredit[general] + a.Credit
		}

		// 汇总本季累计
		var qtDebit, qtCredit int64
		if isQuarterEnd(wb.Month) {
			for k := range activity {
				if isChildOf(k, general) {
					qtDebit += qtdDebit[k]
					qtCredit += qtdCredit[k]
					qtDebit += activity[k].Debit
					qtCredit += activity[k].Credit
				}
			}
			if a, ok := activity[general]; ok {
				qtDebit += qtdDebit[general] + a.Debit
				qtCredit += qtdCredit[general] + a.Credit
			}
		}

		if err := wb.writeMergeGLClosingRows(sheet, mtdDebit, mtdCredit, qtDebit, qtCredit, cumDebit, cumCredit, parentInitial); err != nil {
			return fmt.Errorf("合并总分类账 %s 月结: %w", general, err)
		}
	}

	return nil
}

// writeMergeGLClosingRows 写入合并 GL 的四行月结：本月合计、本季合计（仅季末）、本年累计、期末余额。
func (wb *Workbook) writeMergeGLClosingRows(sheet string, mtdDebit, mtdCredit, qtDebit, qtCredit, cumDebit, cumCredit int64, parentInitial int64) error {
	row, err := wb.nextDataRowAfterBreak(sheet)
	if err != nil {
		return err
	}

	// 本月合计
	wb.File.SetCellValue(sheet, cellName(1, row), "")
	wb.File.SetCellValue(sheet, cellName(2, row), "")
	wb.File.SetCellValue(sheet, cellName(3, row), "本月合计")
	wb.File.SetCellValue(sheet, cellName(4, row), centsToYuan(mtdDebit))
	wb.File.SetCellValue(sheet, cellName(5, row), centsToYuan(mtdCredit))
	wb.File.SetCellValue(sheet, cellName(6, row), "")
	wb.File.SetCellValue(sheet, cellName(7, row), "")

	monthlyStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 10},
		Border: []excelize.Border{
			{Type: "top", Color: "#808080", Style: 1},
		},
	})
	wb.File.SetCellStyle(sheet, cellName(1, row), cellName(7, row), monthlyStyle)
	wb.setMoneyStyle(sheet, row, 4)
	wb.setMoneyStyle(sheet, row, 5)
	wb.setMoneyStyle(sheet, row, 7)
	row++

	// 本季合计（仅季末）
	if isQuarterEnd(wb.Month) {
		wb.File.SetCellValue(sheet, cellName(1, row), "")
		wb.File.SetCellValue(sheet, cellName(2, row), "")
		wb.File.SetCellValue(sheet, cellName(3, row), "本季合计")
		wb.File.SetCellValue(sheet, cellName(4, row), centsToYuan(qtDebit))
		wb.File.SetCellValue(sheet, cellName(5, row), centsToYuan(qtCredit))
		wb.File.SetCellValue(sheet, cellName(6, row), "")
		wb.File.SetCellValue(sheet, cellName(7, row), "")

		qtStyle, _ := wb.File.NewStyle(&excelize.Style{
			Font: &excelize.Font{Bold: true, Size: 10},
		})
		wb.File.SetCellStyle(sheet, cellName(1, row), cellName(7, row), qtStyle)
		wb.setMoneyStyle(sheet, row, 4)
		wb.setMoneyStyle(sheet, row, 5)
		wb.setMoneyStyle(sheet, row, 7)
		row++
	}

	// 本年累计
	wb.File.SetCellValue(sheet, cellName(1, row), "")
	wb.File.SetCellValue(sheet, cellName(2, row), "")
	wb.File.SetCellValue(sheet, cellName(3, row), "本年累计")
	wb.File.SetCellValue(sheet, cellName(4, row), centsToYuan(cumDebit))
	wb.File.SetCellValue(sheet, cellName(5, row), centsToYuan(cumCredit))
	wb.File.SetCellValue(sheet, cellName(6, row), "")
	wb.File.SetCellValue(sheet, cellName(7, row), "")

	cumStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 10},
		Border: []excelize.Border{
			{Type: "bottom", Color: "#808080", Style: 1},
		},
	})
	wb.File.SetCellStyle(sheet, cellName(1, row), cellName(7, row), cumStyle)
	wb.setMoneyStyle(sheet, row, 4)
	wb.setMoneyStyle(sheet, row, 5)
	wb.setMoneyStyle(sheet, row, 7)
	row++

	// 期末余额
	endBalance := parentInitial + mtdDebit - mtdCredit
	endDir, endDisp := directionFor(endBalance, 0)

	wb.File.SetCellValue(sheet, cellName(1, row), "")
	wb.File.SetCellValue(sheet, cellName(2, row), "")
	wb.File.SetCellValue(sheet, cellName(3, row), periodEndLabel)
	wb.File.SetCellValue(sheet, cellName(4, row), "")
	wb.File.SetCellValue(sheet, cellName(5, row), "")
	wb.File.SetCellValue(sheet, cellName(6, row), endDir)
	wb.File.SetCellValue(sheet, cellName(7, row), centsToYuan(endDisp))

	endStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 10},
		Border: []excelize.Border{
			{Type: "bottom", Color: "#000000", Style: 2},
		},
	})
	wb.File.SetCellStyle(sheet, cellName(1, row), cellName(7, row), endStyle)
	wb.setMoneyStyle(sheet, row, 7)

	return nil
}
