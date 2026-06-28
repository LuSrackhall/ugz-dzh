package generator

import (
	"fmt"

	"ledger/balance"
	"ledger/voucher"

	"github.com/xuri/excelize/v2"
)

// GeneratePrintWorkbook 生成打印版 Excel 工作薄。
func GeneratePrintWorkbook(configPath, month, outputDir string, entries []voucher.Entry) error {
	cfg, err := balance.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("加载配置: %w", err)
	}

	// 创建新的工作薄（打印版）
	wb := &Workbook{
		Config:    cfg,
		Month:     month,
		OutputDir: outputDir,
	}

	// 初始化 Excel 文件
	wb.File = excelize.NewFile()

	// 计算期初余额
	initials := computeInitials(cfg, month)

	// 生成打印版总分类账
	if err := wb.AppendPrintEntries(entries, initials); err != nil {
		return fmt.Errorf("生成打印版总分类账: %w", err)
	}

	// 生成打印版多科目明细账
	if err := wb.AppendPrintMLEntries(entries, initials); err != nil {
		return fmt.Errorf("生成打印版多科目明细账: %w", err)
	}

	// 删除默认 Sheet1
	if len(wb.File.GetSheetList()) > 1 {
		wb.File.DeleteSheet("Sheet1")
	}

	// 保存文件
	printPath := fmt.Sprintf("%s/%s-print.xlsx", outputDir, month)
	if err := wb.File.SaveAs(printPath); err != nil {
		return fmt.Errorf("保存打印版工作薄: %w", err)
	}

	return nil
}

// computeInitials 计算期初余额
func computeInitials(cfg *balance.GlobalConfig, month string) map[string]int64 {
	// 这里简化处理，实际需要从配置中计算期初余额
	// 参考现有 GenerateWorkbook 的实现
	initials := make(map[string]int64)

	// 从配置中获取所有叶子科目
	for account := range cfg.Tree {
		initials[account] = 0
	}

	return initials
}
