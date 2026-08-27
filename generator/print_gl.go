// Package generator — 打印版位格输出：总分类账（含合并总账）变换配置。
package generator

import "github.com/xuri/excelize/v2"

// transformGLSheet 把单个总分类账 Sheet（含合并总账）变换为打印版位格布局。
//
// GL 每面（Front/Back）各 3 个金额列（借/贷/余额），共 6 个金额列 → 展开 +66 列。
// GL 为多页块结构：每块 blockRows = (SubHeaderRow+1) + pageSize + 1 + BottomMarginRows 行，
// 交替 Front/Back，每块自有 HeaderRow（大标题，由文本标签分支合并 12 列）与 SubHeaderRow（标签行）。
// 标签行 = 每块的 SubHeaderRow+1，即 (r - SubHeaderRow - 1) % blockRows == 0。
// 垂直分页符在 PageGapStartCol+1（Front 与 Back 列区之间）。
func transformGLSheet(f *excelize.File, sheet string) error {
	lay := glLayout()
	amountCols := []int{
		lay.FrontStartCol + glColDebit,
		lay.FrontStartCol + glColCredit,
		lay.FrontStartCol + glColBalance,
		lay.BackStartCol + glColDebit,
		lay.BackStartCol + glColCredit,
		lay.BackStartCol + glColBalance,
	}
	labelRow1 := lay.SubHeaderRow + 1 // 首块标签行（绝对行号）
	blockRows := (lay.SubHeaderRow + 1) + pageSize + 1 + lay.BottomMarginRows
	dataFirst := labelRow1 + 1 // 首块数据首行（标签行下一行）
	// 借/贷/余额 12 小列（十亿…分）：十亿位 k=0（表头"十"）+2px、百万位 k=3（"百"）
	// 与千位 k=6（"千"）+1.5px；其余 k 各减 0.27px（用户定值）。
	edgePixelDelta := map[[2]int]float64{}
	for _, c := range amountCols {
		for k := 0; k < 12; k++ {
			switch k {
			case 0: // 十亿位 表头"十"
				edgePixelDelta[[2]int{c, k}] = 2
			case 3, 6: // 百万位"百"、千位"千"
				edgePixelDelta[[2]int{c, k}] = 1.5
			default:
				edgePixelDelta[[2]int{c, k}] = -0.27
			}
		}
	}
	// 非金额列加宽：月/日/字/号 +0.5px；借或贷、借方旁对号、贷方旁对号、余额旁对号 +1.5px（正反面）。
	// 列号：正面 FrontStartCol=3（月3 日4 字5 号6 借✓9 贷✓11 借或贷12 余✓14）；
	// 反面 BackStartCol=17（+14）。借或贷+对号列 0.5→1→1.5→2→1.5px（5b517f6/3a91202/598e8b0/d5bafc9/本次）。
	nonAmountPixelDelta := map[int]float64{}
	apply := func(base int) {
		for _, c := range []int{base, base + 1, base + 2, base + 3} { // 月/日/字/号
			nonAmountPixelDelta[c] = 0.5
		}
		nonAmountPixelDelta[base+6] = 1.5  // 借方旁对号
		nonAmountPixelDelta[base+8] = 1.5  // 贷方旁对号
		nonAmountPixelDelta[base+9] = 1.5  // 借或贷
		nonAmountPixelDelta[base+11] = 1.5 // 余额旁对号
	}
	apply(lay.FrontStartCol)
	apply(lay.BackStartCol)
	// GL 摘要列拆 4 格（仅为标题行"双线底边框从摘要列 3/4 处开始"提供列粒度）：
	// 数据区/表头区每行合并回单格（视觉不变），只标题行保留 4 格——前 3 格无底边框、
	// 第 4 格（=3/4 处）起到标题右缘画双线。查看版列号：正面摘要=FrontStartCol+4(7)、
	// 反面摘要=BackStartCol+4(21)。
	splitNA := map[int]int{
		lay.FrontStartCol + 4: 4,
		lay.BackStartCol + 4:  4,
	}
	// 摘要列整体缩短约 3px（正反面；用户定值，配合"字符数等分"口径微调）
	splitNAPixelDelta := map[int]float64{
		lay.FrontStartCol + 4: -3,
		lay.BackStartCol + 4:  -3,
	}
	// 数据区横向居中（不含表头）：凭证号列（view 6/20）与借或贷列（view 12/26）的数据内容区
	dataAlignCols := map[int]string{
		lay.FrontStartCol + 3: "center", // 号
		lay.BackStartCol + 3:  "center",
		lay.FrontStartCol + 9: "center", // 借或贷
		lay.BackStartCol + 9:  "center",
	}
	cfg := printSheetConfig{
		totalViewCols:       lay.TotalCols,
		amountCols:          amountCols,
		splitNA:             splitNA,
		splitNAPixelDelta:   splitNAPixelDelta,
		dataAlignCols:       dataAlignCols,
		edgePixelDelta:      edgePixelDelta,
		nonAmountPixelDelta: nonAmountPixelDelta,
		// 数据区金额数字（借方/贷方/余额）：字体可配（print-config.json 字体.数字；默认 Noteworthy，字号保持 7pt）
		dataFontFamily: currentFonts().Digit,
		postProcess:    applyGLTitleArea,
		isLabelRow: func(r int) bool {
			if r < labelRow1 || blockRows <= 0 {
				return false
			}
			return (r-labelRow1)%blockRows == 0
		},
		isDataRow: func(r int) bool {
			// 数据区 = 数据行 + 过次页行（pageSize+1 行）；下边距行不含在内
			if r < dataFirst || blockRows <= 0 {
				return false
			}
			return (r-dataFirst)%blockRows < pageSize+1
		},
		breakViewCol:    lay.PageGapStartCol + 1,
		applyPageLayout: applyGLPrintPageLayout,
	}
	return transformSheet(f, sheet, cfg)
}

// applyGLTitleArea 打印版 GL 标题区后处理（对每个页面块的标题行与科目行）：
//  1. 总分类账：仿宋 22pt、居中，合并 = 边框起止（摘要列 3/4 处 → 贷方金额列亿位"亿"所在列），
//     底部双线边框自 3/4 处至右缘；3/4 之前的旧合并（月列起）拆分不再使用、区域留空，
//     正面首列仍保留装订边左红双线；
//  2. 分第：移到"借或贷"列 → 余额金额列十亿位（表头"十"所在列），右对齐；
//  3. 页码数字 n：右移至余额亿位 → 余额分，居中（与分第/页 构成"分第 n 页"）；
//  4. 会计科目（科目行）：移到贷方千位 → 贷方分（表头"千"/"分"所在列），右对齐；
//  5. 科目名：右移至贷方旁对号列 → 余额旁对号列。
// 活动半侧：奇页→Front、偶页→Back（与 dataCol 规则一致），仅处理活动半侧。
func applyGLTitleArea(f *excelize.File, sheet string, cm colMap, maxRow int) {
	lay := glLayout()
	blockRows := (lay.SubHeaderRow + 1) + pageSize + 1 + lay.BottomMarginRows
	// 标题行 2, 2+blockRows, …；科目行 = 标题行+1
	// 总分类账样式（居中）：仿宋 22pt、绿、粗体、居中+底对齐，底部双线边框
	// （自摘要列 3/4 处到标题右缘，合并范围=边框起止）。
	titleStyleCenter, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Family: currentFonts().Title, Size: 22, Color: "006100", Bold: true},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "bottom"},
		Border:    []excelize.Border{{Type: "bottom", Color: "006100", Style: 6}},
	})
	if err != nil {
		return
	}
	// 装订边红双线：正面标题行首列（月列）是装订边，需保留 left=double#CC0000。
	// 该列已不在标题合并内（合并=边框起止），单独用 titleStyleEdge（左红双线）。
	titleStyleEdge, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Family: currentFonts().Title, Size: 22, Color: "006100", Bold: true},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "bottom"},
		Border:    []excelize.Border{{Type: "left", Color: "CC0000", Style: 6}},
	})
	if err != nil {
		return
	}
	// 清理用无边框样式（搬走后残留的虚线底/样式）
	plainStyle, err := f.NewStyle(&excelize.Style{Alignment: &excelize.Alignment{Vertical: "bottom"}})
	if err != nil {
		return
	}
	for titleRow := 2; titleRow <= maxRow-1; titleRow += blockRows {
		half := lay.FrontStartCol
		if ((titleRow-2)/blockRows)%2 == 1 {
			half = lay.BackStartCol
		}
		accRow := titleRow + 1
		creditStart := cm.startCol(half + glColCredit) // 贷方金额列（12 小列起）
		balanceStart := cm.startCol(half + glColBalance)
		// 新布局打印列
		tStart := cm.startCol(half)                // 月列
		tEnd := creditStart + 1                    // 贷方亿位 k1
		fenStart := cm.startCol(half + glColDir)   // 借或贷
		fenEnd := balanceStart                     // 余额十亿位 k0
		numStart := balanceStart + 1               // 余额亿位 k1
		numEnd := balanceStart + 11                // 余额分 k11
		accStart := creditStart + 6                // 贷方千位 k6
		accEnd := creditStart + 11                 // 贷方分 k11
		nameStart := cm.startCol(half + 8)         // 贷方旁对号
		nameEnd := cm.startCol(half + 11)          // 余额旁对号（页列）
		// 旧布局打印列
		fenOld := cm.startCol(half + 6)            // 借方旁对号（旧分第起点）
		numOld := cm.startCol(half + 8)            // 贷方旁对号（旧数字起点）
		accOld := cm.startCol(half + 6)            // 旧会计科目
		nameOld := cm.startCol(half + glColCredit) // 旧科目名起点
		// 读取旧值/样式（解除合并前）
		pnLabelSid, _ := f.GetCellStyle(sheet, cellAxis(fenOld, titleRow))
		pnNumSid, _ := f.GetCellStyle(sheet, cellAxis(numOld, titleRow))
		numVal, _ := f.GetCellValue(sheet, cellAxis(numOld, titleRow))
		labelSid, _ := f.GetCellStyle(sheet, cellAxis(accOld, accRow))
		accVal, _ := f.GetCellValue(sheet, cellAxis(accOld, accRow))
		nameSid, _ := f.GetCellStyle(sheet, cellAxis(nameOld, accRow))
		nameVal, _ := f.GetCellValue(sheet, cellAxis(nameOld, accRow))
		// 装订边红双线（反面）：科目名旧合并的右端格（查看版 AB 列）带 right=double#CC0000，
		// 但 nameSid 取自旧合并左上角（无右框），整段覆盖会把右端红双线清掉（55746bb 回归）。
		// 反面 nameEnd 格单独克隆 nameSid + 右红双线。
		nameSidEdge := nameSid
		if half == lay.BackStartCol {
			if st, gerr := f.GetStyle(nameSid); gerr == nil {
				if st.Border == nil {
					st.Border = []excelize.Border{}
				}
				st.Border = append(st.Border, excelize.Border{Type: "right", Color: "CC0000", Style: 6})
				if nid, nerr := f.NewStyle(st); nerr == nil {
					nameSidEdge = nid
				}
			}
		}
		// 解除旧合并（必须先全部解除再建新合并，避免重叠）
		_ = f.UnmergeCell(sheet, cellAxis(tStart, titleRow), cellAxis(cm.endCol(half+glColDebit), titleRow))
		_ = f.UnmergeCell(sheet, cellAxis(fenOld, titleRow), cellAxis(cm.endCol(half+glColCredit), titleRow))
		_ = f.UnmergeCell(sheet, cellAxis(numOld, titleRow), cellAxis(cm.endCol(half+glColBalance), titleRow))
		_ = f.UnmergeCell(sheet, cellAxis(nameOld, accRow), cellAxis(cm.endCol(half+11), accRow))
		// 清空旧值：必须在新合并创建**之前**（excelize SetCellValue 对合并区内部格
		// 会重定向到左上角——若先建新合并再清旧值，会把新合并左上角的值误清）
		_ = f.SetCellValue(sheet, cellAxis(fenOld, titleRow), "")
		_ = f.SetCellValue(sheet, cellAxis(numOld, titleRow), "")
		_ = f.SetCellValue(sheet, cellAxis(accOld, accRow), "")
		_ = f.SetCellValue(sheet, cellAxis(nameOld, accRow), "")
		// ── 标题行：总分类账 / 分第 / 数字 / 页 ──
		// 摘要列（view half+4）已拆 4 格，第 4 子格 = 3/4 处 = 双线底边框起点。
		sumSub4 := cm.startCol(half+4) + 3
		// 总分类账：合并 = 边框起止（摘要列 3/4 处 → 标题右缘），居中；3/4 之前的
		// 旧合并（月列起）拆分不再使用，该区域清空为无边框（正面首列保留装订边左红双线）。
		_ = f.SetCellStyle(sheet, cellAxis(tStart, titleRow), cellAxis(sumSub4-1, titleRow), plainStyle)
		// 清掉旧标题文本（合并已不再覆盖该格，查看版标题值残留在月列）
		_ = f.SetCellValue(sheet, cellAxis(tStart, titleRow), "")
		if half == lay.FrontStartCol {
			// 正面标题行首列 = 装订边红双线左框
			_ = f.SetCellStyle(sheet, cellAxis(tStart, titleRow), cellAxis(tStart, titleRow), titleStyleEdge)
		}
		_ = f.SetCellValue(sheet, cellAxis(sumSub4, titleRow), "总  分  类  账")
		_ = f.SetCellStyle(sheet, cellAxis(sumSub4, titleRow), cellAxis(tEnd, titleRow), titleStyleCenter)
		_ = f.MergeCell(sheet, cellAxis(sumSub4, titleRow), cellAxis(tEnd, titleRow))
		_ = f.SetCellValue(sheet, cellAxis(fenStart, titleRow), "分第 ")
		_ = f.SetCellStyle(sheet, cellAxis(fenStart, titleRow), cellAxis(fenEnd, titleRow), pnLabelSid)
		_ = f.MergeCell(sheet, cellAxis(fenStart, titleRow), cellAxis(fenEnd, titleRow))
		_ = f.SetCellValue(sheet, cellAxis(numStart, titleRow), numVal)
		_ = f.SetCellStyle(sheet, cellAxis(numStart, titleRow), cellAxis(numEnd, titleRow), pnNumSid)
		_ = f.MergeCell(sheet, cellAxis(numStart, titleRow), cellAxis(numEnd, titleRow))
		// 清理：旧数字起点残留的绿色虚线底
		if numOld != numStart {
			_ = f.SetCellStyle(sheet, cellAxis(numOld, titleRow), cellAxis(numOld, titleRow), plainStyle)
		}
		// ── 科目行：会计科目 / 科目名 ──
		_ = f.SetCellValue(sheet, cellAxis(accStart, accRow), accVal)
		_ = f.SetCellStyle(sheet, cellAxis(accStart, accRow), cellAxis(accEnd, accRow), labelSid)
		_ = f.MergeCell(sheet, cellAxis(accStart, accRow), cellAxis(accEnd, accRow))
		_ = f.SetCellValue(sheet, cellAxis(nameStart, accRow), nameVal)
		_ = f.SetCellStyle(sheet, cellAxis(nameStart, accRow), cellAxis(nameEnd, accRow), nameSid)
		if half == lay.BackStartCol && nameSidEdge != nameSid {
			// 反面科目名右端格 = 装订边红双线右框
			_ = f.SetCellStyle(sheet, cellAxis(nameEnd, accRow), cellAxis(nameEnd, accRow), nameSidEdge)
		}
		_ = f.MergeCell(sheet, cellAxis(nameStart, accRow), cellAxis(nameEnd, accRow))
		// 清理：旧科目名残留的绿色虚线底（accOld..accStart-1）
		if accOld < accStart {
			_ = f.SetCellStyle(sheet, cellAxis(accOld, accRow), cellAxis(accStart-1, accRow), plainStyle)
		}
	}
}
