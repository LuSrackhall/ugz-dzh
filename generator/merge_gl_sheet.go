package generator

import (
	"fmt"
	"sort"

	"ledger/voucher"
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
