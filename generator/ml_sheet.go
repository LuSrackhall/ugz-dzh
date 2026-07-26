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
)

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

// mlDetailCol 返回第 i 个明细列的 Excel 列号。
// i=0~3 → Back 侧（左半），i=4~13 → Front 侧（右半）。
func mlDetailCol(lay layout.MLLayout, i int) int {
	if i < 4 {
		return lay.BackStartCol + 7 + i
	}
	return lay.FrontStartCol + (i - 4)
}

// mlDetailRowIdx 返回第 i 个明细列在 GetRows 中的索引。
// i=0~3 → Back 侧 GetRows 索引 = BindingLeftCols + 7 + i，
// i=4~13 → Front 侧 GetRows 索引 = FrontStartCol - 1 + (i - 4)。
func mlDetailRowIdx(lay layout.MLLayout, i int) int {
	if i < 4 {
		return lay.BindingLeftCols + 7 + i
	}
	return lay.FrontStartCol - 1 + (i - 4)
}

// ── ML 独立辅助函数（与 GL 同名函数功能相同但使用 mlLayout） ──

// mlCellName 返回 Excel 单元格名（与 cellName 相同功能，ML 独立版本）。
func mlCellName(col, row int) string {
	name, _ := excelize.CoordinatesToCellName(col, row)
	return name
}

// mlHasPageBreakAt 检查行中是否有"过次页"标记（仅 Back 侧摘要列）。
func mlHasPageBreakAt(row []string, lay layout.MLLayout) bool {
	summaryIdx := lay.BindingLeftCols + 2
	return len(row) > summaryIdx && row[summaryIdx] == pageBreakLabel
}

// mlIsStructuralBreak 检查过次页行是否为结构预写（余额列为空字符串，无翻页数据）。
// 注意：借方为 "0" 的过次页可能是真实翻页（收入类科目无借方），用余额列判断更可靠。
func mlIsStructuralBreak(row []string, lay layout.MLLayout) bool {
	balIdx := lay.BindingLeftCols + 6
	return len(row) <= balIdx ||
		strings.TrimSpace(row[balIdx]) == ""
}

// mlLastPeriodEndRow 从 Sheet 中找到最后一个"期末余额"行的行号（1-based）。
// 期末余额永远是月结的最后一行，用它定位本月数据尾行，无需额外标记列。
func (wb *Workbook) mlLastPeriodEndRow(sheet string) int {
	rows, _ := wb.File.GetRows(sheet)
	for i := len(rows) - 1; i >= 0; i-- {
		if len(rows[i]) > 4 && rows[i][4] == periodEndLabel {
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
		// 只跳过结构过次页
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
		return lay.DataStartRow + 1, nil
	}

	lastDataIdx := mlLastDataBeforeBreak(rows, lay)
	if lastDataIdx < 0 {
		return lay.DataStartRow + 1, nil
	}

	// 如果最后一条数据是真实过次页 → 新页面，承前页在 DataStartRow 后
	if mlHasPageBreakAt(rows[lastDataIdx], lay) && !mlIsStructuralBreak(rows[lastDataIdx], lay) {
		return lastDataIdx + 2 + lay.DataStartRow, nil
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
			if len(r) <= lay.BindingLeftCols+6 {
				continue
			}
			summary := ""
			if len(r) > lay.BindingLeftCols+2 {
				summary = r[lay.BindingLeftCols+2]
			}
			// 跳过承前页行、月结行
			if summary == carryForwardLabel || summary == "上年结转" ||
				summary == "本月合计" || summary == "本季合计" || summary == "本年累计" || summary == periodEndLabel {
				continue
			}
			balStr := strings.TrimSpace(r[lay.BindingLeftCols+6])
			if balStr == "" {
				continue
			}
			if v, err := yuanStrToCents(balStr); err == nil {
				return v
			}
		}
		// 回退到过次页行自身余额
		if len(rows[lastBreak]) > lay.BindingLeftCols+6 {
			if v, err := yuanStrToCents(rows[lastBreak][lay.BindingLeftCols+6]); err == nil {
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
		if len(r) > lay.BindingLeftCols+2 &&
			(r[lay.BindingLeftCols+2] == carryForwardLabel || r[lay.BindingLeftCols+2] == "上年结转") {
			continue
		}
		balIdx := lay.BindingLeftCols + 6
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
		debIdx := lay.BindingLeftCols + 3
		crdIdx := lay.BindingLeftCols + 4
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
	debIdx := lay.BindingLeftCols + 3
	if len(last) <= debIdx || strings.TrimSpace(last[debIdx]) == "" {
		return false
	}
	return true
}

// (wb *Workbook) mlPageStartRow 返回当前页第一个有效数据行的行号。
// 只找真实过次页（有数据的），跳过结构过次页（模板占位）。
func (wb *Workbook) mlPageStartRow(sheet string) int {
	lay := mlLayout()
	rows, err := wb.File.GetRows(sheet)
	if err != nil || len(rows) < 3 {
		return lay.DataStartRow + 1 + lay.DataStartRow
	}
	for i := len(rows) - 1; i >= 0; i-- {
		if mlHasPageBreakAt(rows[i], lay) && !mlIsStructuralBreak(rows[i], lay) {
			return i + 2 + lay.DataStartRow
		}
	}
	return lay.DataStartRow + 1 + lay.DataStartRow
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
		return lay.DataStartRow + 1, nil
	}

	tailRow := wb.mlLastPeriodEndRow(sheet)
	if tailRow > 0 {
		// 检查尾行之后是否有真实过次页
		for i := tailRow; i < len(rows); i++ {
			if mlHasPageBreakAt(rows[i], lay) && !mlIsStructuralBreak(rows[i], lay) {
				return i + 2 + lay.DataStartRow, nil
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

		// 更新标题行（仅更新新增的列，Paper1 Front 表头在第5行）
			lay := mlLayout()
			for _, nd := range newAppended {
				col := mlDetailCol(lay, finalIdx[nd])
				cell := mlCellName(col, 5)
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

	// Paper1 Front 占位页：Front 侧，页码=1，不写 Back 侧
	lay := mlLayout()
	if err := wb.writeMLPageHeader(name, 1, 0, 0, general, false, true); err != nil {
		return "", nil, nil, err
	}
	// Paper1 Front 表头行写入实际明细科目名（数据页列标题行）
	colHeaderRow := 6 + lay.DataStartRow - 1 // 1-based
	for i := 0; i < mlMaxDetails; i++ {
		col := mlDetailCol(lay, i)
		cell := mlCellName(col, colHeaderRow)
		label := ""
		if i < len(initDetails) {
			label = initDetails[i]
		}
		wb.File.SetCellValue(name, cell, label)
	}
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

// checkMLDetailOrderConflict 逐列比对第2行标题与 detailOrder 配置。
func (wb *Workbook) checkMLDetailOrderConflict(sheet string, existingDetails []string, detailOrder []string) error {
	configIdx := 0
	for colIdx := 0; colIdx < mlMaxDetails && configIdx < len(detailOrder); colIdx++ {
		existing := existingDetails[colIdx]
		configured := detailOrder[configIdx]

		if configured == "" {
			if existing != "" {
				return fmt.Errorf("Sheet %s: detailOrder 与现有列序冲突 — 第 %d 列配置为空但实际为 %q。请使用 -f 从首月重新生成", sheet, colIdx+1, existing)
			}
			configIdx++
			continue
		}

		if existing == "" {
			found := false
			for j := colIdx + 1; j < mlMaxDetails; j++ {
				if existingDetails[j] == configured {
					found = true
					break
				}
			}
			if found {
				return fmt.Errorf("Sheet %s: detailOrder 与现有列序冲突 — %q 配置在第 %d 列但实际在更右侧。请使用 -f 从首月重新生成", sheet, configured, configIdx+1)
			}
			configIdx++
			continue
		}

		if existing != configured {
			return fmt.Errorf("Sheet %s: detailOrder 与现有列序冲突 — 第 %d 列配置为 %q 但实际为 %q。请使用 -f 从首月重新生成", sheet, configIdx+1, configured, existing)
		}
		configIdx++
	}
	return nil
}


// cellColLetter 返回列号的字母表示。
func cellColLetter(col int) string {
	l, _ := excelize.ColumnNumberToName(col)
	return l
}

// updateMLDetailHeaders 更新已有 Sheet 的明细列标题（数据页列标题行），以匹配当月明细科目集。
func (wb *Workbook) updateMLDetailHeaders(sheet string, details []string) {
	lay := mlLayout()
	colHeaderRow := 6 + lay.DataStartRow - 1 // 1-based
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

// readMLDetailHeaders 从 Sheet 的数据页列标题行读取现有明细列标题。
// 使用 Layout 的 HeaderRow + DataStartRow 定位列标题行。
func (wb *Workbook) readMLDetailHeaders(sheet string) (detailIdx map[string]int, details []string, err error) {
	lay := mlLayout()
	detailIdx = make(map[string]int)
	details = make([]string, mlMaxDetails)

	rows, err := wb.File.GetRows(sheet)
	if err != nil {
		return nil, nil, fmt.Errorf("读取 Sheet %s: %w", sheet, err)
	}

	// 数据页列标题行：DataStartRow 是相对于页起始的偏移，页起始=6（Paper1 Front）
	// 列标题在 6 + lay.DataStartRow - 1 = 6 + 6 - 1 = 11 (1-based) = index 10
	colHeaderRow := 6 + lay.DataStartRow - 1 // 1-based row number
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
			wb.Config.DetailOrder[general] = merged
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

	rows, _ := wb.File.GetRows(sheet)
	isNew := len(rows) <= 5

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
		wb.writeMLPageHeader(sheet, 6, logicalPageNum, logicalPageNum, general, true, true)

		// 结转行在第11行（6 + DataStartRow = 6 + 5）
		row = 6 + lay.DataStartRow // = 11
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
			row++
			logicalPageNum++
			wb.writeMLPageHeader(sheet, row, logicalPageNum, logicalPageNum, general, true, true)
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

		wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+0, row), e.Date)
		wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+1, row), e.VoucherNum)
		wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+2, row), e.Summary)
		wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+3, row), centsToYuan(e.DebitCents))
		wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+4, row), centsToYuan(e.CreditCents))
		wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+5, row), dir)
		wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+6, row), centsToYuan(dispBal))

		wb.setMoneyStyle(sheet, row, lay.BackStartCol+3)
		wb.setMoneyStyle(sheet, row, lay.BackStartCol+4)
		wb.setMoneyStyle(sheet, row, lay.BackStartCol+6)

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

	// Back 侧：基础列合计
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+0, row), "")
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+1, row), "")
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+2, row), pageBreakLabel)
	// 过次页标签红色加粗
	redStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font: &excelize.Font{Color: "CC0000", Size: 10, Bold: true},
	})
	wb.File.SetCellStyle(sheet, mlCellName(lay.BackStartCol+2, row), mlCellName(lay.BackStartCol+2, row), redStyle)
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+3, row), centsToYuan(pageDebit))
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+4, row), centsToYuan(pageCredit))
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+5, row), dir)
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+6, row), centsToYuan(dispBal))

	wb.setMoneyStyle(sheet, row, lay.BackStartCol+3)
	wb.setMoneyStyle(sheet, row, lay.BackStartCol+4)
	wb.setMoneyStyle(sheet, row, lay.BackStartCol+6)

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

	// Back 侧：基础列合计
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+0, row), "")
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+1, row), "")
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+2, row), label)
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+3, row), centsToYuan(pageDebit))
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+4, row), centsToYuan(pageCredit))
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+5, row), dir)
	wb.File.SetCellValue(sheet, mlCellName(lay.BackStartCol+6, row), centsToYuan(dispBal))

	wb.setMoneyStyle(sheet, row, lay.BackStartCol+3)
	wb.setMoneyStyle(sheet, row, lay.BackStartCol+4)
	wb.setMoneyStyle(sheet, row, lay.BackStartCol+6)

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


// writeMLPageHeader 写入多科目明细账后续页双面标题行（过次页之后、承前页之前调用）。
// 写入 Back 和/或 Front 两侧的标题、页码、科目名、列标题。
// hasBack/hasFront 控制是否写对应侧；backPageNum/frontPageNum 分别是两侧的"分第N页"。
// 行结构（5 行）：
//   Row +0: 分第 n 页 — Back 侧标题区 + Front 侧标题区
//   Row +1: 多科目明细账 — XXX（Back 侧标题区）| 科目名（Front 侧标题区，印章红）
//   Row +2: 科目名 — Back 侧 + Front 侧科目区（印章红）
//   Row +3: [空行]
//   Row +4: 列标题 — Back 侧（年|凭证|摘要|借方|贷方|方向|余额 + 明细1~4）| Front 侧（明细5~14）
//   Row +5: 子表头 — Back 侧（月|日|字|号，仅前4列有内容）
func (wb *Workbook) writeMLPageHeader(sheet string, row int, backPageNum, frontPageNum int, general string, hasBack, hasFront bool) error {
	lay := mlLayout()
	darkGreen := "006100"
	sealRed := "CC0000"

	// Row N+0: 分第 n 页（右侧，绿色，数字印章红）
	pnLeft := mlCellName(lay.BackAccountColLeft, row)
	pnRight := mlCellName(lay.BackAccountColRight, row)
	wb.File.MergeCell(sheet, pnLeft, pnRight)
	wb.File.SetCellRichText(sheet, pnLeft, []excelize.RichTextRun{
		{Text: "分第 ", Font: &excelize.Font{Color: darkGreen, Size: 10}},
		{Text: fmt.Sprintf("%d", pageNum), Font: &excelize.Font{Color: sealRed, Size: 10}},
		{Text: " 页", Font: &excelize.Font{Color: darkGreen, Size: 10}},
	})
	wb.File.SetRowHeight(sheet, row, 18)
	row++

	// Row N+1: 多科目明细账 — 科目名（居中）+ 科目名称（右侧）
	tl := mlCellName(lay.BackTitleColLeft, row)
	tr := mlCellName(lay.BackTitleColRight, row)
	wb.File.MergeCell(sheet, tl, tr)
	wb.File.SetCellValue(sheet, tl, "多科目明细账 — "+general)
	titleStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 14, Color: darkGreen, Underline: "double"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	wb.File.SetCellStyle(sheet, tl, tr, titleStyle)

	al := mlCellName(lay.BackAccountColLeft, row)
	ar := mlCellName(lay.BackAccountColRight, row)
	wb.File.MergeCell(sheet, al, ar)
	wb.File.SetCellValue(sheet, al, general)
	accStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: sealRed, Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	wb.File.SetCellStyle(sheet, al, ar, accStyle)
	wb.File.SetRowHeight(sheet, row, 28)
	row++

	// Row N+2: 科目名称（右侧，印章红）
	acLeft := mlCellName(lay.BackAccountColLeft, row)
	acRight := mlCellName(lay.BackAccountColRight, row)
	wb.File.MergeCell(sheet, acLeft, acRight)
	wb.File.SetCellValue(sheet, acLeft, general)
	acRowStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: sealRed, Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	wb.File.SetCellStyle(sheet, acLeft, acRight, acRowStyle)
	wb.File.SetRowHeight(sheet, row, 18)
	row++

	// Row +3: 空行
	row++

	// Row +4: 列标题 — Back 侧（两行表头：年|凭证 合并，子表头 月|日|字|号）
	year := wb.Month[:4]
	if hasBack {
		// "年" 合并月+日两列
		yearLeft := mlCellName(lay.BackStartCol, row)
		yearRight := mlCellName(lay.BackStartCol+1, row)
		wb.File.MergeCell(sheet, yearLeft, yearRight)
		wb.File.SetCellValue(sheet, yearLeft, year+"年")
		// "凭证" 合并字+号两列
		vouchLeft := mlCellName(lay.BackStartCol+2, row)
		vouchRight := mlCellName(lay.BackStartCol+3, row)
		wb.File.MergeCell(sheet, vouchLeft, vouchRight)
		wb.File.SetCellValue(sheet, vouchLeft, "凭证")
		// 摘要 ~ 余额 各占单列
		otherCols := []string{"摘要", "借方金额", "贷方金额", "方向", "余额"}
		for i, h := range otherCols {
			cell := mlCellName(lay.BackStartCol+4+i, row)
			wb.File.SetCellValue(sheet, cell, h)
		}
		// Back 侧明细1~4 列标题
		for i := 0; i < 4 && i < mlMaxDetails; i++ {
			col := mlDetailCol(lay, i)
			cell := mlCellName(col, row)
			wb.File.SetCellValue(sheet, cell, fmt.Sprintf("明细%d", i+1))
		}
	}

	// Row +4: 列标题 — Front 侧（明细5~14）
	if hasFront {
		for i := 4; i < mlMaxDetails; i++ {
			col := mlDetailCol(lay, i)
			cell := mlCellName(col, row)
			wb.File.SetCellValue(sheet, cell, fmt.Sprintf("明细%d", i+1))
		}
	}

	headerStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 10, Color: darkGreen},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#D9E1F2"}, Pattern: 1},
		Border: []excelize.Border{{Type: "bottom", Color: "#808080", Style: 1}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	// Back 侧基础列表头样式（Row +4）
	if hasBack {
		hsBack := mlCellName(lay.BackStartCol, row)
		heBack := mlCellName(lay.BackStartCol+6, row)
		wb.File.SetCellStyle(sheet, hsBack, heBack, headerStyle)
		for i := 0; i < 4 && i < mlMaxDetails; i++ {
			cell := mlCellName(mlDetailCol(lay, i), row)
			wb.File.SetCellStyle(sheet, cell, cell, headerStyle)
		}
	}
	// Front 侧明细5~14 表头样式（Row +4）
	if hasFront {
		for i := 4; i < mlMaxDetails; i++ {
			cell := mlCellName(mlDetailCol(lay, i), row)
			wb.File.SetCellStyle(sheet, cell, cell, headerStyle)
		}
	}
	row++

	// Row +5: 子表头 — Back 侧前4列（月/日/字/号）
	if hasBack {
		subHeaders := []string{"月", "日", "字", "号"}
		for i, h := range subHeaders {
			cell := mlCellName(lay.BackStartCol+i, row)
			wb.File.SetCellValue(sheet, cell, h)
		}
		subStyle, _ := wb.File.NewStyle(&excelize.Style{
			Font: &excelize.Font{Size: 9, Color: darkGreen},
			Fill: excelize.Fill{Type: "pattern", Color: []string{"#D9E1F2"}, Pattern: 1},
			Border: []excelize.Border{{Type: "bottom", Color: "#808080", Style: 1}},
			Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		})
		subLeft := mlCellName(lay.BackStartCol, row)
		subRight := mlCellName(lay.BackStartCol+3, row)
		wb.File.SetCellStyle(sheet, subLeft, subRight, subStyle)
	}

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
