package generator

import (
	"testing"

	"ledger/balance"

	"github.com/xuri/excelize/v2"
)

// TestExtractLastMonthFinalsFallsBackToCarryRow 固化期初提取回退（移除 M4 补月结的配套）：
// 无发生额月份的账页只有上年结转/期初行、没有期末余额行，上月期末提取必须回退到
// 账页上最后一条带符号余额的行，否则下月期初塌成 0、余额链断裂。
func TestExtractLastMonthFinalsFallsBackToCarryRow(t *testing.T) {
	f := excelize.NewFile()
	wb := &Workbook{File: f, Config: &balance.GlobalConfig{}}
	sheet := sheetNameGL("库存现金")
	if _, err := f.NewSheet(sheet); err != nil {
		t.Fatalf("建 sheet: %v", err)
	}
	lay := glLayout()
	row := lay.DataStartRow + 1 + lay.TopMarginRows
	// 模拟无发生额月份账页：仅一行上年结转（贷 1349.35，无任何期末余额行）
	f.SetCellValue(sheet, cellName(dataCol(lay, 1, 4), row), "上年结转")
	f.SetCellValue(sheet, cellName(dataCol(lay, 1, glColDir), row), "贷")
	f.SetCellValue(sheet, cellName(dataCol(lay, 1, glColBalance), row), "1349.35")

	finals, err := wb.ExtractLastMonthFinals()
	if err != nil {
		t.Fatalf("提取上月期末: %v", err)
	}
	if got := finals["库存现金"]; got != -134935 {
		t.Errorf("库存现金上月期末应为 -134935 分（贷 1349.35），实际 %d —— 回退失效会导致下月期初塌成 0", got)
	}
}
