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
	mlMaxDetails    = 14 // 明细科目上限
	mlDetailStartCol = 8  // 明细列起始列 H（保留常量用于非 Layout 上下文）
)

// mlLayout 返回多科目明细账的布局规格（独立于 GL 布局）。
func mlLayout() layout.MLLayout {
	return layout.MLComputeLayout(layout.DefaultMLSpec())
}

// mlPrintMarkCol 返回多科目明细账打印标记列号（Layout 内容区末列）。
func mlPrintMarkCol() int {
	lay := mlLayout()
	return lay.FrontStartCol + 7 + mlMaxDetails
}

// mlDetailExcelCol 返回多科目明细账第 i 个明细列的 Excel 列号（Layout 坐标）。
func mlDetailExcelCol(lay layout.MLLayout, i int) int {
	return lay.FrontStartCol + 7 + i
}

// mlDetailRowIdx 返回多科目明细账第 i 个明细列在 GetRows 中的索引（Layout 坐标）。
func mlDetailRowIdx(lay layout.MLLayout, i int) int {
	return lay.BindingLeftCols + 7 + i
}

// ── ML 独立辅助函数（与 GL 同名函数功能相同但使用 mlLayout） ──

// mlCellName 返回 Excel 单元格名（与 cellName 相同功能，ML 独立版本）。
func mlCellName(col, row int) string {
	name, _ := excelize.CoordinatesToCellName(col, row)
	return name
}

// mlHasPageBreakAt 检查行中是否有"过次页"标记。
func mlHasPageBreakAt(row []string, lay layout.MLLayout) bool {
	return (len(row) > lay.BindingLeftCols+2 && row[lay.BindingLeftCols+2] == pageBreakLabel) ||
		(len(row) > lay.BackStartCol+1 && row[lay.BackStartCol+1] == pageBreakLabel)
}

// (wb *Workbook) mlNextDataRow 返回 Sheet 中下一个可用数据行号。
func (wb *Workbook) mlNextDataRow(sheet string) (int, error) {
	lay := mlLayout()
	rows, err := wb.File.GetRows(sheet)
	if err != nil {
		return 3, nil
	}
	if len(rows) < 3 {
		return 3, nil
	}
	lastBreak := 0
	for i := len(rows) - 1; i >= 0; i-- {
		if mlHasPageBreakAt(rows[i], lay) {
			lastBreak = i + 1
			break
		}
	}
	if lastBreak > 0 && lastBreak == len(rows) {
		return lastBreak + 1, nil
	}
	if lastBreak > 0 && lastBreak+1 == len(rows) {
		return len(rows) + 1, nil
	}
	dataStart := lastBreak + 1
	if dataStart == 1 {
		dataStart = 3
	}
	usedDataRows := len(rows) - dataStart + 1
	if usedDataRows >= pageSize {
		return len(rows) + 1, nil
	}
	return len(rows) + 1, nil
}

// (wb *Workbook) mlLastPageBalance 获取最后一个过次页行的余额。
func (wb *Workbook) mlLastPageBalance(sheet string) int64 {
	lay := mlLayout()
	rows, err := wb.File.GetRows(sheet)
	if err != nil {
		return 0
	}
	for i := len(rows) - 1; i >= 0; i-- {
		if !mlHasPageBreakAt(rows[i], lay) {
			continue
		}
		if len(rows[i]) > lay.BindingLeftCols+2 && rows[i][lay.BindingLeftCols+2] == pageBreakLabel {
			if len(rows[i]) >= lay.BindingLeftCols+7 {
				if v, err := yuanStrToCents(rows[i][lay.BindingLeftCols+6]); err == nil {
					return v
				}
			}
		}
		if len(rows[i]) > lay.BackStartCol+1 && rows[i][lay.BackStartCol+1] == pageBreakLabel {
			balIdx := lay.BackStartCol + 5
			if len(rows[i]) > balIdx {
				if v, err := yuanStrToCents(rows[i][balIdx]); err == nil {
					return v
				}
			}
		}
		return 0
	}
	return 0
}

// (wb *Workbook) mlLastBreakTotals 获取最后一个过次页行的页借贷合计。
func (wb *Workbook) mlLastBreakTotals(sheet string) (debit, credit int64) {
	lay := mlLayout()
	rows, err := wb.File.GetRows(sheet)
	if err != nil {
		return 0, 0
	}
	for i := len(rows) - 1; i >= 0; i-- {
		if !mlHasPageBreakAt(rows[i], lay) {
			continue
		}
		if len(rows[i]) > lay.BindingLeftCols+2 && rows[i][lay.BindingLeftCols+2] == pageBreakLabel {
			if len(rows[i]) >= lay.BindingLeftCols+5 {
				if v, err := yuanStrToCents(rows[i][lay.BindingLeftCols+3]); err == nil {
					debit = v
				}
				if v, err := yuanStrToCents(rows[i][lay.BindingLeftCols+4]); err == nil {
					credit = v
				}
			}
		}
		if len(rows[i]) > lay.BackStartCol+1 && rows[i][lay.BackStartCol+1] == pageBreakLabel {
			debIdx := lay.BackStartCol + 2
			crdIdx := lay.BackStartCol + 3
			if len(rows[i]) > crdIdx {
				if v, err := yuanStrToCents(rows[i][debIdx]); err == nil {
					debit = v
				}
				if v, err := yuanStrToCents(rows[i][crdIdx]); err == nil {
					credit = v
				}
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
	return mlHasPageBreakAt(last, lay)
}

// (wb *Workbook) mlPageStartRow 返回当前页第一个有效数据行的行号。
func (wb *Workbook) mlPageStartRow(sheet string) int {
	lay := mlLayout()
	rows, err := wb.File.GetRows(sheet)
	if err != nil || len(rows) < 3 {
		return 3
	}
	for i := len(rows) - 1; i >= 0; i-- {
		if mlHasPageBreakAt(rows[i], lay) {
			return i + 2 + lay.DataStartRow
		}
	}
	return lay.DataStartRow + 1
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
		if mlHasPageBreakAt(rows[i], lay) {
			return true
		}
	}
	return false
}

// (wb *Workbook) mlNextDataRowAfterBreak 返回过次页/月结等非数据行后的下一个可用数据行（ML 版）。
func (wb *Workbook) mlNextDataRowAfterBreak(sheet string) (int, error) {
	lay := mlLayout()
	rows, err := wb.File.GetRows(sheet)
	if err != nil {
		return 3, nil
	}
	if len(rows) < 3 {
		return 3, nil
	}
	lastBreak := 0
	for i := len(rows) - 1; i >= 0; i-- {
		r := rows[i]
		if mlHasPageBreakAt(r, lay) || (len(r) > lay.BindingLeftCols+2 &&
			(r[lay.BindingLeftCols+2] == "本月合计" || r[lay.BindingLeftCols+2] == periodEndLabel)) {
			lastBreak = i + 1
			break
		}
	}
	if lastBreak > 0 && lastBreak == len(rows) {
		return lastBreak + 1, nil
	}
	if lastBreak > 0 && lastBreak+1 == len(rows) {
		return len(rows) + 1, nil
	}
	return len(rows) + 1, nil
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

		// 更新标题行（仅更新新增的列）
			lay := mlLayout()
			for _, nd := range newAppended {
				col := mlDetailExcelCol(lay, finalIdx[nd])
				cell := mlCellName(col, 2)
				wb.File.SetCellValue(name, cell, nd)
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

	if err := wb.writeMLTitle(name, general, initDetails); err != nil {
		return "", nil, nil, err
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

// writeMLTitle 写入多科目明细账标题行和列标题，固定 7 基础列 + 14 明细列。
// 标题和列标题使用 Layout 坐标（基础列在 FrontStartCol+0~6，明细列在 FrontStartCol+7~20）。
func (wb *Workbook) writeMLTitle(sheet, general string, details []string) error {
	lay := mlLayout()
	nCols := 7 + mlMaxDetails                      // 总列数 = 基础列 + 明细列
	lastCol := lay.FrontStartCol + nCols                     // 最后数据列（不含打印标记）
	titleStart := mlCellName(lay.FrontStartCol, 1)
	titleEndCell, _ := excelize.CoordinatesToCellName(lastCol-1, 1)

	title := "多科目明细账 — " + general
	wb.File.SetCellValue(sheet, titleStart, title)
	wb.File.MergeCell(sheet, titleStart, titleEndCell)

	titleStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 14},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	wb.File.SetCellStyle(sheet, titleStart, titleEndCell, titleStyle)
	wb.File.SetRowHeight(sheet, 1, 22)

	// 基础列标题 FrontStartCol+0~6（日期/凭证号/摘要/借方/贷方/方向/余额）
	for i, h := range glHeaders {
		cell := mlCellName(lay.FrontStartCol+i, 2)
		wb.File.SetCellValue(sheet, cell, h)
	}
	// 明细列标题 FrontStartCol+7~20（H-U 对应）
	for i := 0; i < mlMaxDetails; i++ {
		col := mlDetailExcelCol(lay, i)
		cell := mlCellName(col, 2)
		label := ""
		if i < len(details) {
			label = details[i]
		}
		wb.File.SetCellValue(sheet, cell, label)
	}

	headerStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 10},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#D9E1F2"}, Pattern: 1},
		Border: []excelize.Border{
			{Type: "bottom", Color: "#808080", Style: 1},
		},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	headerEndCell, _ := excelize.CoordinatesToCellName(lastCol-1, 2)
	wb.File.SetCellStyle(sheet, titleStart, headerEndCell, headerStyle)

	// 基础列宽
	wb.File.SetColWidth(sheet, cellColLetter(lay.FrontStartCol+0), cellColLetter(lay.FrontStartCol+0), 12)
	wb.File.SetColWidth(sheet, cellColLetter(lay.FrontStartCol+1), cellColLetter(lay.FrontStartCol+1), 8)
	wb.File.SetColWidth(sheet, cellColLetter(lay.FrontStartCol+2), cellColLetter(lay.FrontStartCol+2), 35)
	wb.File.SetColWidth(sheet, cellColLetter(lay.FrontStartCol+3), cellColLetter(lay.FrontStartCol+3), 14)
	wb.File.SetColWidth(sheet, cellColLetter(lay.FrontStartCol+4), cellColLetter(lay.FrontStartCol+4), 14)
	wb.File.SetColWidth(sheet, cellColLetter(lay.FrontStartCol+5), cellColLetter(lay.FrontStartCol+5), 6)
	wb.File.SetColWidth(sheet, cellColLetter(lay.FrontStartCol+6), cellColLetter(lay.FrontStartCol+6), 16)
	// 明细列宽
	for i := 0; i < mlMaxDetails; i++ {
		colLetter := cellColLetter(mlDetailExcelCol(lay, i))
		wb.File.SetColWidth(sheet, colLetter, colLetter, 14)
	}

	return nil
}

// cellColLetter 返回列号的字母表示。
func cellColLetter(col int) string {
	l, _ := excelize.ColumnNumberToName(col)
	return l
}

// updateMLDetailHeaders 更新已有 Sheet 的明细列标题，以匹配当月明细科目集。
func (wb *Workbook) updateMLDetailHeaders(sheet string, details []string) {
	lay := mlLayout()
	for i := 0; i < mlMaxDetails; i++ {
		col := mlDetailExcelCol(lay, i)
		cell := mlCellName(col, 2)
		label := ""
		if i < len(details) {
			label = details[i]
		}
		wb.File.SetCellValue(sheet, cell, label)
	}
}

// readMLDetailHeaders 从 Sheet 第2行读取现有明细列标题，构建 detailName → colIndex 映射。
// 返回的 details 按列顺序排列（空列对应空字符串）。
func (wb *Workbook) readMLDetailHeaders(sheet string) (detailIdx map[string]int, details []string, err error) {
	lay := mlLayout()
	detailIdx = make(map[string]int)
	details = make([]string, mlMaxDetails)

	rows, err := wb.File.GetRows(sheet)
	if err != nil {
		return nil, nil, fmt.Errorf("读取 Sheet %s: %w", sheet, err)
	}
	if len(rows) < 2 {
		return detailIdx, details, nil
	}

	row2 := rows[1]
	for i := 0; i < mlMaxDetails; i++ {
		colIdx := mlDetailRowIdx(lay, i) // GetRows 索引 = BindingLeftCols + 7 + i
		label := ""
		if colIdx < len(row2) {
			label = strings.TrimSpace(row2[colIdx])
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
func (wb *Workbook) appendToMLSheet(general string, entries []voucher.Entry, detailIdx map[string]int, initial int64) error {
	sheet := sheetNameML(general)
	lay := mlLayout()

	numDetails := mlMaxDetails

	rows, _ := wb.File.GetRows(sheet)
	isNew := len(rows) <= 2

	if isNew && initial != 0 {
		wb.writeMLCarryForwardRow(sheet, 3, initial, 0, 0, make([]mlDetailTotals, numDetails), "上年结转")
	}

	// 计算页码：已有过次页数 + 1
	pageNum := 1
	for _, r := range rows {
		if len(r) > lay.BindingLeftCols+2 && r[lay.BindingLeftCols+2] == pageBreakLabel {
			pageNum++
		}
	}

	row, err := wb.mlNextDataRow(sheet)
	if err != nil {
		return err
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Date != entries[j].Date {
			return entries[i].Date < entries[j].Date
		}
		return entries[i].VoucherNum < entries[j].VoucherNum
	})

	balance := initial
	var pageDebit, pageCredit int64
	pageDetails := make([]mlDetailTotals, numDetails)

	if !isNew {
		balance = wb.mlLastPageBalance(sheet)
		if !wb.mlPageHasBreakRow(sheet) {
			wb.markExistingMLPageForPrint(sheet)
		}
	}

	for _, e := range entries {
		// 补承前页（上月遗留的孤立过次页）
		if wb.mlLastRowIsOrphanBreak(sheet) {
			pbDebit, pbCredit := wb.mlLastBreakTotals(sheet)
			pbDetails := wb.lastBreakDetailTotals(sheet)
			pageNum++
			wb.writeMLPageHeader(sheet, row, pageNum, general)
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
			pageNum++
			wb.writeMLPageHeader(sheet, row, pageNum, general)
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

		wb.File.SetCellValue(sheet, mlCellName(lay.FrontStartCol+0, row), e.Date)
		wb.File.SetCellValue(sheet, mlCellName(lay.FrontStartCol+1, row), e.VoucherNum)
		wb.File.SetCellValue(sheet, mlCellName(lay.FrontStartCol+2, row), e.Summary)
		wb.File.SetCellValue(sheet, mlCellName(lay.FrontStartCol+3, row), centsToYuan(e.DebitCents))
		wb.File.SetCellValue(sheet, mlCellName(lay.FrontStartCol+4, row), centsToYuan(e.CreditCents))
		wb.File.SetCellValue(sheet, mlCellName(lay.FrontStartCol+5, row), dir)
		wb.File.SetCellValue(sheet, mlCellName(lay.FrontStartCol+6, row), centsToYuan(dispBal))

		wb.setMoneyStyle(sheet, row, lay.FrontStartCol+3)
		wb.setMoneyStyle(sheet, row, lay.FrontStartCol+4)
		wb.setMoneyStyle(sheet, row, lay.FrontStartCol+6)

		if e.DetailAccount != "" {
			if idx, ok := detailIdx[e.DetailAccount]; ok {
				net := e.DebitCents - e.CreditCents
				col := mlDetailStartCol + idx
				wb.File.SetCellValue(sheet, mlCellName(col, row), centsToYuan(net))
				wb.setMoneyStyle(sheet, row, col)
				pageDetails[idx].debit += e.DebitCents
				pageDetails[idx].credit += e.CreditCents
			}
		}

		wb.markMLRowForPrint(sheet, row)
		row++
	}

	return nil
}

// writeMLPageBreakRow 写多科目明细账的"过次页"行，A-G 总计 + H-U 各明细本页净额。
func (wb *Workbook) writeMLPageBreakRow(sheet string, row int, balance int64, pageDebit, pageCredit int64, pageDetails []mlDetailTotals) {
	lay := mlLayout()
	dir, dispBal := directionFor(balance, 0)
	wb.File.SetCellValue(sheet, mlCellName(lay.FrontStartCol+0, row), "")
	wb.File.SetCellValue(sheet, mlCellName(lay.FrontStartCol+1, row), "")
	wb.File.SetCellValue(sheet, mlCellName(lay.FrontStartCol+2, row), pageBreakLabel)
	wb.File.SetCellValue(sheet, mlCellName(lay.FrontStartCol+3, row), centsToYuan(pageDebit))
	wb.File.SetCellValue(sheet, mlCellName(lay.FrontStartCol+4, row), centsToYuan(pageCredit))
	wb.File.SetCellValue(sheet, mlCellName(lay.FrontStartCol+5, row), dir)
	wb.File.SetCellValue(sheet, mlCellName(lay.FrontStartCol+6, row), centsToYuan(dispBal))

	wb.setMoneyStyle(sheet, row, lay.FrontStartCol+3)
	wb.setMoneyStyle(sheet, row, lay.FrontStartCol+4)
	wb.setMoneyStyle(sheet, row, lay.FrontStartCol+6)

	for i, pd := range pageDetails {
		net := pd.debit - pd.credit
		col := mlDetailExcelCol(lay, i)
		wb.File.SetCellValue(sheet, mlCellName(col, row), centsToYuan(net))
		wb.setMoneyStyle(sheet, row, col)
	}
}

// writeMLCarryForwardRow 写多科目明细账的"承前页"行，与过次页数据相同。
func (wb *Workbook) writeMLCarryForwardRow(sheet string, row int, balance int64, pageDebit, pageCredit int64, pageDetails []mlDetailTotals, label string) {
	lay := mlLayout()
	dir, dispBal := directionFor(balance, 0)
	wb.File.SetCellValue(sheet, mlCellName(lay.FrontStartCol+0, row), "")
	wb.File.SetCellValue(sheet, mlCellName(lay.FrontStartCol+1, row), "")
	wb.File.SetCellValue(sheet, mlCellName(lay.FrontStartCol+2, row), label)
	wb.File.SetCellValue(sheet, mlCellName(lay.FrontStartCol+3, row), centsToYuan(pageDebit))
	wb.File.SetCellValue(sheet, mlCellName(lay.FrontStartCol+4, row), centsToYuan(pageCredit))
	wb.File.SetCellValue(sheet, mlCellName(lay.FrontStartCol+5, row), dir)
	wb.File.SetCellValue(sheet, mlCellName(lay.FrontStartCol+6, row), centsToYuan(dispBal))

	wb.setMoneyStyle(sheet, row, lay.FrontStartCol+3)
	wb.setMoneyStyle(sheet, row, lay.FrontStartCol+4)
	wb.setMoneyStyle(sheet, row, lay.FrontStartCol+6)

	for i, pd := range pageDetails {
		net := pd.debit - pd.credit
		col := mlDetailExcelCol(lay, i)
		wb.File.SetCellValue(sheet, mlCellName(col, row), centsToYuan(net))
		wb.setMoneyStyle(sheet, row, col)
	}
}


// writeMLPageHeader 写入多科目明细账后续页标题行（过次页之后、承前页之前调用），
// 包含页码、科目名称、多科目明细账标题、列标题。
// 行结构（5 行，与 writePageHeader 一致）：
//   Row N+0: 分第 n 页（右侧，绿色+数字红色）
//   Row N+1: 多科目明细账 — 科目名（居中，绿色+双下划线）| 科目名称（右侧）
//   Row N+2: 科目名称（右侧，印章红）
//   Row N+3: [空行]
//   Row N+4: 日期│凭证号│摘要│借方金额│贷方金额│方向│余额
func (wb *Workbook) writeMLPageHeader(sheet string, row int, pageNum int, general string) error {
	lay := mlLayout()

	darkGreen := "006100"
	sealRed := "CC0000"

	// Row N+0: 分第 n 页（右侧，绿色，数字印章红）
	pnLeft := mlCellName(lay.AccountColLeft, row)
	pnRight := mlCellName(lay.AccountColRight, row)
	wb.File.MergeCell(sheet, pnLeft, pnRight)
	wb.File.SetCellRichText(sheet, pnLeft, []excelize.RichTextRun{
		{Text: "分第 ", Font: &excelize.Font{Color: darkGreen, Size: 10}},
		{Text: fmt.Sprintf("%d", pageNum), Font: &excelize.Font{Color: sealRed, Size: 10}},
		{Text: " 页", Font: &excelize.Font{Color: darkGreen, Size: 10}},
	})
	wb.File.SetRowHeight(sheet, row, 18)
	row++

	// Row N+1: 多科目明细账 — 科目名（居中）+ 科目名称（右侧）
	tl := mlCellName(lay.TitleColLeft, row)
	tr := mlCellName(lay.TitleColRight, row)
	wb.File.MergeCell(sheet, tl, tr)
	wb.File.SetCellValue(sheet, tl, "多科目明细账 — "+general)
	titleStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 14, Color: darkGreen, Underline: "double"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	wb.File.SetCellStyle(sheet, tl, tr, titleStyle)

	al := mlCellName(lay.AccountColLeft, row)
	ar := mlCellName(lay.AccountColRight, row)
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
	acLeft := mlCellName(lay.AccountColLeft, row)
	acRight := mlCellName(lay.AccountColRight, row)
	wb.File.MergeCell(sheet, acLeft, acRight)
	wb.File.SetCellValue(sheet, acLeft, general)
	acRowStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: sealRed, Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	wb.File.SetCellStyle(sheet, acLeft, acRight, acRowStyle)
	wb.File.SetRowHeight(sheet, row, 18)
	row++

	// Row N+3: [空行]
	row++

	// Row N+4: 列标题
	colNames := []string{"日期", "凭证号", "摘要", "借方金额", "贷方金额", "方向", "余额"}
	for i, h := range colNames {
		cell := mlCellName(lay.FrontStartCol+i, row)
		wb.File.SetCellValue(sheet, cell, h)
	}
	headerStyle, _ := wb.File.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 10},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#D9E1F2"}, Pattern: 1},
		Border: []excelize.Border{
			{Type: "bottom", Color: "#808080", Style: 1},
		},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	hs := mlCellName(lay.FrontStartCol, row)
	he := mlCellName(lay.FrontStartCol+len(colNames)-1, row)
	wb.File.SetCellStyle(sheet, hs, he, headerStyle)

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
		if mlHasPageBreakAt(rows[i], lay) {
			result := make([]mlDetailTotals, mlMaxDetails)
			for j := 0; j < mlMaxDetails; j++ {
				colIdx := mlDetailRowIdx(lay, j) // GetRows 索引 = BindingLeftCols + 7 + j
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
