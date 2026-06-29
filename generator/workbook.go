package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"ledger/balance"
	"ledger/voucher"

	"github.com/xuri/excelize/v2"
)

// Workbook 持有 excelize.File 和当月生成上下文。
type Workbook struct {
	File         *excelize.File
	Config       *balance.GlobalConfig
	Month        string // YYYY-MM
	OutputDir    string
	ConfigPath   string
	moneyStyleID int
}

// NewWorkbook 创建或加载工作薄。若上月 xlsx 存在则复制之，否则新建。
func NewWorkbook(configPath, month, outputDir string) (*Workbook, error) {
	cfg, err := balance.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("加载配置: %w", err)
	}

	wb := &Workbook{
		Config:     cfg,
		Month:      month,
		OutputDir:  outputDir,
		ConfigPath: configPath,
	}

	prevPath := wb.prevMonthPath()
	if _, err := os.Stat(prevPath); err == nil {
		src, err := excelize.OpenFile(prevPath)
		if err != nil {
			return nil, fmt.Errorf("打开上月 xlsx %s: %w", prevPath, err)
		}
		wb.File = src
	} else {
		wb.File = excelize.NewFile()
		// "Sheet1" 作为唯一 sheet 时无法删除，延迟到 Save() 处理
	}

	moneyStyle, err := wb.File.NewStyle(&excelize.Style{
		CustomNumFmt: stringPtr("#,##0.00"),
	})
	if err != nil {
		return nil, fmt.Errorf("创建金额样式: %w", err)
	}
	wb.moneyStyleID = moneyStyle

	return wb, nil
}

// prevMonthPath 返回上月 xlsx 路径。
func (wb *Workbook) prevMonthPath() string {
	prev := prevMonth(wb.Month)
	if prev == "" {
		return ""
	}
	// 上月 xlsx 在上月目录中
	prevDir := filepath.Join(wb.OutputDir, prev)
	return filepath.Join(prevDir, prev+".xlsx")
}

// currentPath 返回本月 xlsx 路径。
func (wb *Workbook) currentPath() string {
	// outputDir 已经是月度目录，直接使用
	return filepath.Join(wb.OutputDir, wb.Month+".xlsx")
}

// Save 保存工作薄到本月文件。
func (wb *Workbook) Save() error {
	// 清理默认 "Sheet1" — 此时已有其他 sheet，可以安全删除
	if len(wb.File.GetSheetList()) > 1 {
		wb.File.DeleteSheet("Sheet1")
	}
	return wb.File.SaveAs(wb.currentPath())
}

// ExtractLastMonthFinals 从各总分类账 Sheet 的"期末余额"行提取 G 列余额。
func (wb *Workbook) ExtractLastMonthFinals() (map[string]int64, error) {
	finals := make(map[string]int64)
	for _, name := range wb.File.GetSheetList() {
		if !strings.HasPrefix(name, sheetPrefixGL) {
			continue
		}
		account := strings.TrimPrefix(name, sheetPrefixGL)
		rows, err := wb.File.GetRows(name)
		if err != nil {
			continue
		}
	// 找到最后一个"期末余额"行（月结行）
		var lastBalance int64
		for _, row := range rows {
			if len(row) >= 3 && row[2] == periodEndLabel {
				if len(row) >= 7 {
					if v, err := yuanStrToCents(row[6]); err == nil {
						lastBalance = v
					}
				}
			}
		}
		finals[account] = lastBalance
	}
	return finals, nil
}

// sheet naming constants
const (
	sheetPrefixGL = "总分类账-"
	sheetPrefixML = "多科目明细账-"
	pageBreakLabel = "过次页"
	periodEndLabel = "期末余额"
)

const pageSize = 20

// yuanStrToCents 将 "1234.56" 格式字符串转为分。
func yuanStrToCents(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "0" {
		return 0, nil
	}
	parts := strings.Split(s, ".")
	yuan, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, err
	}
	var cents int64
	if len(parts) > 1 {
		frac := (parts[1] + "00")[:2]
		cents, _ = strconv.ParseInt(frac, 10, 64)
	}
	if yuan < 0 {
		return yuan*100 - cents, nil
	}
	return yuan*100 + cents, nil
}

// prevMonth 返回上个月标识。
func prevMonth(m string) string {
	yy := int(m[0]-'0')*1000 + int(m[1]-'0')*100 + int(m[2]-'0')*10 + int(m[3]-'0')
	mm := int(m[5]-'0')*10 + int(m[6]-'0')
	mm--
	if mm < 1 {
		mm = 12
		yy--
		if yy < 0 {
			return ""
		}
	}
	return fmt.Sprintf("%04d-%02d", yy, mm)
}

// centsToYuan 分转元数值。
func centsToYuan(c int64) float64 {
	return float64(c) / 100
}

// directionFor 根据借贷差返回方向和显示余额。
func directionFor(debit, credit int64) (direction string, displayBalance int64) {
	net := debit - credit
	if net > 0 {
		return "借", net
	} else if net < 0 {
		return "贷", -net
	}
	return "平", 0
}

// sheetNameGL 返回总分类账 Sheet 名称。
func sheetNameGL(account string) string {
	return sheetPrefixGL + account
}

// sheetNameML 返回多科目明细账 Sheet 名称。
func sheetNameML(general string) string {
	return sheetPrefixML + general
}

// entryMonth 返回分录的月份标识。
func entryMonth(e voucher.Entry) string {
	if len(e.Date) >= 7 {
		return e.Date[:7]
	}
	return ""
}

func stringPtr(s string) *string {
	return &s
}

// setMoneyStyle 对指定单元格应用金额数字格式 #,##0.00。
func (wb *Workbook) setMoneyStyle(sheet string, row, col int) {
	cell, _ := excelize.CoordinatesToCellName(col, row)
	wb.File.SetCellStyle(sheet, cell, cell, wb.moneyStyleID)
}
