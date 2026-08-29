package generator

import (
	"fmt"
	"sort"
	"strings"

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

	// 有子科目余额但当月无分录的合并父级：仍须建页并写期初/结转行。
	// 合并账页是子科目汇总视图——父级汇总期初非零时账页必须存在，否则跨年 1 月
	// 等无分录月份的合并账页整体消失（4b696b4 将合并父级排除出普通建页路径后，
	// "有余额无分录建页"这条职责由合并路径承担）。跨月累积的已有数据页跳过，
	// 期初链由页内上月末余额自持。
	for _, general := range wb.Config.Settings.MergeGLAccounts {
		if _, ok := groups[general]; ok {
			continue
		}
		if err := wb.ensureMergeGLPageWithInitial(general, initials); err != nil {
			return fmt.Errorf("合并总分类账 %s 期初建页: %w", general, err)
		}
	}

	return nil
}

// glSheetHasData 判断 GL sheet 表头以下是否已有数据行（分录/期初/月结/过次页均算）。
// writeGLTitle 预置的标题与表头行不算数据——旧的 len(rows)<=2 新页判定在标题写完后恒为 false。
func glSheetHasData(rows [][]string) bool {
	lay := glLayout()
	dataStart := lay.DataStartRow + 1 + lay.TopMarginRows // 首个数据行（Excel 行号）
	for i, r := range rows {
		if i+1 < dataStart {
			continue
		}
		for _, c := range r {
			if strings.TrimSpace(c) != "" {
				return true
			}
		}
	}
	return false
}

// mergeGLCarryForwardLabel 期初行摘要：1 月跨年延续且调整额未生效 → "上年结转"，否则 "期初余额"。
// （合并父级禁止直接设期初调整额，InitialAdjust[父级] 恒 false，此处与普通 GL 语义保持一致。）
func (wb *Workbook) mergeGLCarryForwardLabel(general string) string {
	if !wb.InitialAdjust[general] && strings.HasSuffix(wb.Month, "-01") {
		return "上年结转"
	}
	return "期初余额"
}

// ensureMergeGLPageWithInitial 确保有汇总期初的合并父级存在账页：
// 无页则建页并在数据区首行写期初/结转行（金额=子科目期初之和）；已有数据页不动。
func (wb *Workbook) ensureMergeGLPageWithInitial(general string, initials map[string]int64) error {
	var parentInitial int64
	for k, v := range initials {
		if isChildOf(k, general) {
			parentInitial += v
		}
	}
	if parentInitial == 0 {
		return nil
	}
	sheet, err := wb.ensureMergeGLSheet(general)
	if err != nil {
		return err
	}
	rows, _ := wb.File.GetRows(sheet)
	if glSheetHasData(rows) {
		return nil
	}
	pageNum := wb.getPageNum(sheet)
	return wb.insertCarryForwardAtRow(sheet, parentInitial, pageNum, wb.mergeGLCarryForwardLabel(general))
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

	lay := glLayout()
	rows, _ := wb.File.GetRows(sheet)
	// 新页判定：表头以下无任何数据行。旧判断 len(rows)<=2 在 ensureMergeGLSheet 写完
	// 标题（占 6 行）后恒为 false → 新建合并页永远不写期初行，分录从 0 起算，
	// 与月结期末（=父级期初+当月发生）余额链断裂（铁律二）。
	isNew := !glSheetHasData(rows)

	// 计算页码：已有过次页数 + 1
	pageNum := 1
	for _, r := range rows {
		if hasPageBreakAt(r, lay) {
			pageNum++
		}
	}

	// 计算父级期初余额 = 各子科目期初之和
	var parentInitial int64
	for k, v := range initials {
		if isChildOf(k, general) {
			parentInitial += v
		}
	}

	// 期初行：新页汇总期初≠0 必写；已有页在 1 月跨年延续或调整额生效时续写
	// （与普通 GL appendToGLSheet 语义一致）。统一走数据区下一行（AtRow），
	// 避免落到子表头行上。
	if parentInitial != 0 {
		if isNew || strings.HasSuffix(wb.Month, "-01") || wb.InitialAdjust[general] {
			if err := wb.insertCarryForwardAtRow(sheet, parentInitial, pageNum, wb.mergeGLCarryForwardLabel(general)); err != nil {
				return err
			}
		}
	}

	// 按日期+凭证号排序
	sortEntries(entries)

	balance := parentInitial
	var pageDebit, pageCredit int64
	if !isNew && parentInitial == 0 {
		balance = wb.lastPageBalance(sheet)
	}

	for _, e := range entries {
		row, err := wb.nextDataRow(sheet)
		if err != nil {
			return err
		}

		// 上一生成月以孤立过次页结尾：补新页头 + 承前页。
		// 必须与翻页分支同款定位（越过边距 → 新页标题行 → 数据首行），
		// 并先更新页码——否则承前页带着旧页码写进旧列区（反页内容错位到正面列）。
		if wb.lastRowIsOrphanBreak(sheet) {
			pbDebit, pbCredit := wb.lastBreakTotals(sheet)
			pageNum = wb.getPageNum(sheet)
			row += lay.BottomMarginRows + lay.TopMarginRows
			wb.writePageHeader(sheet, row, pageNum, general)
			row += lay.DataStartRow
			wb.writeCarryForwardRow(sheet, row, balance, pbDebit, pbCredit, pageNum)
			row++
			pageDebit = 0
			pageCredit = 0
		}

		// 页满 → 过次页 + 新页头 + 承前页（与普通 GL appendToGLSheet 一致）。
		// 此前缺三步：页码不更新（承前页/后续分录带着旧页码写进旧列区——
		// 反面页一部分被写进左侧正面页列数中）、不写新页头（新页无标题表头，
		// 逐月月结视觉上连成一片）、不跳数据首行（承前页落在新页边距上）。
		if wb.rowIsPageBreak(sheet, row) {
			wb.writePageBreakRow(sheet, row, balance, pageDebit, pageCredit, pageNum)
			row++
			pageNum = wb.getPageNum(sheet)
			row += lay.BottomMarginRows + lay.TopMarginRows
			marginStart := row - lay.BottomMarginRows - lay.TopMarginRows
			for d := marginStart; d < row; d++ {
				h := 19.0 // 下边距（与 GL 其他翻页统一）
				if d >= row-lay.TopMarginRows {
					h = 16.0 // 下页上边距（与 GL 其他翻页统一）
				}
				wb.File.SetRowHeight(sheet, d, h)
			}
			wb.writePageHeader(sheet, row, pageNum, general)
			row += lay.DataStartRow
			wb.writeCarryForwardRow(sheet, row, balance, pageDebit, pageCredit, pageNum)
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

		month := e.Date[5:7]
		day := e.Date[8:10]
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 0), row), month)
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 1), row), day)
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 2), row), "")
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 3), row), fmt.Sprintf("%d", e.VoucherNum))
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 4), row), summary)
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, glColDebit), row), centsToYuan(e.DebitCents))
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, glColCredit), row), centsToYuan(e.CreditCents))
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, glColDir), row), dir)
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, glColBalance), row), centsToYuan(dispBal))

		wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, glColDebit))
		wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, glColCredit))
		wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, glColBalance))

		// 摘要列自动换行 + 9号加粗
		summaryCell := cellName(dataCol(lay, pageNum, 4), row)
		summaryStyle, _ := wb.File.NewStyle(&excelize.Style{
			Font:      &excelize.Font{Size: 9, Bold: true},
			Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center", WrapText: true},
			Border: []excelize.Border{
				{Type: "top", Color: "#006100", Style: 1},
				{Type: "right", Color: "#006100", Style: 1},
				{Type: "bottom", Color: "#006100", Style: 1},
				{Type: "left", Color: "#006100", Style: 1},
			},
		})
		wb.File.SetCellStyle(sheet, summaryCell, summaryCell, summaryStyle)
		row++
	}

	return nil
}

// isChildOf 判断 account 是否为 parent 的子科目（account 以 "parent-" 开头）。
func isChildOf(account, parent string) bool {
	return len(account) > len(parent) && account[:len(parent)] == parent && account[len(parent)] == '-'
}

// applyGLClosingRowStyle 为合并 GL 月结行应用整行样式：完整绿框（#006100 thin，四边），
// 页内每第 5 行底边加粗，金额列套 #,##0.00 格式——与普通 GL 月结（WriteMonthClosings）一致。
// 样式必须自包含：无分录月 appendToMergeGLSheet 不执行，账页没有预置绿框可依赖，
// 否则打印版月结行列线断裂。范围用 dataCol 按当前页奇偶取列，避免反页样式写进正面区。
func (wb *Workbook) applyGLClosingRowStyle(sheet string, row, pageNum int) {
	lay := glLayout()
	thick := glRowInPage(lay, row)%5 == 0
	bottomStyle := 1
	if thick {
		bottomStyle = 2
	}
	st, _ := wb.File.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 10},
		Border: []excelize.Border{
			{Type: "top", Color: "#006100", Style: 1},
			{Type: "right", Color: "#006100", Style: 1},
			{Type: "bottom", Color: "#006100", Style: bottomStyle},
			{Type: "left", Color: "#006100", Style: 1},
		},
	})
	wb.File.SetCellStyle(sheet, cellName(dataCol(lay, pageNum, 0), row),
		cellName(dataCol(lay, pageNum, glColCount-1), row), st)
	if thick {
		wb.setMoneyStyleThick(sheet, row, dataCol(lay, pageNum, glColDebit))
		wb.setMoneyStyleThick(sheet, row, dataCol(lay, pageNum, glColCredit))
		wb.setMoneyStyleThick(sheet, row, dataCol(lay, pageNum, glColBalance))
	} else {
		wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, glColDebit))
		wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, glColCredit))
		wb.setMoneyStyle(sheet, row, dataCol(lay, pageNum, glColBalance))
	}
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

		// 汇总期初（第三轮审查 D1c：仅由子科目期初聚合；父级自身 initials 不再叠加——
		// 合并父级不作叶子记账，叠加会虚增（initials[general] 可能来自合并视图污染或父级自身期初））
		var parentInitial int64
		for k, v := range initials {
			if isChildOf(k, general) {
				parentInitial += v
			}
		}

		// 汇总本年累计 = 截至上月的 ytd + 当月 activity
		// 遍历 activity ∪ Tree 全部叶子——审计二审 H3 两处缺口：
		//   ① 只遍历 activity：无活动子科目的历史累计被漏（父级累计 < 叶子之和）；
		//   ② 只遍历 Tree：当月**首次出现**的子科目尚未回写进 Tree，累计被漏（如新年首月）。
		var cumDebit, cumCredit int64
		seen := make(map[string]bool)
		for k := range activity {
			if isChildOf(k, general) {
				cumDebit += ytdDebit[k] + activity[k].Debit
				cumCredit += ytdCredit[k] + activity[k].Credit
				seen[k] = true
			}
		}
		for k := range wb.Config.Tree {
			if !seen[k] && isChildOf(k, general) {
				cumDebit += ytdDebit[k] + activity[k].Debit
				cumCredit += ytdCredit[k] + activity[k].Credit
			}
		}
		if a, ok := activity[general]; ok {
			cumDebit += ytdDebit[general] + a.Debit
			cumCredit += ytdCredit[general] + a.Credit
		}

		// 汇总本季累计
		var qtDebit, qtCredit int64
		if isQuarterEnd(wb.Month) {
			seenQ := make(map[string]bool)
			for k := range activity {
				if isChildOf(k, general) {
					qtDebit += qtdDebit[k] + activity[k].Debit
					qtCredit += qtdCredit[k] + activity[k].Credit
					seenQ[k] = true
				}
			}
			for k := range wb.Config.Tree {
				if !seenQ[k] && isChildOf(k, general) {
					qtDebit += qtdDebit[k] + activity[k].Debit
					qtCredit += qtdCredit[k] + activity[k].Credit
				}
			}
			if a, ok := activity[general]; ok {
				qtDebit += qtdDebit[general] + a.Debit
				qtCredit += qtdCredit[general] + a.Credit
			}
		}

		if err := wb.writeMergeGLClosingRows(sheet, general, mtdDebit, mtdCredit, qtDebit, qtCredit, cumDebit, cumCredit, parentInitial); err != nil {
			return fmt.Errorf("合并总分类账 %s 月结: %w", general, err)
		}
	}

	return nil
}

// writeMergeGLClosingRows 写入合并 GL 的四行月结：本月合计、本季合计（仅季末）、本年累计、期末余额。
// 每写一行前检查页容量，满了就过次页翻页（审计 P1-1：补 checkBreak 翻页保护，保证每页恰好20数据行+1过次页行，
// 避免末页剩余<4行时月结越界破坏余额链连续性——铁律二）。
func (wb *Workbook) writeMergeGLClosingRows(sheet string, account string, mtdDebit, mtdCredit, qtDebit, qtCredit, cumDebit, cumCredit int64, parentInitial int64) error {
	lay := glLayout()

	// 计算末页页码
	rows, _ := wb.File.GetRows(sheet)
	pageNum := 1
	for _, r := range rows {
		if hasPageBreakAt(r, lay) {
			pageNum++
		}
	}

	row, err := wb.nextDataRowAfterBreak(sheet)
	if err != nil {
		return err
	}

	// 期末余额 = 期初 + 本月借 - 本月贷（月结翻页时承前页承接此余额）
	balance := parentInitial + mtdDebit - mtdCredit
	var closingDebit, closingCredit int64

	// 检查页容量，满了就翻页（照搬 WriteMonthClosings 的 checkBreak 模式）
	checkBreak := func() {
		pageStart := wb.pageStartRow(sheet)
		if row-pageStart >= pageSize {
			wb.writePageBreakRow(sheet, row, balance, closingDebit, closingCredit, pageNum)
			row++
			pageNum = wb.getPageNum(sheet)
			row += lay.BottomMarginRows + lay.TopMarginRows
			marginStart := row - lay.BottomMarginRows - lay.TopMarginRows
			for d := marginStart; d < row; d++ {
				h := 19.0 // 下边距（与 GL 其他翻页统一）
				if d >= row-lay.TopMarginRows {
					h = 16.0 // 下页上边距（与 GL 其他翻页统一）
				}
				wb.File.SetRowHeight(sheet, d, h)
			}
			wb.writePageHeader(sheet, row, pageNum, account)
			row += lay.DataStartRow
			wb.writeCarryForwardRow(sheet, row, balance, closingDebit, closingCredit, pageNum)
			row++
			closingDebit = 0
			closingCredit = 0
		}
	}

	// 本月合计
	checkBreak()
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 0), row), "")
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 1), row), "")
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 4), row), "本月合计")
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, glColDebit), row), centsToYuan(mtdDebit))
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, glColCredit), row), centsToYuan(mtdCredit))
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, glColDir), row), "")
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, glColBalance), row), "")
	wb.applyGLClosingRowStyle(sheet, row, pageNum)
	closingDebit += mtdDebit
	closingCredit += mtdCredit
	row++

	// 本季合计（仅季末）
	if isQuarterEnd(wb.Month) {
		checkBreak()
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 0), row), "")
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 1), row), "")
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 4), row), "本季合计")
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, glColDebit), row), centsToYuan(qtDebit))
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, glColCredit), row), centsToYuan(qtCredit))
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, glColDir), row), "")
		wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, glColBalance), row), "")
		wb.applyGLClosingRowStyle(sheet, row, pageNum)
		row++
	}

	// 本年累计
	checkBreak()
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 0), row), "")
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 1), row), "")
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 4), row), "本年累计")
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, glColDebit), row), centsToYuan(cumDebit))
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, glColCredit), row), centsToYuan(cumCredit))
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, glColDir), row), "")
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, glColBalance), row), "")
	wb.applyGLClosingRowStyle(sheet, row, pageNum)
	row++

	// 期末余额
	checkBreak()
	endBalance := parentInitial + mtdDebit - mtdCredit
	endDir, endDisp := directionFor(endBalance, 0)

	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 0), row), "")
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 1), row), "")
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, 4), row), periodEndLabel)
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, glColDir), row), endDir)
	wb.File.SetCellValue(sheet, cellName(dataCol(lay, pageNum, glColBalance), row), centsToYuan(endDisp))
	wb.applyGLClosingRowStyle(sheet, row, pageNum)

	return nil
}
