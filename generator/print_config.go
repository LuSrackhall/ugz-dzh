// Package generator — 打印版配置文件（print-config.json，可选）。
//
// 用途：把跨平台标定系数与各区域字体从代码硬编码中解放出来，用户/部署时
// 直接改 JSON 即可调整（无需改代码重新编译发版）。配置按平台分组，
// 每个平台可独立设置补偿系数与字体。
// 缺省使用默认值 = 当前标定行为。
//
// 配置示例（generate --config print-config.json）：
//
//	{
//	  "platforms": {
//	    "windows": {
//	      "colScale": 1.1075, "rowScale": 0.992,
//	      "fonts": { "normal": "Calibri", "digit": "Noteworthy", "title": "仿宋", "default": "宋体" }
//	    },
//	    "mac": {
//	      "colScale": 1.0, "rowScale": 1.0,
//	      "fonts": { "normal": "Calibri", "digit": "Noteworthy", "title": "仿宋", "default": "宋体" }
//	    }
//	  }
//	}
//
// 字段说明：
//   - platforms.<平台>.colScale / rowScale：该平台打印版列宽/行高补偿系数
//     （解决 WPS 各平台/机器渲染尺寸不一致；Windows 默认 1.1075/0.992 为肉眼标定值）
//   - fonts.normal：Normal 默认字体（列宽像素计算基准，可统一两端用同一字体文件如 Arimo）
//   - fonts.digit：数据区金额数字字体
//   - fonts.title：大标题（总分类账/明细分户帐）字体
//   - fonts.default：表头/标签/摘要等其余区域字体
package generator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
)

// printSheetType 当前正在变换的账本类型（"gl"/"ml"/""=平台级），
// 由 TransformToPrint 在遍历 sheet 时设置，驱动 GL/ML 分账本配置。
var printSheetType = ""

// PrintConfig 打印版配置（英文键，可直接 json.Unmarshal）。
type PrintConfig struct {
	Platforms map[string]PlatformConfig `json:"platforms"`
}

// PlatformConfig 单平台配置（补偿系数 + 分区域字体 + 可选 GL/ML 分账本覆盖）。
type PlatformConfig struct {
	ColScale float64      `json:"colScale"`
	RowScale float64      `json:"rowScale"`
	Fonts    FontConfig   `json:"fonts"`
	GL       *SheetConfig `json:"gl"` // 总分类账专用（可选；缺省用平台级）
	ML       *SheetConfig `json:"ml"` // 多科目明细账专用（可选；缺省用平台级）
}

// SheetConfig 单账本类型（GL/ML）覆盖配置：系数/字体未填时回退平台级。
type SheetConfig struct {
	ColScale float64    `json:"colScale"`
	RowScale float64    `json:"rowScale"`
	Fonts    FontConfig `json:"fonts"`
}

// empty 判断 sheet 配置是否全空（模板里 "gl": {} 视为未配置）。
func (s *SheetConfig) empty() bool {
	return s == nil || (s.ColScale == 0 && s.RowScale == 0 && s.Fonts == (FontConfig{}))
}

// FontConfig 分区域字体（normal=列宽基准；digit=金额数字；title=大标题；default=其余）。
type FontConfig struct {
	Normal  string `json:"normal"`
	Digit   string `json:"digit"`
	Title   string `json:"title"`
	Default string `json:"default"`
	// 金额区域列数字：字号/加粗（0/null=现状：GL 7pt / ML 6pt，不加粗）
	DigitSize float64 `json:"digitSize"`
	DigitBold *bool   `json:"digitBold"`
	// 摘要/借/贷/余额表头：字号/加粗/字体（0/null/空=现状：GL 7pt / ML 6pt，加粗，宋体）。
	// labelFamily 非空时该表头字体用它覆盖（如 Windows 默认"等线 Light"）。
	LabelSize   float64 `json:"labelSize"`
	LabelBold   *bool   `json:"labelBold"`
	LabelFamily string  `json:"labelFamily"`
}

// printCfg 全局打印版配置（默认值=当前标定行为）。
var printCfg = defaultPrintConfig()

// defaultFonts 两端共用基础字体默认：Normal=Calibri（Mac 端原效果）。
// Windows 端 Normal 默认改宋体在 defaultPrintConfig 单独设置——两端隔离，互不影响。
func defaultFonts() FontConfig {
	return FontConfig{Normal: "Calibri", Digit: "Noteworthy", Title: "仿宋", Default: "宋体"}
}

func defaultPrintConfig() *PrintConfig {
	cfg := &PrintConfig{Platforms: map[string]PlatformConfig{}}
	// Windows：Normal 基础字体默认宋体（列宽像素基准，Win 中易宋体可解析）；
	// 摘要/借/贷/余额表头字体默认"等线 Light"（Win 系统字体，用户 2026-08-28 定）；
	// 平台级补偿为肉眼标定收敛值（1.1075/0.992）；GL 独立系数（1.13595/0.99495），ML 用平台级
	winFonts := defaultFonts()
	winFonts.Normal = "宋体"
	winFonts.LabelFamily = "等线 Light"
	cfg.Platforms["windows"] = PlatformConfig{
		ColScale: 1.1075, RowScale: 0.992, Fonts: winFonts,
		GL: &SheetConfig{ColScale: 1.13595, RowScale: 0.99495},
	}
	// Mac：保持原效果（Normal=Calibri，系数 1.0）——与 Windows 完全隔离
	cfg.Platforms["mac"] = PlatformConfig{ColScale: 1.0, RowScale: 1.0, Fonts: defaultFonts()}
	return cfg
}

// currentPlatform 当前目标平台（PrintPlatform 由 cmd 层设置；auto=当前系统）。
func currentPlatform() string {
	plat := PrintPlatform
	if plat == "" || plat == "auto" {
		plat = runtime.GOOS
	}
	return plat
}

// platformConfig 当前平台配置（未知平台回退 mac 默认，系数 1.0）。
func platformConfig() PlatformConfig {
	if cfg, ok := printCfg.Platforms[currentPlatform()]; ok {
		return cfg
	}
	return defaultPrintConfig().Platforms["mac"]
}

// mergeFonts 用 over 的非空字段覆盖 base，返回合并结果（sheet 级字体未填时回退平台级）。
func mergeFonts(base FontConfig, over FontConfig) FontConfig {
	if over.Normal != "" {
		base.Normal = over.Normal
	}
	if over.Digit != "" {
		base.Digit = over.Digit
	}
	if over.Title != "" {
		base.Title = over.Title
	}
	if over.Default != "" {
		base.Default = over.Default
	}
	if over.DigitSize != 0 {
		base.DigitSize = over.DigitSize
	}
	if over.DigitBold != nil {
		base.DigitBold = over.DigitBold
	}
	if over.LabelSize != 0 {
		base.LabelSize = over.LabelSize
	}
	if over.LabelBold != nil {
		base.LabelBold = over.LabelBold
	}
	if over.LabelFamily != "" {
		base.LabelFamily = over.LabelFamily
	}
	return base
}

// sheetConfig 当前账本类型（GL/ML）的覆盖配置；无则返回 nil。
func sheetConfig() *SheetConfig {
	cfg := platformConfig()
	switch printSheetType {
	case "gl":
		if cfg.GL.empty() {
			return nil
		}
		return cfg.GL
	case "ml":
		if cfg.ML.empty() {
			return nil
		}
		return cfg.ML
	}
	return nil
}

// currentFonts 当前平台的分区域字体（GL/ML 有专用字体时优先，未填项回退平台级）。
func currentFonts() FontConfig {
	base := platformConfig().Fonts
	if sc := sheetConfig(); sc != nil {
		return mergeFonts(base, sc.Fonts)
	}
	return base
}

// sheetCompensate 当前账本类型的列宽/行高补偿系数（GL/ML 专用系数优先，0 回退平台级）。
func sheetCompensate() (colScale, rowScale float64) {
	cfg := platformConfig()
	colScale, rowScale = cfg.ColScale, cfg.RowScale
	if sc := sheetConfig(); sc != nil {
		if sc.ColScale != 0 {
			colScale = sc.ColScale
		}
		if sc.RowScale != 0 {
			rowScale = sc.RowScale
		}
	}
	return colScale, rowScale
}

// CurrentConfigSummary 当前平台配置摘要（供 generate 打印确认加载状态）。
func CurrentConfigSummary() string {
	plat := currentPlatform()
	cfg := platformConfig()
	s := fmt.Sprintf("平台=%s 列宽系数=%.4f 行高系数=%.4f 字体(normal=%s digit=%s title=%s default=%s)",
		plat, cfg.ColScale, cfg.RowScale, cfg.Fonts.Normal, cfg.Fonts.Digit, cfg.Fonts.Title, cfg.Fonts.Default)
	// GL/ML 分账本覆盖（显示回退后的生效值）
	for _, st := range []struct {
		name string
		sc   *SheetConfig
	}{{"GL", cfg.GL}, {"ML", cfg.ML}} {
		if sc := st.sc; !sc.empty() {
			c, r := cfg.ColScale, cfg.RowScale
			if sc.ColScale != 0 {
				c = sc.ColScale
			}
			if sc.RowScale != 0 {
				r = sc.RowScale
			}
			s += fmt.Sprintf(" %s(列宽%.4f 行高%.4f)", st.name, c, r)
		}
	}
	return s
}

// LoadPrintConfig 从 JSON 文件加载打印版配置（可选，缺省用默认值）。
// 未配置的平台/字段保持默认值不变。
func LoadPrintConfig(path string) error {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取打印版配置 %s: %w", path, err)
	}
	// 容忍 UTF-8 BOM（Windows 记事本"UTF-8 with BOM"保存时产生）
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	// 防呆：检测旧版中文字段名（早期实现用中文键，Go json 对未知字段静默忽略→配置不生效）
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err == nil {
		for _, cn := range []string{"平台", "字体", "列宽系数", "行高系数", "基准", "数字", "标题", "默认"} {
			if _, ok := raw[cn]; ok {
				return fmt.Errorf("配置文件 %s 使用了中文字段名（如 %q）——旧格式已被废弃，json 会静默忽略导致配置不生效。请改用英文键：platforms.{windows,mac}.{colScale,rowScale,fonts.{normal,digit,title,default}}（字段说明见 docs/print-config.md）", path, cn)
			}
		}
	}
	loaded := &PrintConfig{}
	if err := json.Unmarshal(data, loaded); err != nil {
		return fmt.Errorf("解析打印版配置 %s: %w（请确认文件是 UTF-8 无 BOM 编码；JSON 不能有注释/尾逗号）", path, err)
	}
	// 合并：覆盖配置里出现的平台（字段非零覆盖，未配置项保持默认）
	for name, pc := range loaded.Platforms {
		base, ok := printCfg.Platforms[name]
		if !ok {
			base = defaultPrintConfig().Platforms["mac"] // 新平台从 mac 默认起
		}
		if pc.ColScale != 0 {
			base.ColScale = pc.ColScale
		}
		if pc.RowScale != 0 {
			base.RowScale = pc.RowScale
		}
		if pc.Fonts.Normal != "" {
			base.Fonts.Normal = pc.Fonts.Normal
		}
		if pc.Fonts.Digit != "" {
			base.Fonts.Digit = pc.Fonts.Digit
		}
		if pc.Fonts.Title != "" {
			base.Fonts.Title = pc.Fonts.Title
		}
		if pc.Fonts.Default != "" {
			base.Fonts.Default = pc.Fonts.Default
		}
		if pc.Fonts.DigitSize != 0 {
			base.Fonts.DigitSize = pc.Fonts.DigitSize
		}
		if pc.Fonts.DigitBold != nil {
			base.Fonts.DigitBold = pc.Fonts.DigitBold
		}
		if pc.Fonts.LabelSize != 0 {
			base.Fonts.LabelSize = pc.Fonts.LabelSize
		}
		if pc.Fonts.LabelBold != nil {
			base.Fonts.LabelBold = pc.Fonts.LabelBold
		}
		if pc.Fonts.LabelFamily != "" {
			base.Fonts.LabelFamily = pc.Fonts.LabelFamily
		}
		// GL/ML 分账本覆盖（全空视为未配置）
		if !pc.GL.empty() {
			sc := &SheetConfig{ColScale: pc.GL.ColScale, RowScale: pc.GL.RowScale, Fonts: mergeFonts(base.Fonts, pc.GL.Fonts)}
			base.GL = sc
		}
		if !pc.ML.empty() {
			sc := &SheetConfig{ColScale: pc.ML.ColScale, RowScale: pc.ML.RowScale, Fonts: mergeFonts(base.Fonts, pc.ML.Fonts)}
			base.ML = sc
		}
		printCfg.Platforms[name] = base
	}
	return nil
}
