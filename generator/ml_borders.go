package generator

import (
	"github.com/xuri/excelize/v2"
)

// 颜色与线型常量（与 GL 一致）
const (
	mlGreen      = "006100"
	mlRed        = "CC0000"
	mlBorderThin = 1
	mlBorderThick = 2
	mlBorderDouble = 6
)

// mlBorderSet 替换指定单元格某一侧边框（保留其他属性：字体/对齐/金额格式/其他边框）。
func mlBorderSet(f *excelize.File, sheet string, col, row int, side, color string, style int) {
	cell := mlCellName(col, row)
	sid, err := f.GetCellStyle(sheet, cell)
	if err != nil {
		return
	}
	var st *excelize.Style
	if sid != 0 {
		st, err = f.GetStyle(sid)
		if err != nil {
			return
		}
	} else {
		st = &excelize.Style{}
	}
	borders := make([]excelize.Border, 0, len(st.Border)+1)
	for _, b := range st.Border {
		if b.Type != side {
			borders = append(borders, b)
		}
	}
	borders = append(borders, excelize.Border{Type: side, Color: color, Style: style})
	st.Border = borders
	ns, err := f.NewStyle(st)
	if err != nil {
		return
	}
	f.SetCellStyle(sheet, cell, cell, ns)
}

// mlBorderClear 移除指定单元格某一侧边框（保留其他属性）。
func mlBorderClear(f *excelize.File, sheet string, col, row int, side string) {
	cell := mlCellName(col, row)
	sid, err := f.GetCellStyle(sheet, cell)
	if err != nil || sid == 0 {
		return
	}
	st, err := f.GetStyle(sid)
	if err != nil {
		return
	}
	borders := make([]excelize.Border, 0, len(st.Border))
	for _, b := range st.Border {
		if b.Type != side {
			borders = append(borders, b)
		}
	}
	st.Border = borders
	ns, err := f.NewStyle(st)
	if err != nil {
		return
	}
	f.SetCellStyle(sheet, cell, cell, ns)
}

// applyMLBorders 为多科目明细账每个页面块应用完整边框：
//   - 表内区域（4行表头+数据+过次页，标题区在表外）全部绿色细框
//   - 每页表格最上（表头顶）/最下（过次页底） 红色双线
//   - 表头最下行下边框 与 每5数据行下边框 绿色加粗
//   - 表头 月/字 右边框绿色加粗，日/号 右边框红色单线
//   - 金额栏（借方/贷方/余额/明细1-14）左右红色双线
//   - Back 页最左空、最右红色双线；Front 页最左红色双线、最右空；红色双线溢出直达边距行
func applyMLBorders(f *excelize.File, sheet string) {
	lay := mlLayout()
	rows, _ := f.GetRows(sheet)
	if len(rows) == 0 {
		return
	}
	blockRows := lay.DataStartRow + pageSize + 1 + lay.BottomMarginRows // 31
	lastRow := len(rows)

	backL := lay.BackStartCol                    // B = 2
	backR := mlDetailCol(lay, 3)                 // N（明细4）= 14
	frontL := mlDetailCol(lay, 4)                // S（明细5）= 19
	frontR := mlDetailCol(lay, mlMaxDetails-1)   // AB（明细14）= 28
	gapL := backR + 1                            // O 中间装订区起始 = 15
	gapR := frontL - 1                           // R 中间装订区末 = 18
	isGutter := func(c int) bool { return c >= gapL && c <= gapR }

	for start := 1; start <= lastRow; start += blockRows {
		// Paper1 Front 占位块（首块）只有 Front（右侧）表，Back 侧（C-O）无表
		isPaper1 := start == 1
		colStart := backL
		if isPaper1 {
			colStart = frontL
		}

		hStart := start + 4                        // 4行表头 h1 = 表内区域顶
		hEnd := start + lay.DataStartRow - 1       // 4行表头 h4
		dataStart := start + lay.DataStartRow      // 首数据行
		breakRow := dataStart + pageSize           // 过次页行（表底）
		bottomMargin := breakRow + 1               // 下边距行

		// 1) 表内区域绿色细框（表头+数据+过次页；标题区在外，不加边框）
		for r := hStart; r <= breakRow; r++ {
			for c := colStart; c <= frontR; c++ {
				if isGutter(c) {
					continue
				}
				mlBorderSet(f, sheet, c, r, "top", mlGreen, mlBorderThin)
				mlBorderSet(f, sheet, c, r, "bottom", mlGreen, mlBorderThin)
				mlBorderSet(f, sheet, c, r, "left", mlGreen, mlBorderThin)
				mlBorderSet(f, sheet, c, r, "right", mlGreen, mlBorderThin)
			}
		}

		// 2) 横向特殊边框
		// 每页表格最上方（表头顶）与最下方（过次页底） 红色双线
		for c := colStart; c <= frontR; c++ {
			if isGutter(c) {
				continue
			}
			mlBorderSet(f, sheet, c, hStart, "top", mlRed, mlBorderDouble)
			mlBorderSet(f, sheet, c, breakRow, "bottom", mlRed, mlBorderDouble)
		}
		// 表头最下行下边框 绿色加粗（= 首数据行上边框）
		for c := colStart; c <= frontR; c++ {
			if isGutter(c) {
				continue
			}
			mlBorderSet(f, sheet, c, hEnd, "bottom", mlGreen, mlBorderThick)
		}
		// 数据区每5行下边框 绿色加粗（第5/10/15/20 数据行）
		for k := 4; k < pageSize; k += 5 {
			r := dataStart + k
			for c := colStart; c <= frontR; c++ {
				if isGutter(c) {
					continue
				}
				mlBorderSet(f, sheet, c, r, "bottom", mlGreen, mlBorderThick)
			}
		}

		// 3) 表头 月/日/字/号 右边框：月、字=绿色加粗；日、号=红色单线（仅 Back 侧有）
		//    C=月, D=日, E=字, F=号；仅作用于 4 行表头区
		if !isPaper1 {
			for r := hStart; r <= hEnd; r++ {
				mlBorderSet(f, sheet, backL, r, "right", mlGreen, mlBorderThick)   // 月
				mlBorderSet(f, sheet, backL+1, r, "right", mlRed, mlBorderThin)    // 日
				mlBorderSet(f, sheet, backL+2, r, "right", mlGreen, mlBorderThick) // 字
				mlBorderSet(f, sheet, backL+3, r, "right", mlRed, mlBorderThin)    // 号
			}
		}

		// 4) 金额栏（借方H/贷方I/余额K/明细1-14）左右红色双线（表头+数据+过次页）
		//    Paper1 仅明细5-14（Front 侧）
		var moneyCols []int
		if !isPaper1 {
			moneyCols = append(moneyCols,
				backL+mlOffDebit, backL+mlOffCredit, backL+mlOffBalance)
		}
		for i := 0; i < mlMaxDetails; i++ {
			if isPaper1 && i < 4 {
				continue // Paper1 无 Back 明细
			}
			moneyCols = append(moneyCols, mlDetailCol(lay, i))
		}
		for _, c := range moneyCols {
			for r := hStart; r <= breakRow; r++ {
				mlBorderSet(f, sheet, c, r, "left", mlRed, mlBorderDouble)
				mlBorderSet(f, sheet, c, r, "right", mlRed, mlBorderDouble)
			}
		}

		// 5) 外侧边缘（红色双线溢出直达边距行，贯穿标题区）
		//    Back 最左（C）左边框为空；Back 最右（O）右边框红色双线（含边距行）
		//    Front 最左（Q）左边框红色双线（含边距行）；Front 最右（Z）右边框为空
		for r := start; r <= bottomMargin; r++ {
			if r >= hStart && r <= breakRow {
				if !isPaper1 {
					mlBorderClear(f, sheet, backL, r, "left") // Back 左空
				}
				mlBorderClear(f, sheet, frontR, r, "right") // Front 右空
			}
			if !isPaper1 {
				mlBorderSet(f, sheet, backR, r, "right", mlRed, mlBorderDouble) // Back 右
			}
			mlBorderSet(f, sheet, frontL, r, "left", mlRed, mlBorderDouble) // Front 左
		}

		// 6) 中间书口列（P/Q）清除所有边框 — 应为空白书口（防止月结样式等误画）
		for r := start; r <= bottomMargin; r++ {
			for c := gapL; c <= gapR; c++ {
				mlBorderClear(f, sheet, c, r, "left")
				mlBorderClear(f, sheet, c, r, "right")
				mlBorderClear(f, sheet, c, r, "top")
				mlBorderClear(f, sheet, c, r, "bottom")
			}
		}
	}
}
