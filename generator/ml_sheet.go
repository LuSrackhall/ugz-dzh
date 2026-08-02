package generator

import (
	"fmt"
	"sort"
	"strings"

	"ledger/generator/layout"
	"ledger/voucher"

	"github.com/xuri/excelize/v2"
)

const (
	mlMaxDetails     = 14 // 明细科目上限
	mlDetailStartCol = 8  // 明细列起始列 H（保留常量用于非 Layout 上下文）

	// Back 侧基础列偏移（相对 BackStartCol），与四行表头对齐：
	// Row4: [年(合并)] [凭证(合并)] 摘要 借..方 贷..方 借或贷 余..额
	// Row5:  月  日   字  号      明细科目们（中两行）
	// Row6: （延续）              （延续）
	// Row7: （延续）  摘要(四行)   借或贷(四行)
	mlOffMonth    = 0 // 月
	mlOffDay      = 1 // 日
	mlOffVouChar  = 2 // 字
	mlOffVouNum   = 3 // 号
	mlOffSummary  = 4 // 摘要
	mlOffDebit    = 5 // 借方金额
	mlOffCredit   = 6 // 贷方金额
	mlOffDir      = 7 // 方向
	mlOffBalance  = 8 // 余额
)

// mlFirstDataPageStart 返回第一个数据页 block 的起始行号（= 该页上边距行）。
// Paper1 Front 占位页占 rows 1 ~ (DataStartRow + pageSize + 1 + BottomMarginRows)，
// 即 1 ~ 30（上边距 + 7页头 + 20数据 + 过次页 + 下边距）。
func mlFirstDataPageStart() int {
	lay := mlLayout()
	return lay.DataStartRow + pageSize + 2 + lay.BottomMarginRows // = 8 + 20 + 2 + 1 = 31
}

// mlLayout 返回多科目明细账的布局规格（独立于 GL 布局）。
func mlLayout() layout.MLLayout {
	return layout.MLComputeLayout(layout.DefaultMLSpec())
}

// mlPrintMarkCol 返回多科目明细账打印标记列号（Back 区末列与 Front 区末列取较大值）。
func mlPrintMarkCol() int {
	lay := mlLayout()
	backLast := lay.BackStartCol + lay.BackColCount
	frontLast := lay.FrontStartCol + lay.FrontColCount
	if frontLast > backLast {
		return frontLast
	}
	return backLast
}

// mlDetailCol 返回第 i 个明细列的 Excel 列号（1-indexed）。
// i=0~3 → Back 侧（左半），i=4~13 → Front 侧（右半）。
func mlDetailCol(lay layout.MLLayout, i int) int {
	if i < 4 {
		return lay.BackStartCol + 9 + i // 9个基本列后直接接明细
	}
	return lay.FrontStartCol + (i - 4) + 2 // 1-indexed: FrontStartCol(15) + 1 = 16 for i=4
}

// mlDetailRowIdx 返回第 i 个明细列在 GetRows 中的索引。
// i=0~3 → Back 侧 GetRows 索引 = BindingLeftCols + 10 + i，
// i=4~13 → Front 侧 GetRows 索引 = FrontStartCol - 1 + (i - 4)。
func mlDetailRowIdx(lay layout.MLLayout, i int) int {
	if i < 4 {
		return lay.BindingLeftCols + 9 + i // 0-indexed: BindingLeftCols(2) + 9 + 0 = 11 (col L)
	}
	return lay.FrontStartCol + (i - 4) + 1 // 0-indexed: FrontStartCol(15) + 1 = 16 for i=4
}

// ── ML 独立辅助函数（与 GL 同名函数功能相同但使用 mlLayout） ──

// mlCellName 返回 Excel 单元格名（与 cellName 相同功能，ML 独立版本）。
func mlCellName(col, row int) string {
	name, _ := excelize.CoordinatesToCellName(col, row)
	return name
}

// mlHasPageBreakAt 检查行中是否有"过次页"标记（仅 Back 侧摘要列）。
func mlHasPageBreakAt(row []string, lay layout.MLLayout) bool {
	summaryIdx := lay.BindingLeftCols + mlOffSummary
	return len(row) > summaryIdx && row[summaryIdx] == pageBreakLabel
}

// mlIsStructuralBreak 检查过次页行是否为结构预写（余额列为空字符串，无翻页数据）。
// 注意：借方为 "0" 的过次页可能是真实翻页（收入类科目无借方），用余额列判断更可靠。
func mlIsStructuralBreak(row []string, lay layout.MLLayout) bool {
	balIdx := lay.BindingLeftCols + mlOffBalance
	return len(row) <= balIdx ||
		strings.TrimSpace(row[balIdx]) == ""
}

// mlLastPeriodEndRow 从 Sheet 中找到最后一个"期末余额"行的行号（1-based）。
// 期末余额永远是月结的最后一行，用它定位本月数据尾行，无需额外标记列。
func (wb *Workbook) mlLastPeriodEndRow(sheet string) int {
	lay := mlLayout()
	rows, _ := wb.File.GetRows(sheet)
	sumIdx := lay.BindingLeftCols + mlOffSummary
	for i := len(rows) - 1; i >= 0; i-- {
		if len(rows[i]) > sumIdx && rows[i][sumIdx] == periodEndLabel {
			return i + 1
		}
	}
	return 0
}

// mlLastContentRow 返回 Sheet 中最后一个有内容的行索引（0-based）。
// 跳过尾部空行，但不跳过结构过次页（它是模板的一部分）。
// 找不到返回 -1。
func mlLastContentRow(rows [][]string) int {
	for i := len(rows) - 1; i >= 0; i-- {
		isEmpty := true
		for k := 0; k < len(rows[i]); k++ {
			if strings.TrimSpace(rows[i][k]) != "" {
				isEmpty = false
				break
			}
		}
		if !isEmpty {
			return i
		}
	}
	return -1
}

// mlLastDataBeforeBreak 返回 Sheet 中最后一个数据行索引（0-based）。
// 只跳过结构过次页（余额为空）和尾部空行。
// 真实过次页（有余额数据）和月结行都算数据行。
func mlLastDataBeforeBreak(rows [][]string, lay layout.MLLayout) int {
	for i := len(rows) - 1; i >= 0; i-- {
		if mlHasPageBreakAt(rows[i], lay) && mlIsStructuralBreak(rows[i], lay) {
			continue
		}
		isEmpty := true
		for k := 0; k < len(rows[i]); k++ {
			if strings.TrimSpace(rows[i][k]) != "" {
				isEmpty = false
				break
			}
		}
		if isEmpty {
			continue
		}
		return i
	}
	return -1
}

// (wb *Workbook) mlNextDataRow 返回 Sheet 中下一个可用数据行号。
// 找最后一条数据行（跳过空行和过次页），返回其下一行。
// 数据自然填满空行，到第21行（过次页位置）时 mlRowIsPageBreak 触发翻页。
func (wb *Workbook) mlNextDataRow(sheet string) (int, error) {
	lay := mlLayout()
	rows, err := wb.File.GetRows(sheet)
	if err != nil || len(rows) < 3 {
		return mlFirstDataPageStart() + lay.DataStartRow + 1, nil
	}

	lastDataIdx := mlLastDataBeforeBreak(rows, lay)
	if lastDataIdx < 0 {
		return mlFirstDataPageStart() + lay.DataStartRow + 1, nil
	}

	// 如果最后一条数据是真实过次页 → 新页面，承前页在 DataStartRow 后
	if mlHasPageBreakAt(rows[lastDataIdx], lay) && !mlIsStructuralBreak(rows[lastDataIdx], lay) {
		return lastDataIdx + 2 + lay.DataStartRow + lay.BottomMarginRows, nil
	}

	return lastDataIdx + 2, nil // 0-indexed → 1-based, then +1 for next row
}

// (wb *Workbook) mlLastPageBalance 获取最近一个过次页结束的余额。
// 有过次页时读最后一个过次页之前最近的期末余额或数据行余额。
// 无过次页时从最后一行往前找最近的余额（跳过承前页和结构行）。
func (wb *Workbook) mlLastPageBalance(sheet string) int64 {
	lay := mlLayout()
	rows, err := wb.File.GetRows(sheet)
	if err != nil {
		return 0
	}
	// 从后往前找最后一个真实过次页
	lastBreak := -1
	for i := len(rows) - 1; i >= 0; i-- {
		if mlHasPageBreakAt(rows[i], lay) && !mlIsStructuralBreak(rows[i], lay) {
			lastBreak = i
			break
		}
	}
	if lastBreak >= 0 {
		// 找上一个过次页（或页首），确定当前页范围
		prevBreak := 0
		for i := lastBreak - 1; i >= 0; i-- {
			if mlHasPageBreakAt(rows[i], lay) && !mlIsStructuralBreak(rows[i], lay) {
				prevBreak = i + 1
				break
			}
		}
		// 优先：找最近的非承前页、非月结行的数据行余额（即分录行的 running balance）
		for i := lastBreak - 1; i >= prevBreak; i-- {
			r := rows[i]
			if len(r) <= lay.BindingLeftCols+mlOffBalance {
				continue
			}
			summary := ""
			if len(r) > lay.BindingLeftCols+mlOffSummary {
				summary = r[lay.BindingLeftCols+mlOffSummary]
			}
			// 跳过承前页行、月结行
			if summary == carryForwardLabel || summary == "上年结转" ||
				summary == "本月合计" || summary == "本季合计" || summary == "本年累计" || summary == periodEndLabel {
				continue
			}
			balStr := strings.TrimSpace(r[lay.BindingLeftCols+mlOffBalance])
			if balStr == "" {
				continue
			}
			if v, err := yuanStrToCents(balStr); err == nil {
				return v
			}
		}
		// 回退到过次页行自身余额
		if len(rows[lastBreak]) > lay.BindingLeftCols+mlOffBalance {
			if v, err := yuanStrToCents(rows[lastBreak][lay.BindingLeftCols+mlOffBalance]); err == nil {
				return v
			}
		}
		return 0
	}
	// 无过次页 → 从后往前找最近的余额（跳过承前页行和结构行）
	for i := len(rows) - 1; i >= 0; i-- {
		r := rows[i]
		if mlHasPageBreakAt(r, lay) && mlIsStructuralBreak(r, lay) {
			continue
		}
		if len(r) > lay.BindingLeftCols+mlOffSummary &&
			(r[lay.BindingLeftCols+mlOffSummary] == carryForwardLabel || r[lay.BindingLeftCols+mlOffSummary] == "上年结转") {
			continue
		}
		balIdx := lay.BindingLeftCols + mlOffBalance
		if len(r) <= balIdx {
			continue
		}
		if v, err := yuanStrToCents(r[balIdx]); err == nil {
			return v
		}
	}
	return 0
}

// (wb *Workbook) mlLastBreakTotals 获取最后一个过次页行的页借贷合计（仅从 Back 侧读取）。
func (wb *Workbook) mlLastBreakTotals(sheet string) (debit, credit int64) {
	lay := mlLayout()
	rows, err := wb.File.GetRows(sheet)
	if err != nil {
		return 0, 0
	}
	for i := len(rows) - 1; i >= 0; i-- {
		if !mlHasPageBreakAt(rows[i], lay) || mlIsStructuralBreak(rows[i], lay) {
			continue
		}
		debIdx := lay.BindingLeftCols + mlOffDebit
		crdIdx := lay.BindingLeftCols + mlOffCredit
		if len(rows[i]) > crdIdx {
			if v, err := yuanStrToCents(rows[i][debIdx]); err == nil {
				debit = v
			}
			if v, err := yuanStrToCents(rows[i][crdIdx]); err == nil {
				credit = v
			}
		}
		return
	}
	return 0, 0
}

// (wb *Workbook) mlLastRowIsOrphanBreak 检查最后一行是否为没有承前页跟随的孤立过次页。
func (wb *Workbook) mlLastRowIsOrphanBreak(sheet string) bool {
	lay := mlLayout()
	rows, err := wb.File.GetRows(sheet)
	if err != nil || len(rows) == 0 {
		return false
	}
	last := rows[len(rows)-1]
	if !mlHasPageBreakAt(last, lay) {
		return false
	}
	// 预写的结构过次页只有摘要文字，没有借方数据，不视为孤立翻页
	debIdx := lay.BindingLeftCols + mlOffDebit
	if len(last) <= debIdx || strings.TrimSpace(last[debIdx]) == "" {
		return false
	}
	return true
}

// (wb *Workbook) mlPageStartRow 返回当前页第一个有效数据行的行号。
// 只找真实过次页（有余额数据），跳过结构过次页（模板占位）。
func (wb *Workbook) mlPageStartRow(sheet string) int {
	lay := mlLayout()
	rows, err := wb.File.GetRows(sheet)
	if err != nil || len(rows) < 3 {
		return mlFirstDataPageStart() + lay.DataStartRow
	}
	for i := len(rows) - 1; i >= 0; i-- {
		if mlHasPageBreakAt(rows[i], lay) && !mlIsStructuralBreak(rows[i], lay) {
			return i + 2 + lay.DataStartRow + lay.BottomMarginRows
		}
	}
	return mlFirstDataPageStart() + lay.DataStartRow
}

// (wb *Workbook) mlRowIsPageBreak 检查指定行是否已超出当页容量。
func (wb *Workbook) mlRowIsPageBreak(sheet string, row int) bool {
	start := wb.mlPageStartRow(sheet)
	return row-start >= pageSize
}

// (wb *Workbook) mlPageHasBreakRow 检查当前页是否已有过次页行。
func (wb *Workbook) mlPageHasBreakRow(sheet string) bool {
	lay := mlLayout()
	rows, err := wb.File.GetRows(sheet)
	if err != nil {
		return false
	}
	for i := len(rows) - 1; i >= 0; i-- {
		if mlHasPageBreakAt(rows[i], lay) && !mlIsStructuralBreak(rows[i], lay) {
			return true
		}
	}
	return false
}

// (wb *Workbook) mlNextDataRowAfterBreak 返回月结追加的下一个可用数据行。
// 找最后一个"期末余额"行（月结最后一行），返回其下一行。
// 如果尾行之后有真实过次页（翻页），返回新页面的承前页之后。
func (wb *Workbook) mlNextDataRowAfterBreak(sheet string) (int, error) {
	lay := mlLayout()
	rows, err := wb.File.GetRows(sheet)
	if err != nil || len(rows) < 3 {
		return mlFirstDataPageStart() + lay.DataStartRow + 1, nil
	}

	tailRow := wb.mlLastPeriodEndRow(sheet)
	if tailRow > 0 {
		// 检查尾行之后是否有真实过次页
		for i := tailRow; i < len(rows); i++ {
			if mlHasPageBreakAt(rows[i], lay) && !mlIsStructuralBreak(rows[i], lay) {
				return i + 2 + lay.DataStartRow + lay.BottomMarginRows, nil
			}
		}
		return tailRow + 1, nil
	}

	// 无期末余额 → 回退到 mlNextDataRow
	return wb.mlNextDataRow(sheet)
}

// mlDetailTotals 明细科目合计。
type mlDetailTotals struct {
	debit  int64
	credit int64
}

// ensureMLSheet 确保多科目明细账 Sheet 存在并已初始化标题和扩展列。
// 已存在的 Sheet 读头保序，新科目追加到右侧空列；全新 Sheet 按 detailOrder 或字母序初始化。
func (wb *Workbook) ensureMLSheet(general string, details []string, detailOrder []string) (string, map[string]int, []string, error) {
	name := sheetNameML(general)
	if idx, err := wb.File.GetSheetIndex(name); err == nil && idx >= 0 {
		// Sheet 已存在 — 读头保序
		existingIdx, existingDetails, err := wb.readMLDetailHeaders(name)
		if err != nil {
			return "", nil, nil, err
		}
		_ = existingIdx

		// 冲突检测：若配置了 detailOrder，逐列比对
		if len(detailOrder) > 0 {
			var existNonEmpty []string
			for _, d := range existingDetails {
				if d != "" { existNonEmpty = append(existNonEmpty, d) }
			}
			fmt.Printf("DEBUG %s: existNonEmpty=%v detailOrder=%v\n", name, existNonEmpty, detailOrder)
			if err := wb.checkMLDetailOrderConflict(name, existingDetails, detailOrder); err != nil {
				return "", nil, nil, err
			}
		}

		// 合并新科目到空列
		finalDetails, finalIdx, newAppended, err := resolveMLDetailColumns(existingDetails, details, detailOrder)
		if err != nil {
			return "", nil, nil, err
		}
		_ = finalDetails

		// 更新标题行（仅更新新增的列，数据页列标题行）
			lay := mlLayout()
			colHeaderRow := mlFirstDataPageStart() + 5 // Row+4 of first data page header
			for _, nd := range newAppended {
				col := mlDetailCol(lay, finalIdx[nd])
				cell := mlCellName(col, colHeaderRow)
				wb.File.SetCellValue(name, cell, nd)
		}
			// 更新列宽（Front 侧明细列）
			for i := 0; i < mlMaxDetails; i++ {
				colLetter := cellColLetter(mlDetailCol(lay, i))
				wb.File.SetColWidth(name, colLetter, colLetter, 14)
		}

		return name, finalIdx, newAppended, nil
	}

	// 新 Sheet — 创建
	idx, err := wb.File.NewSheet(name)
	if err != nil {
		return "", nil, nil, fmt.Errorf("创建 Sheet %s: %w", name, err)
	}
	wb.File.SetActiveSheet(idx)

	// 初始化列序：若存在 detailOrder，使用配置；否则按字母序
	var initDetails []string
	var newAppended []string
	if len(detailOrder) > 0 {
		// 直接复制 detailOrder 完整列表（含 "" 跳列和未发生科目）
		initDetails = make([]string, len(detailOrder))
		copy(initDetails, detailOrder)

		// 当月分录中不在 detailOrder 中的科目 → 追加到右侧空列
		inOrder := make(map[string]bool)
		for _, d := range detailOrder {
			if d != "" {
				inOrder[d] = true
			}
		}
		var remaining []string
		for _, d := range details {
			if !inOrder[d] {
				remaining = append(remaining, d)
			}
		}
		sort.Strings(remaining)
		initDetails = append(initDetails, remaining...)
		newAppended = remaining
	} else {
		initDetails = make([]string, len(details))
		copy(initDetails, details)
		sort.Strings(initDetails)
		newAppended = initDetails
	}

	if len(initDetails) > mlMaxDetails {
		return "", nil, nil, fmt.Errorf("总账科目 %q 明细科目数 %d 超过上限 %d", general, len(initDetails), mlMaxDetails)
	}

	// Paper1 Front 占位页：空占位表（Front 侧，页码=0，不写 Back 侧）
	lay := mlLayout()
	if err := wb.writeMLPageHeader(name, 1, 0, 0, general, false, true); err != nil {
		return "", nil, nil, err
	}

	// 占位表为空白模板：不写明细科目名（保持空列），也不写 Back 侧结构过次页占位
	// 首个数据页页头写入实际明细科目名
	wb.writeMLDetailNamesAt(name, mlFirstDataPageStart(), initDetails)
	// 列宽（Front 侧明细列）
	for i := 0; i < mlMaxDetails; i++ {
		colLetter := cellColLetter(mlDetailCol(lay, i))
		wb.File.SetColWidth(name, colLetter, colLetter, 14)
	}

	detailIdx := make(map[string]int)
	for i, d := range initDetails {
		if d != "" {
			detailIdx[d] = i
		}
	}

	return name, detailIdx, newAppended, nil
}

// checkMLDetailOrderConflict 验证现有 Sheet 列标题与 detailOrder 配置的兼容性。
// 检查现有列标题在配置中是否保持相对顺序，允许配置有额外的未写入列。
func (wb *Workbook) checkMLDetailOrderConflict(sheet string, existingDetails []string, detailOrder []string) error {
	if len(detailOrder) == 0 {
		return nil
	}
	// 提取现有非空列
	var existingNonEmpty []string
	for _, d := range existingDetails {
		if d != "" {
			existingNonEmpty = append(existingNonEmpty, d)
		}
	}
	if len(existingNonEmpty) == 0 {
		return nil
	}
	// 检查现有列在 detailOrder 中是否保持相对顺序
	lastFoundIdx := -1
	for _, existing := range existingNonEmpty {
		foundIdx := -1
		for j := lastFoundIdx + 1; j < len(detailOrder); j++ {
			if detailOrder[j] == existing {
				foundIdx = j
				break
			}
		}
		if foundIdx < 0 {
			return fmt.Errorf("Sheet %s: detailOrder 与现有列序冲突 — %q 在现有列中但不在配置中。请使用 -f 从首月重新生成", sheet, existing)
		}
		lastFoundIdx = foundIdx
	}
	return nil
}


// cellColLetter 返回列号的字母表示。
func cellColLetter(col int) string {
	l, _ := excelize.ColumnNumberToName(col)
	return l
}

// updateMLDetailHeaders 更新已有 Sheet 的明细列标题（Paper1 Front 第5行），以匹配当月明细科目集。
func (wb *Workbook) updateMLDetailHeaders(sheet string, details []string) {
	lay := mlLayout()
	colHeaderRow := mlFirstDataPageStart() + 5 // Row+4 of first data page header // 1-based
	for i := 0; i < mlMaxDetails; i++ {
		col := mlDetailCol(lay, i)
		cell := mlCellName(col, colHeaderRow)
		label := ""
		if i < len(details) {
			label = details[i]
		}
		wb.File.SetCellValue(sheet, cell, label)
	}
}

// readMLDetailHeaders 从 Sheet 第5行（Paper1 Front 表头行）读取现有明细列标题。
// 返回的 details 按列顺序排列（空列对应空字符串）。
func (wb *Workbook) readMLDetailHeaders(sheet string) (detailIdx map[string]int, details []string, err error) {
	lay := mlLayout()
	detailIdx = make(map[string]int)
	details = make([]string, mlMaxDetails)

	rows, err := wb.File.GetRows(sheet)
	if err != nil {
		return nil, nil, fmt.Errorf("读取 Sheet %s: %w", sheet, err)
	}
	colHeaderRow := mlFirstDataPageStart() + 5 // Row+4 of first data page header // 1-based row number
	if len(rows) < colHeaderRow {
		return detailIdx, details, nil
	}

	rowData := rows[colHeaderRow-1]
	for i := 0; i < mlMaxDetails; i++ {
		colIdx := mlDetailRowIdx(lay, i)
		label := ""
		if colIdx < len(rowData) {
			label = strings.TrimSpace(rowData[colIdx])
		}
		details[i] = label
		if label != "" {
			detailIdx[label] = i
		}
	}
	return detailIdx, details, nil
}

// writeMLDetailNamesAt 在指定页头的表头第2行（4行表头的中两行首行）写入实际明细科目名。
// 无科目或空列写入空字符串（不写"明细N"占位）。
// headerRow 是页 block 起始行（上边距行），明细名位于 headerRow + 5。
func (wb *Workbook) writeMLDetailNamesAt(sheet string, headerRow int, details []string) {
	lay := mlLayout()
	colHeaderRow := headerRow + 5
	for i := 0; i < mlMaxDetails; i++ {
		col := mlDetailCol(lay, i)
		cell := mlCellName(col, colHeaderRow)
		label := ""
		if i < len(details) {
			label = details[i]
		}
		wb.File.SetCellValue(sheet, cell, label)
	}
}

// resolveMLDetailColumns 合并已有列映射与新科目，返回完整列序、映射、新增科目列表。
// existingDetails: 从 Sheet 第2行读取的现有列序（空字符串表示空列）
// newDetails: 当月分录中的明细科目集合
// detailOrder: 用户配置的列序（nil 表示无配置）
func resolveMLDetailColumns(existingDetails []string, newDetails []string, detailOrder []string) (details []string, detailIdx map[string]int, newAppended []string, err error) {
	details = make([]string, mlMaxDetails)
	copy(details, existingDetails)

	detailIdx = make(map[string]int)
	for i, d := range details {
		if d != "" {
			detailIdx[d] = i
		}
	}

	// 找出不在现有列中的新科目
	var toAdd []string
	for _, nd := range newDetails {
		if _, ok := detailIdx[nd]; !ok {
			toAdd = append(toAdd, nd)
		}
	}
	if len(toAdd) == 0 {
		return details, detailIdx, nil, nil
	}

	// 如果有 detailOrder，按配置顺序排列；否则按字母序
	if len(detailOrder) > 0 {
		orderMap := make(map[string]int)
		for i, d := range detailOrder {
			orderMap[d] = i
		}
		sort.Slice(toAdd, func(i, j int) bool {
			oi, iok := orderMap[toAdd[i]]
			oj, jok := orderMap[toAdd[j]]
			if iok && jok {
				return oi < oj
			}
			if iok {
				return true
			}
			if jok {
				return false
			}
			return toAdd[i] < toAdd[j]
		})
	} else {
		sort.Strings(toAdd)
	}

	// 追加到右侧第一个空列
	for _, nd := range toAdd {
		placed := false
		for i := 0; i < mlMaxDetails; i++ {
			if details[i] == "" {
				details[i] = nd
				detailIdx[nd] = i
				newAppended = append(newAppended, nd)
				placed = true
				break
			}
		}
		if !placed {
			return nil, nil, nil, fmt.Errorf("多科目明细列已满（14列全部占用），无法追加 %q。已占用: %v", nd, nonEmptyDetails(details))
		}
	}

	return details, detailIdx, newAppended, nil
}

// nonEmptyDetails 返回非空明细科目列表。
func nonEmptyDetails(details []string) []string {
	var result []string
	for _, d := range details {
		if d != "" {
			result = append(result, d)
		}
	}
	return result
}

// AppendMLEntries 将分录追加到多科目明细账 Sheet。
func (wb *Workbook) AppendMLEntries(entries []voucher.Entry, initials map[string]int64) error {
	type mlGroup struct {
		entries []voucher.Entry
		details []string
	}
	groups := make(map[string]*mlGroup)

	// 构建忽略集合
	mlSuppress := make(map[string]bool)
	for _, a := range wb.Config.Settings.MLSuppressAccounts {
		mlSuppress[a] = true
	}

	for _, e := range entries {
		// 若分录所属父级在多科目明细账忽略列表中，跳过
		if mlSuppress[e.GeneralAccount] {
			continue
		}

		g, ok := groups[e.GeneralAccount]
		if !ok {
			g = &mlGroup{}
			groups[e.GeneralAccount] = g
		}
		g.entries = append(g.entries, e)
		if e.DetailAccount != "" {
			found := false
			for _, d := range g.details {
				if d == e.DetailAccount {
					found = true
					break
				}
			}
			if !found {
				g.details = append(g.details, e.DetailAccount)
			}
		}
	}

	for general, g := range groups {
		if len(g.details) == 0 {
			continue
		}
		detailOrder := wb.Config.DetailOrder[general]
		// 去重：防止历史配置中的重复项导致冲突检查误报
		if len(detailOrder) > 0 {
			seen := make(map[string]bool)
			var deduped []string
			for _, d := range detailOrder {
				if d != "" && !seen[d] {
					seen[d] = true
					deduped = append(deduped, d)
				}
			}
			if len(deduped) != len(detailOrder) {
				detailOrder = deduped
				wb.Config.DetailOrder[general] = deduped
			}
		}
		_, detailIdx, newAppended, err := wb.ensureMLSheet(general, g.details, detailOrder)
		if err != nil {
			return err
		}

		// 回写：新科目追加到 detailOrder，或首次引导从标题初始化
		needsWriteback := false
		if len(newAppended) > 0 {
			needsWriteback = true
		}
		if detailOrder == nil {
			needsWriteback = true
		}
		if needsWriteback {
			if wb.Config.DetailOrder == nil {
				wb.Config.DetailOrder = make(map[string][]string)
			}
			if detailOrder == nil {
				// 首次引导：从 Sheet 标题读取现有列序作为初始 detailOrder
				_, existingDetails, err := wb.readMLDetailHeaders(sheetNameML(general))
				if err == nil {
					for _, d := range existingDetails {
						if d != "" {
							detailOrder = append(detailOrder, d)
						}
					}
				}
			}
			merged := detailOrder
			for _, nd := range newAppended {
				found := false
				for _, d := range merged {
					if d == nd {
						found = true
						break
					}
				}
				if !found {
					merged = append(merged, nd)
				}
			}
			// 去重 merged
			seen := make(map[string]bool)
			var dedupedMerged []string
			for _, d := range merged {
				if d != "" && !seen[d] {
					seen[d] = true
					dedupedMerged = append(dedupedMerged, d)
				}
			}
			wb.Config.DetailOrder[general] = dedupedMerged
		}
		if err := wb.appendToMLSheet(general, g.entries, detailIdx, initials[general]); err != nil {
			return fmt.Errorf("多科目明细账 %s: %w", general, err)
		}
	}

	return nil
}

// appendToMLSheet 追加分录到指定总账科目的多科目明细账 Sheet。
// 逻辑页号：数据块两侧页码相同
// Paper1 Front 占位页已经由 ensureMLSheet 写入第1-5行。
// 数据页标题从第6行开始。
func (wb *Workbook) appendToMLSheet(general string, entries []voucher.Entry, detailIdx map[string]int, initial int64) error {
	sheet := sheetNameML(general)
	lay := mlLayout()
	numDetails := mlMaxDetails

	// 明细列实际科目名（空列留空），供每个页头写入
	reDetails := make([]string, mlMaxDetails)
	for name, idx := range detailIdx {
		reDetails[idx] = name
	}

	rows, _ := wb.File.GetRows(sheet)
	fdp := mlFirstDataPageStart()
	isNew := len(rows) < fdp || len(rows[fdp-1]) == 0

	// ── 计算逻辑页号──
	// logicalPageNum = 已有真实过次页数 + 1（跳过结构预写）
	logicalPageNum := 1
	for _, r := range rows {
		if len(r) > lay.BindingLeftCols+2 && r[lay.BindingLeftCols+2] == pageBreakLabel {
			debIdx := lay.BindingLeftCols + 3
			if len(r) > debIdx && strings.TrimSpace(r[debIdx]) != "" {
				logicalPageNum++
			}
		}
	}

	// ── 排序 ──
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Date != entries[j].Date {
			return entries[i].Date < entries[j].Date
		}
		return entries[i].VoucherNum < entries[j].VoucherNum
	})

	balance := initial
	var pageDebit, pageCredit int64
	pageDetails := make([]mlDetailTotals, numDetails)
	var row int

	if isNew {
		// Paper1 Front 已在 rows 1-5（ensureMLSheet 写入）
		// 写入第一个数据页标题：两侧同一逻辑页号
		wb.writeMLPageHeader(sheet, mlFirstDataPageStart(), logicalPageNum, logicalPageNum, general, true, true)
		// 页头写完后写入实际明细科目名（空列留空，无"明细N"占位）
		wb.writeMLDetailNamesAt(sheet, mlFirstDataPageStart(), reDetails)

		// 结转行在第38行（mlFirstDataPageStart + DataStartRow = 30 + 8）
		row = mlFirstDataPageStart() + lay.DataStartRow // = 38: 数据页header后承前页行
		cfLabel := carryForwardLabel
		if initial != 0 {
			cfLabel = "上年结转"
		}
		wb.writeMLCarryForwardRow(sheet, row, initial, 0, 0, make([]mlDetailTotals, numDetails), cfLabel)
		row++ // = 12：第一条分录
		// preWrite removed — 由 break handler 负责
	} else {
		// 已有数据 — 找到下一个可用数据行
		var err error
		row, err = wb.mlNextDataRow(sheet)
		if err != nil {
			return err
		}
		balance = wb.mlLastPageBalance(sheet)
		// 打印标记延迟到 FinalizeMLPages 统一添加
	}

	for _, e := range entries {
		// 补承前页（上月遗留的孤立过次页）
		if wb.mlLastRowIsOrphanBreak(sheet) {
			pbDebit, pbCredit := wb.mlLastBreakTotals(sheet)
			pbDetails := wb.lastBreakDetailTotals(sheet)
			logicalPageNum++
			wb.writeMLPageHeader(sheet, row, logicalPageNum, logicalPageNum, general, true, true)
			wb.writeMLDetailNamesAt(sheet, row, reDetails)
			row += lay.DataStartRow
			wb.writeMLCarryForwardRow(sheet, row, balance, pbDebit, pbCredit, pbDetails, carryForwardLabel)
			row++
				pageDebit = 0
			pageCredit = 0
			pageDetails = make([]mlDetailTotals, numDetails)
		}

		// 页满 → 过次页 + 标题 + 承前页
		if wb.mlRowIsPageBreak(sheet, row) {
			wb.writeMLPageBreakRow(sheet, row, balance, pageDebit, pageCredit, pageDetails)
			row += 1 + lay.BottomMarginRows // 下边距 + 新页上边距
			logicalPageNum++
			wb.writeMLPageHeader(sheet, row, logicalPageNum, logicalPageNum, general, true, true)
			wb.writeMLDetailNamesAt(sheet, row, reDetails)
			row += lay.DataStartRow
			wb.writeMLCarryForwardRow(sheet, row, balance, pageDebit, pageCredit, pageDetails, carryForwardLabel)
			row++
				pageDebit = 0
			pageCredit = 0
			pageDetails = make([]mlDetailTotals, numDetails)
		}

		balance = balance + e.DebitCents - e.CreditCents
		pageDebit += e.DebitCents
		pageCredit += e.CreditCents

		dir, dispBal := directionFor(balance, 0)

		// 日期拆分：月/日
		dateParts := strings.Split(e.Date, "-")
		monthStr, dayStr := "", ""
		if len(dateParts) >= 3 {
			monthStr = dateParts[1]
			dayStr = dateParts[2]
		}

		wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffMonth, row), monthStr)
		wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffDay, row), dayStr)
		wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffVouChar, row), "记")
		wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffVouNum, row), e.VoucherNum)
		wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffSummary, row), e.Summary)
		wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffDebit, row), centsToYuan(e.DebitCents))
		wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffCredit, row), centsToYuan(e.CreditCents))
		wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffDir, row), dir)
		wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffBalance, row), centsToYuan(dispBal))

		wb.setMoneyStyle(sheet, row, lay.BackStartCol+mlOffDebit)
		wb.setMoneyStyle(sheet, row, lay.BackStartCol+mlOffCredit)
		wb.setMoneyStyle(sheet, row, lay.BackStartCol+mlOffBalance)

		if e.DetailAccount != "" {
			if idx, ok := detailIdx[e.DetailAccount]; ok {
				net := e.DebitCents - e.CreditCents
				col := mlDetailCol(lay, idx)
				wb.File.SetCellValue(sheet, mlCellName(col, row), centsToYuan(net))
				wb.setMoneyStyle(sheet, row, col)
				pageDetails[idx].debit += e.DebitCents
				pageDetails[idx].credit += e.CreditCents
			}
		}

		// 打印标记延迟到 FinalizeMLPages 统一添加
		row++
	}

	return nil
}

// writeMLPageBreakRow 写多科目明细账的"过次页"行，双面写入：
// Back 侧：基础列（日期=空、凭证号=空、摘要=过次页、借方/贷方合计数、方向、余额）+ 明细1~4净额
// Front 侧：明细5~14净额
func (wb *Workbook) writeMLPageBreakRow(sheet string, row int, balance int64, pageDebit, pageCredit int64, pageDetails []mlDetailTotals) {
	lay := mlLayout()
	dir, dispBal := directionFor(balance, 0)

	// Back 侧：基础列（月/日/字/号=空，摘要=过次页，借方/贷方合计数，方向，余额）
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffMonth, row), "")
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffDay, row), "")
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffVouChar, row), "")
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffVouNum, row), "")
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffSummary, row), pageBreakLabel)
	// 过次页标签红色加粗
	redStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: "CC0000", Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	wb.File.SetCellStyle(sheet, mlCellName(lay.BackStartCol+mlOffSummary, row), mlCellName(lay.BackStartCol+mlOffSummary, row), redStyle)
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffDebit, row), centsToYuan(pageDebit))
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffCredit, row), centsToYuan(pageCredit))
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffDir, row), dir)
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffBalance, row), centsToYuan(dispBal))

	wb.setMoneyStyle(sheet, row, lay.BackStartCol+mlOffDebit)
	wb.setMoneyStyle(sheet, row, lay.BackStartCol+mlOffCredit)
	wb.setMoneyStyle(sheet, row, lay.BackStartCol+mlOffBalance)

	// Back 侧：明细1~4 净额
	for i := 0; i < 4 && i < len(pageDetails); i++ {
		net := pageDetails[i].debit - pageDetails[i].credit
		col := mlDetailCol(lay, i)
		wb.File.SetCellValue(sheet, mlCellName(col, row), centsToYuan(net))
		wb.setMoneyStyle(sheet, row, col)
	}

	// Front 侧：明细5~14 净额
	for i := 4; i < len(pageDetails); i++ {
		net := pageDetails[i].debit - pageDetails[i].credit
		col := mlDetailCol(lay, i)
		wb.File.SetCellValue(sheet, mlCellName(col, row), centsToYuan(net))
		wb.setMoneyStyle(sheet, row, col)
	}

}
// writeMLCarryForwardRow 写多科目明细账的"承前页"行，双面写入（结构与过次页相同，标签可定制）。

// 翻页触发时 writeMLPageBreakRow 会覆盖为完整数据。
func (wb *Workbook) writeMLCarryForwardRow(sheet string, row int, balance int64, pageDebit, pageCredit int64, pageDetails []mlDetailTotals, label string) {
	lay := mlLayout()
	dir, dispBal := directionFor(balance, 0)

	// Back 侧：基础列（月/日/字/号=空，摘要=label，借方/贷方合计数，方向，余额）
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffMonth, row), "")
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffDay, row), "")
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffVouChar, row), "")
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffVouNum, row), "")
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffSummary, row), label)
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffDebit, row), centsToYuan(pageDebit))
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffCredit, row), centsToYuan(pageCredit))
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffDir, row), dir)
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffBalance, row), centsToYuan(dispBal))

	wb.setMoneyStyle(sheet, row, lay.BackStartCol+mlOffDebit)
	wb.setMoneyStyle(sheet, row, lay.BackStartCol+mlOffCredit)
	wb.setMoneyStyle(sheet, row, lay.BackStartCol+mlOffBalance)

	// Back 侧：明细1~4 净额
	for i := 0; i < 4 && i < len(pageDetails); i++ {
		net := pageDetails[i].debit - pageDetails[i].credit
		col := mlDetailCol(lay, i)
		wb.File.SetCellValue(sheet, mlCellName(col, row), centsToYuan(net))
		wb.setMoneyStyle(sheet, row, col)
	}

	// Front 侧：明细5~14 净额
	for i := 4; i < len(pageDetails); i++ {
		net := pageDetails[i].debit - pageDetails[i].credit
		col := mlDetailCol(lay, i)
		wb.File.SetCellValue(sheet, mlCellName(col, row), centsToYuan(net))
		wb.setMoneyStyle(sheet, row, col)
	}

// 翻页触发时 writeMLPageBreakRow 覆盖为完整数据；不触发时红色文字作为模板结构保留。
}


// writeMLPageNumLayout 写"分第 n 页(suffix)"，三段分别占指定列区间：
//   分第（右对齐）| n（居中，绿色虚线下划线）| 页(suffix)（左对齐）
// num<=0 时 n 位置仅绿色虚线下划线（无数字），分第/页 绿字保留（空白表占位）。
func (wb *Workbook) writeMLPageNumLayout(sheet string, row, num int, suffix string,
	fenDiStart, fenDiSpan, nStart, nSpan, sufStart, sufSpan int) {
	darkGreen := "006100"
	sealRed := "CC0000"
	l := mlCellName(fenDiStart, row)
	r := mlCellName(fenDiStart+fenDiSpan-1, row)
	wb.File.MergeCell(sheet, l, r)
	wb.File.SetCellValue(sheet, l, "分第 ")
	s1, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: darkGreen, Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "right", Vertical: "bottom"},
	})
	wb.File.SetCellStyle(sheet, l, r, s1)

	n1 := mlCellName(nStart, row)
	n2 := mlCellName(nStart+nSpan-1, row)
	wb.File.MergeCell(sheet, n1, n2)
	if num > 0 {
		wb.File.SetCellValue(sheet, n1, fmt.Sprintf("%d", num))
	}
	s2, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: sealRed, Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "bottom"},
		Border:    []excelize.Border{{Type: "bottom", Color: darkGreen, Style: 4}},
	})
	wb.File.SetCellStyle(sheet, n1, n2, s2)

	s3 := mlCellName(sufStart, row)
	s4 := mlCellName(sufStart+sufSpan-1, row)
	wb.File.MergeCell(sheet, s3, s4)
	wb.File.SetCellValue(sheet, s3, " 页"+suffix)
	s5, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: darkGreen, Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "bottom"},
	})
	wb.File.SetCellStyle(sheet, s3, s4, s5)
}

// writeMLPageHeader 写入多科目明细账后续页双面标题行（过次页之后、承前页之前调用）。
// 写入 Back 和/或 Front 两侧的标题、页码、科目名、列标题。
// hasBack/hasFront 控制是否写对应侧；backPageNum/frontPageNum 分别是两侧的"分第N页"。
// row 是该页 block 的起始行（上边距行）。
// 行结构（9 行）：
//   Row +0: 上边距行（空）
//   Row +1: 分第 n 页 — Back 侧标题区 + Front 侧标题区
//   Row +2: 多科目明细账 — XXX（Back 侧标题区）| 科目名（Front 侧标题区，印章红）
//   Row +3: 科目名 — Back 侧 + Front 侧科目区（印章红）
//   Row +4: [空行]
//   Row +5~8: 列标题 — Back 侧（7基础列 + 明细1~4）| Front 侧（明细5~14）
func (wb *Workbook) writeMLPageHeader(sheet string, row int, backPageNum, frontPageNum int, general string, hasBack, hasFront bool) error {
	lay := mlLayout()
	darkGreen := "006100"
	sealRed := "CC0000"

	// Row +0: 上边距行（空，高 28）
	wb.File.SetRowHeight(sheet, row, 28)
	row++

	// Row +1: 标题行 — 分第N页(左) 在反面页左边；正面页 明 细 帐 + 分第N页(右)
	if hasBack {
		// Back 标题（空，下双线边框保留）
		dbStyle, _ := wb.File.NewStyle(&excelize.Style{
			Font:      &excelize.Font{Bold: true, Size: 14, Color: darkGreen},
			Border:    []excelize.Border{{Type: "bottom", Color: darkGreen, Style: 6}},
			Alignment: &excelize.Alignment{Horizontal: "right", Vertical: "bottom"},
		})
		// 明细3~4列逐列设双线边框
		for i := 2; i < 4; i++ {
			cell := mlCellName(mlDetailCol(lay, i), row)
			wb.File.SetCellStyle(sheet, cell, cell, dbStyle)
		}
		// 分第N页(左) — 分第 占两列(如年C-D)、n 占两列(如凭证E-F)、页(左) 对齐摘要列G
		wb.writeMLPageNumLayout(sheet, row, backPageNum, "(左)",
			lay.BackStartCol, 2, lay.BackStartCol+2, 2, lay.BackStartCol+4, 1)
	}

	if hasFront {
		// Front 标题 "明      细      帐"
		fcell := mlCellName(mlDetailCol(lay, 4), row)
		wb.File.SetCellValue(sheet, fcell, "明      细      帐")
		frontTitleStyle, _ := wb.File.NewStyle(&excelize.Style{
			Font:      &excelize.Font{Bold: true, Size: 14, Color: "006100"},
			Border:    []excelize.Border{{Type: "bottom", Color: "006100", Style: 6}},
			Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "bottom"},
		})
		// 明细5~7列逐列设双线边框
		for i := 4; i <= 6; i++ {
			cell := mlCellName(mlDetailCol(lay, i), row)
			wb.File.SetCellStyle(sheet, cell, cell, frontTitleStyle)
		}
		// 分第N页(右) — 分第 对齐明细12(X)、n 对齐明细13(Y)、页(右) 对齐明细14(Z)
		wb.writeMLPageNumLayout(sheet, row, frontPageNum, "(右)",
			mlDetailCol(lay, 11), 1, mlDetailCol(lay, 12), 1, mlDetailCol(lay, 13), 1)
	}
	wb.File.SetRowHeight(sheet, row, 28)
	row++

	// Row +2: 科目行 — 反面不展示科目字样；正面 "会计科目" 对齐明细12列 + 科目名
	isPaper1 := backPageNum == 0 && frontPageNum == 0
	detail12Col := mlDetailCol(lay, 11) // 明细12 = X
	if hasFront {
		label := mlCellName(detail12Col, row)
		wb.File.SetCellValue(sheet, label, "会计科目")
		labelStyle, _ := wb.File.NewStyle(&excelize.Style{
			Font:      &excelize.Font{Color: darkGreen, Size: 10},
			Alignment: &excelize.Alignment{Horizontal: "right", Vertical: "bottom"},
		})
		wb.File.SetCellStyle(sheet, label, label, labelStyle)
		nameLeft := mlCellName(detail12Col+1, row)
		nameRight := mlCellName(detail12Col+2, row)
		wb.File.MergeCell(sheet, nameLeft, nameRight)
		if !isPaper1 {
			wb.File.SetCellValue(sheet, nameLeft, general)
		}
		nameStyle, _ := wb.File.NewStyle(&excelize.Style{
			Font:      &excelize.Font{Color: sealRed, Size: 10},
			Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "bottom"},
			Border:    []excelize.Border{{Type: "bottom", Color: darkGreen, Style: 4}},
		})
		wb.File.SetCellStyle(sheet, nameLeft, nameRight, nameStyle)
	}
	wb.File.SetRowHeight(sheet, row, 18)
	row++

	// Row +3: 空行
	row++

	// Row +4 ~ Row +7: 四行表头（行高比 2.5:1:1:2.5，总高与旧两行一致 = 30pt）
	year := wb.Month[:4]
	h1, h2, h3, h4 := row, row+1, row+2, row+3

	headerBorders := []excelize.Border{
		{Type: "top", Color: "#808080", Style: 1},
		{Type: "bottom", Color: "#808080", Style: 1},
		{Type: "left", Color: "#808080", Style: 1},
		{Type: "right", Color: "#808080", Style: 1},
	}
	// 主标签样式（粗体深绿、四边灰细框、居中）
	gridStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 10, Color: darkGreen},
		Border:    headerBorders,
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	// 子表头样式（月/日/字/号、明细科目名：小字号，同四边灰细框）
	subHeadStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 9, Color: darkGreen},
		Border:    headerBorders,
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})

	// 1) 年|凭证：上两行合并；月|日|字|号：下两行合并
	// 2) 摘要：四行合并
	// 3) 借方/贷方/余额：上三行合并，两字间隔 12 空格
	// 4) 方向：四行合并，自动换行居中
	// 6) 明细科目们：中两行合并
	if hasBack {
		wb.File.MergeCell(sheet, mlCellName(lay.BackStartCol, h1), mlCellName(lay.BackStartCol+1, h2))
		// 年份数字红色，"年"绿色（GL 风格）
		wb.File.SetCellRichText(sheet, mlCellName(lay.BackStartCol, h1), []excelize.RichTextRun{
			{Text: year, Font: &excelize.Font{Bold: true, Size: 10, Color: "CC0000"}},
			{Text: "年", Font: &excelize.Font{Bold: true, Size: 10, Color: "006100"}},
		})
		wb.File.MergeCell(sheet, mlCellName(lay.BackStartCol+2, h1), mlCellName(lay.BackStartCol+3, h2))
		wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+2, h1), "凭证")
		for i, h := range []string{"月", "日", "字", "号"} {
			wb.File.MergeCell(sheet, mlCellName(lay.BackStartCol+i, h3), mlCellName(lay.BackStartCol+i, h4))
			wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+i, h3), h)
		}
		wb.File.MergeCell(sheet, mlCellName(lay.BackStartCol+mlOffSummary, h1), mlCellName(lay.BackStartCol+mlOffSummary, h4))
		wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffSummary, h1), "摘要")
		wb.File.MergeCell(sheet, mlCellName(lay.BackStartCol+mlOffDebit, h1), mlCellName(lay.BackStartCol+mlOffDebit, h3))
		wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffDebit, h1), "借            方")
		wb.File.MergeCell(sheet, mlCellName(lay.BackStartCol+mlOffCredit, h1), mlCellName(lay.BackStartCol+mlOffCredit, h3))
		wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffCredit, h1), "贷            方")
		wb.File.MergeCell(sheet, mlCellName(lay.BackStartCol+mlOffDir, h1), mlCellName(lay.BackStartCol+mlOffDir, h4))
		wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffDir, h1), "借或贷")
		wb.File.MergeCell(sheet, mlCellName(lay.BackStartCol+mlOffBalance, h1), mlCellName(lay.BackStartCol+mlOffBalance, h3))
		wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+mlOffBalance, h1), "余            额")
	}

	// 6) 明细科目们：中两行合并（科目名由调用方 writeMLDetailNamesAt 写入，空列留空，无"明细N"占位）
	for i := 0; i < mlMaxDetails; i++ {
		if (i < 4 && !hasBack) || (i >= 4 && !hasFront) {
			continue
		}
		col := mlDetailCol(lay, i)
		wb.File.MergeCell(sheet, mlCellName(col, h2), mlCellName(col, h3))
	}

	// 明细科目上方的行：左4列合并为"( ) 方 金"，右10列合并为"额 分 析"（金额分析表头）
	if hasBack {
		wb.File.MergeCell(sheet, mlCellName(mlDetailCol(lay, 0), h1), mlCellName(mlDetailCol(lay, 3), h1))
		wb.File.SetCellValue(sheet, mlCellName(mlDetailCol(lay, 0), h1),
			"("+strings.Repeat(" ", 10)+")"+strings.Repeat(" ", 20)+"方"+strings.Repeat(" ", 20)+"金")
	}
	if hasFront {
		wb.File.MergeCell(sheet, mlCellName(mlDetailCol(lay, 4), h1), mlCellName(mlDetailCol(lay, mlMaxDetails-1), h1))
		wb.File.SetCellValue(sheet, mlCellName(mlDetailCol(lay, 4), h1),
			"额"+strings.Repeat(" ", 100)+"分"+strings.Repeat(" ", 100)+"析")
	}

	// 整块先铺网格样式，再对子表头/方向覆盖
	if hasBack {
		wb.File.SetCellStyle(sheet, mlCellName(lay.BackStartCol, h1), mlCellName(lay.BackStartCol+mlOffBalance, h4), gridStyle)
	}
	for i := 0; i < mlMaxDetails; i++ {
		if (i < 4 && !hasBack) || (i >= 4 && !hasFront) {
			continue
		}
		col := mlDetailCol(lay, i)
		wb.File.SetCellStyle(sheet, mlCellName(col, h1), mlCellName(col, h4), gridStyle)
	}
	// 方向：自动换行居中
	dirStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 10, Color: darkGreen},
		Border:    headerBorders,
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
	})
	if hasBack {
		wb.File.SetCellStyle(sheet, mlCellName(lay.BackStartCol+mlOffDir, h1), mlCellName(lay.BackStartCol+mlOffDir, h1), dirStyle)
	}
	// 月/日/字/号、明细科目名：小字号
	if hasBack {
		wb.File.SetCellStyle(sheet, mlCellName(lay.BackStartCol, h3), mlCellName(lay.BackStartCol+3, h3), subHeadStyle)
	}
	for i := 0; i < mlMaxDetails; i++ {
		if (i < 4 && !hasBack) || (i >= 4 && !hasFront) {
			continue
		}
		col := mlDetailCol(lay, i)
		wb.File.SetCellStyle(sheet, mlCellName(col, h2), mlCellName(col, h2), subHeadStyle)
	}

	// 行高：2.5:1:1:2.5，总高 56pt（单行高 8pt）
	const headerTotalHeight = 56.0
	hu := headerTotalHeight / 7.0
	wb.File.SetRowHeight(sheet, h1, hu*2.5)
	wb.File.SetRowHeight(sheet, h2, hu)
	wb.File.SetRowHeight(sheet, h3, hu)
	wb.File.SetRowHeight(sheet, h4, hu*2.5)

	return nil
}

// lastBreakDetailTotals 读取最后一个过次页行的各明细列净额。
func (wb *Workbook) lastBreakDetailTotals(sheet string) []mlDetailTotals {
	lay := mlLayout()
	rows, err := wb.File.GetRows(sheet)
	if err != nil {
		return make([]mlDetailTotals, mlMaxDetails)
	}
	for i := len(rows) - 1; i >= 0; i-- {
		if mlHasPageBreakAt(rows[i], lay) && !mlIsStructuralBreak(rows[i], lay) {
			result := make([]mlDetailTotals, mlMaxDetails)
			for j := 0; j < mlMaxDetails; j++ {
				colIdx := mlDetailRowIdx(lay, j) // GetRows 索引：j<4→BindingLeftCols+7+j(Back侧), j>=4→FrontStartCol-1+(j-4)(Front侧)
				if colIdx < len(rows[i]) {
					if v, err := yuanStrToCents(rows[i][colIdx]); err == nil {
						if v >= 0 {
							result[j].debit = v
						} else {
							result[j].credit = -v
						}
					}
				}
			}
			return result
		}
	}
	return make([]mlDetailTotals, mlMaxDetails)
}
