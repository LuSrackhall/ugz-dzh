// Package generator — 打印版位格输出：入口。
//
// TransformToPrint 从已落盘的查看版 xlsx 生成打印版位格 xlsx：
//  1. 复制查看版文件为打印版（查看版本身零改动）
//  2. 打开打印版，对 总分类账-/多科目明细账- 前缀的 Sheet 做金额列 12 小列展开变换
//  3. 其余 Sheet（期初/期末等）保持原样（数据与样式不变）
//  4. 保存打印版
//
// 失败返回 error，调用方应仅告警而不中断主流程（查看版已成功落盘）。
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
	if err := f.SetDefaultFont(printCfg.字体.基准); err != nil {
		fmt.Fprintf(os.Stderr, "警告: 设置基准字体失败: %v\n", err)
	}

	for _, sheet := range f.GetSheetList() {
		switch {
		case strings.HasPrefix(sheet, sheetPrefixGL):
			if err := transformGLSheet(f, sheet); err != nil {
				return fmt.Errorf("变换总分类账 %s: %w", sheet, err)
			}
			applyPrintFont(f, sheet) // 字体统一宋体加粗（在拆位样式生成后）
		case strings.HasPrefix(sheet, sheetPrefixML):
			if err := transformMLSheet(f, sheet); err != nil {
				return fmt.Errorf("变换多科目明细账 %s: %w", sheet, err)
			}
			applyPrintFont(f, sheet)
		}
	}

	if err := f.SaveAs(printPath); err != nil {
		return fmt.Errorf("保存打印版: %w", err)
	}
	return nil
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
