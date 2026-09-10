// Package generator — 打印版位格输出：入口。
//
// TransformToPrint 从已落盘的查看版 xlsx 生成打印版位格 xlsx：
//  1. 复制查看版文件为打印版（查看版本身零改动）
//  2. 打开打印版，对 总分类账-/多科目明细账-/明细账- 前缀的 Sheet 做金额列拆位变换
//  3. 其余 Sheet（期初/期末等）保持原样（数据与样式不变）
//  4. 保存打印版（**始终含全部 Sheet**；纯账页版见 TransformToPrintLedgerOnly）
//
// 失败返回 error，调用方应仅告警而不中断主流程（查看版已成功落盘）。
//
// TransformToPrintLedgerOnly 在完整变换的基础上再移除全部非账页 Sheet，
// 产出**纯账页版**（GL/多科目明细账/分离明细账）——供导出 PDF 成册使用，
// 由 generate 同步输出到 {年}/pdf/ 子目录（默认产物，不受配置开关影响）。
package generator

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
)

// TransformToPrint 从查看版 xlsx 生成打印版位格 xlsx。
// viewPath 为已落盘的查看版路径，printPath 为目标打印版路径。
func TransformToPrint(viewPath, printPath string) error {
	if err := os.MkdirAll(filepath.Dir(printPath), 0o755); err != nil {
		return fmt.Errorf("创建打印目录: %w", err)
	}
	if err := copyFile(viewPath, printPath); err != nil {
		return fmt.Errorf("复制查看版为打印版: %w", err)
	}

	f, err := excelize.OpenFile(printPath)
	if err != nil {
		return fmt.Errorf("打开打印版: %w", err)
	}
	defer f.Close()

	// 基准字体（Normal 默认字体，列宽像素计算基准）：可由 print-config.json 字体.基准 配置
	// （如统一两端用 Arimo 时改为 "Arimo"）。默认 Calibri 与现状一致。
	if err := f.SetDefaultFont(currentFonts().Normal); err != nil {
		fmt.Fprintf(os.Stderr, "警告: 设置基准字体失败: %v\n", err)
	}

	// 先变换并收集各 sheet 的打印区域规划，全部归位后统一写入——
	// SetDefinedName 的 scope 依赖最终 sheet 顺序，且中途的删除/重建会
	// 干扰已写 definedName 的 localSheetId（实测串位）。
	pending := make(map[string][]areaRect)
	for _, sheet := range f.GetSheetList() {
		switch {
		case strings.HasPrefix(sheet, sheetPrefixGL):
			printSheetType = "gl" // GL/ML 分账本配置（printSheetType 驱动系数/字体按账本类型取）
			rects, err := transformGLSheet(f, sheet)
			if err != nil {
				return fmt.Errorf("变换总分类账 %s: %w", sheet, err)
			}
			if len(rects) > 0 {
				pending[sheet] = rects
			}
		case strings.HasPrefix(sheet, sheetPrefixDetail):
			// 明细科目独立账页（ML 家族分离形态，ML 样式多栏式）：走 ML 变换（11/10 列拆位）
			printSheetType = "ml"
			rects, err := transformMLSheet(f, sheet)
			if err != nil {
				return fmt.Errorf("变换独立明细账 %s: %w", sheet, err)
			}
			if len(rects) > 0 {
				pending[sheet] = rects
			}
		case strings.HasPrefix(sheet, sheetPrefixML):
			printSheetType = "ml"
			rects, err := transformMLSheet(f, sheet)
			if err != nil {
				return fmt.Errorf("变换多科目明细账 %s: %w", sheet, err)
			}
			if len(rects) > 0 {
				pending[sheet] = rects
			}
		}
	}

	for sheet, rects := range pending {
		if err := writeSheetPrintArea(f, sheet, rects); err != nil {
			return fmt.Errorf("写打印区域 %s: %w", sheet, err)
		}
	}

	if err := f.SaveAs(printPath); err != nil {
		return fmt.Errorf("保存打印版: %w", err)
	}
	return nil
}

// TransformToPrintLedgerOnly 生成纯账页打印版（供导出 PDF 成册）：
// 与 TransformToPrint 相同的位格变换，再移除日记账/报表/期初期末表等非账页 Sheet。
// 返回移除的非账页 Sheet 数（供调用方日志）。
func TransformToPrintLedgerOnly(viewPath, outPath string) (int, error) {
	if err := TransformToPrint(viewPath, outPath); err != nil {
		return 0, err
	}
	f, err := excelize.OpenFile(outPath)
	if err != nil {
		return 0, fmt.Errorf("打开纯账页版: %w", err)
	}
	removed := removeNonLedgerSheets(f)
	if err := f.SaveAs(outPath); err != nil {
		f.Close()
		return 0, fmt.Errorf("保存纯账页版: %w", err)
	}
	f.Close()
	return removed, nil
}

// removeNonLedgerSheets 移除全部非账页 Sheet，返回删除数。
// 账页判定复用 isLedgerSheet（print_font.go）。账页至少一张（GL 必在）；
// 删除后激活首张账页。
func removeNonLedgerSheets(f *excelize.File) int {
	removed := 0
	for _, sheet := range f.GetSheetList() {
		if isLedgerSheet(sheet) {
			continue
		}
		if err := f.DeleteSheet(sheet); err != nil {
			continue // 单张删除失败不阻断（保底：账页不受影响）
		}
		removed++
	}
	if list := f.GetSheetList(); len(list) > 0 {
		f.SetActiveSheet(0)
	}
	return removed
}

// copyFile 复制文件内容（覆盖目标）。
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
