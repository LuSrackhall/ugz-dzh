package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"ledger/balance"
	"ledger/voucher"

	"github.com/xuri/excelize/v2"
)

// Workbook 持有 excelize.File 和当月生成上下文。
type Workbook struct {
	File              *excelize.File
	Config            *balance.GlobalConfig
	Month             string // YYYY-MM
	OutputDir         string
	ConfigPath        string
	moneyStyleID      int
	MLSheetBalances   map[string]int64 // sheet名 → 最近期末余额
	moneyStyleThickID int              // 金额样式（底边加粗）
	InitialAdjust     map[string]bool  // 当月期初来自期初调整额的科目（摘要"期初余额"）
	detailCreditOnce  sync.Once
	detailCreditCache map[string]bool // 总账名 → 是否贷方向基准（科目树叶子属性优先，官方推断兜底）
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

	// 若当月文件已存在（如 year-close 预生成），优先加载
	currentPath := wb.currentPath()
	if _, err := os.Stat(currentPath); err == nil {
		src, err := excelize.OpenFile(currentPath)
		if err == nil {
			wb.File = src
			return wb, nil
		}
	}

	prevPath := wb.prevMonthPath()
	wb.File = excelize.NewFile()
	// "Sheet1" 作为唯一 sheet 时无法删除，延迟到 Save() 处理
	// 跨年安全：仅当上月 xlsx 与当前月同一年度时才复制（year-close 语义：新年首月从空开始，
	// 期初取自 JSON 上年末余额；若复制旧年 12 月会把旧年明细带进新年账本）
	if prevPath != "" && strings.HasPrefix(filepath.Base(prevPath), wb.Month[:5]) {
		if _, err := os.Stat(prevPath); err == nil {
			src, err := excelize.OpenFile(prevPath)
			if err != nil {
				return nil, fmt.Errorf("打开上月 xlsx %s: %w", prevPath, err)
			}
			wb.File = src
		}
	}

	moneyStyle, err := wb.File.NewStyle(&excelize.Style{
		CustomNumFmt: stringPtr("#,##0.00"),
		Border: []excelize.Border{
			{Type: "top", Color: "#006100", Style: 1},
			{Type: "right", Color: "#006100", Style: 1},
			{Type: "bottom", Color: "#006100", Style: 1},
			{Type: "left", Color: "#006100", Style: 1},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("创建金额样式: %w", err)
	}
	wb.moneyStyleID = moneyStyle

	// 金额样式（底边加粗，用于每5行）
	moneyStyleThick, err := wb.File.NewStyle(&excelize.Style{
		CustomNumFmt: stringPtr("#,##0.00"),
		Border: []excelize.Border{
			{Type: "top", Color: "#006100", Style: 1},
			{Type: "right", Color: "#006100", Style: 1},
			{Type: "bottom", Color: "#006100", Style: 2},
			{Type: "left", Color: "#006100", Style: 1},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("创建金额加粗样式: %w", err)
	}
	wb.moneyStyleThickID = moneyStyleThick

	// 设置页面布局（所有现有Sheet）
	setAllSheetPageLayout(wb.File)
	// 金额样式创建后再设置页面布局（不影响已有Sheet布局）

	return wb, nil
}

// setAllSheetPageLayout 为所有非多科目明细账 Sheet 设置 A4 横向、页边距 0、FitToWidth=1。
// 多科目明细账使用独立布局 setMLSheetPageLayout，避免被 FitToWidth 压缩左右两半。
func setAllSheetPageLayout(f *excelize.File) {
	for _, sheet := range f.GetSheetList() {
		if len(sheet) >= len(sheetPrefixML) && sheet[:len(sheetPrefixML)] == sheetPrefixML {
			continue // ML sheet 由 setMLSheetPageLayout 单独处理
		}
		if len(sheet) >= len(sheetPrefixDetail) && sheet[:len(sheetPrefixDetail)] == sheetPrefixDetail {
			continue // 独立明细账页（ML 家族分离形态，ML 样式）由 setMLSheetPageLayout 单独处理
		}
		if len(sheet) >= len(sheetPrefixGL) && sheet[:len(sheetPrefixGL)] == sheetPrefixGL {
			// GL（含合并 GL）：B5 横向、固定缩放 74%、边距 0、显式分页
			paperSize := 13 // B5 (JIS)
			scale := uint(74)
			fw := 0
			fh := 0
			fp := false
			f.SetPageLayout(sheet, &excelize.PageLayoutOptions{
				Orientation: stringPtr("landscape"),
				Size:        &paperSize,
				AdjustTo:    &scale,
				FitToWidth:  &fw,
				FitToHeight: &fh,
			})
			f.SetPageMargins(sheet, &excelize.PageLayoutMarginsOptions{
				Top:    float64Ptr(0),
				Bottom: float64Ptr(0),
				Left:   float64Ptr(0),
				Right:  float64Ptr(0),
			})
			f.SetSheetProps(sheet, &excelize.SheetPropsOptions{
				FitToPage: &fp,
			})
			// 垂直分页：反面书口列前分页（正面+正面书口 / 反面书口+反面数据各一张纸）
			lay := glLayout()
			pbCell, _ := excelize.ColumnNumberToName(lay.PageGapStartCol + 1)
			f.InsertPageBreak(sheet, pbCell+"1")
			continue
		}
		// 其他 sheet：A4 横向、页边距 0、FitToWidth=1
		paperSize := 9
		fw := 1
		fh := 0
		fp := true
		f.SetPageLayout(sheet, &excelize.PageLayoutOptions{
			Orientation: stringPtr("landscape"),
			Size:        &paperSize,
			FitToWidth:  &fw,
			FitToHeight: &fh,
		})
		f.SetPageMargins(sheet, &excelize.PageLayoutMarginsOptions{
			Top:    float64Ptr(0),
			Bottom: float64Ptr(0),
			Left:   float64Ptr(0),
			Right:  float64Ptr(0),
		})
		f.SetSheetProps(sheet, &excelize.SheetPropsOptions{
			FitToPage: &fp,
		})
	}
}

// prevMonthPath 返回上月 xlsx 路径。
func (wb *Workbook) prevMonthPath() string {
	prev := prevMonth(wb.Month)
	if prev == "" {
		return ""
	}
	return filepath.Join(wb.OutputDir, prev+".xlsx")
}

// currentPath 返回本月 xlsx 路径。
func (wb *Workbook) currentPath() string {
	return filepath.Join(wb.OutputDir, wb.Month+".xlsx")
}

// hasSheet 判断工作薄中是否存在指定 Sheet。
func (wb *Workbook) hasSheet(name string) bool {
	for _, s := range wb.File.GetSheetList() {
		if s == name {
			return true
		}
	}
	return false
}

// Save 保存工作薄到本月文件。
func (wb *Workbook) Save() error {
	// 清理默认 "Sheet1" — 此时已有其他 sheet，可以安全删除
	if len(wb.File.GetSheetList()) > 1 {
		wb.File.DeleteSheet("Sheet1")
	}
	// 多科目明细账独立布局（左右各一张 A4，关闭 FitToWidth）
	setMLSheetPageLayout(wb.File)
	return wb.File.SaveAs(wb.currentPath())
}

// ExtractLastMonthFinals 从各总分类账 Sheet 的"期末余额"行提取余额。
func (wb *Workbook) ExtractLastMonthFinals() (map[string]int64, error) {
	lay := glLayout()
	finals := make(map[string]int64)
	// 第三轮审查 D1b：合并总账科目的 sheet 是合并视图（非账页），不参与上月期末提取，
	// 否则父级期初被合并期末污染（实测 2026-02 银行存款期初 8000，应为 5000）。
	mergeSet := make(map[string]bool)
	if wb.Config != nil {
		for _, g := range wb.Config.Settings.MergeGLAccounts {
			mergeSet[g] = true
		}
	}
	for _, name := range wb.File.GetSheetList() {
		if !strings.HasPrefix(name, sheetPrefixGL) {
			continue
		}
		account := strings.TrimPrefix(name, sheetPrefixGL)
		if mergeSet[account] {
			continue
		}
		rows, err := wb.File.GetRows(name)
		if err != nil {
			continue
		}
		// 优先找最后一个"期末余额"行（月结行，权威运行余额）。
		// 移除 M4 补月结后，无发生额月份的账页只有上年结转/期初行或分录行，没有
		// 期末余额行——此时回退取账页上最后一条带符号余额的行：无发生额月份不改
		// 变余额，账页上最后的余额即科目当前余额（借正贷负）。
		var lastBalance int64
		foundClosing := false
		for _, row := range rows {
			if glRowLabel(row, lay) == periodEndLabel {
				if v, ok := glRowSignedBalance(row, lay); ok {
					lastBalance = v
					foundClosing = true
				}
			}
		}
		if !foundClosing {
			for _, row := range rows {
				if v, ok := glRowSignedBalance(row, lay); ok {
					lastBalance = v
				}
			}
		}
		finals[account] = lastBalance
	}
	return finals, nil
}

// sheet naming constants
const (
	sheetPrefixGL     = "总分类账-"
	sheetPrefixML     = "多科目明细账-"
	sheetPrefixDetail = "明细账-"
	pageBreakLabel    = "过    次    页"
	periodEndLabel    = "期末余额"
)

const pageSize = 20

// yuanStrToCents 将 "1234.56" 格式字符串转为分。
func yuanStrToCents(s string) (int64, error) {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")
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

// sheetNameDetail 返回明细科目独立账页 Sheet 名称（全局设置.明细账独立科目）。
func sheetNameDetail(account string) string {
	return sheetPrefixDetail + account
}

// detailSplit 报告科目（叶子全路径）是否配置为分离明细账页（ML 家族分离形态）。
// 条目双匹配（对仗 总分类账忽略科目 先例）：
//   - 总账科目名（如 "公益支出"）→ 其下所有明细科目各自分离成独立账页；
//   - 叶子全路径（如 "公益支出-补助费用"）→ 单个明细分离。
//
// 仅带明细段的路径（叶子）可命中——总账直记路径（无明细段）不建分离页。
func (wb *Workbook) detailSplit(account string) bool {
	i := strings.IndexByte(account, '-')
	if i <= 0 {
		return false
	}
	general := account[:i]
	for _, s := range wb.Config.Settings.DetailSplit {
		if s == "" {
			continue
		}
		if s == account || s == general {
			return true
		}
	}
	return false
}

// detailSplitSet 返回分离配置集合原样（供按分录批量路由与守卫使用）。
func (wb *Workbook) detailSplitSet() []string {
	return wb.Config.Settings.DetailSplit
}

// detailCreditBased 报告总账科目是否以贷方向为基准（收入/负债/权益类）。
// 判定来源：科目树中该总账下叶子的科目属性（subjects import 用户确认值）优先，
// 无叶子时按官方科目表/大类推断（balance.InferPropertyByType，含备抵科目贷余特例）。
// 结果按总账名缓存（树遍历一次）。
func (wb *Workbook) detailCreditBased(general string) bool {
	wb.detailCreditOnce.Do(func() {
		cache := make(map[string]bool)
		for leaf, node := range wb.Config.Tree {
			i := strings.IndexByte(leaf, '-')
			if i <= 0 {
				continue
			}
			g := leaf[:i]
			if _, seen := cache[g]; !seen {
				cache[g] = node.Property == "贷"
			}
		}
		wb.detailCreditCache = cache
	})
	if v, ok := wb.detailCreditCache[general]; ok {
		return v
	}
	return balance.InferPropertyByType(general) == "贷"
}

// detailColNet 明细列显示净额：以科目基准方向为正——贷方向科目（收入/负债/权益）
// = 贷-借，其余 = 借-贷。负值 = 与基准方向相反的发生额（冲减/红字冲销），
// 打印版按负数红字标记（审计 H2：手工账红笔惯例）——方向定向后红字语义即正确。
func (wb *Workbook) detailColNet(general string, debit, credit int64) int64 {
	net := debit - credit
	if wb.detailCreditBased(general) {
		net = -net
	}
	return net
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

func float64Ptr(f float64) *float64 {
	return &f
}

// setMoneyStyle 对指定单元格应用金额数字格式 #,##0.00。
func (wb *Workbook) setMoneyStyle(sheet string, row, col int) {
	cell, _ := excelize.CoordinatesToCellName(col, row)
	wb.File.SetCellStyle(sheet, cell, cell, wb.moneyStyleID)
}

// setMoneyStyleThick 对指定单元格应用金额数字格式 #,##0.00（底边加粗）。
func (wb *Workbook) setMoneyStyleThick(sheet string, row, col int) {
	cell, _ := excelize.CoordinatesToCellName(col, row)
	wb.File.SetCellStyle(sheet, cell, cell, wb.moneyStyleThickID)
}
