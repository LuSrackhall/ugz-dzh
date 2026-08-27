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

// PrintConfig 打印版配置（英文键，可直接 json.Unmarshal）。
type PrintConfig struct {
	Platforms map[string]PlatformConfig `json:"platforms"`
}

// PlatformConfig 单平台配置（补偿系数 + 分区域字体）。
type PlatformConfig struct {
	ColScale float64    `json:"colScale"`
	RowScale float64    `json:"rowScale"`
	Fonts    FontConfig `json:"fonts"`
}

// FontConfig 分区域字体（normal=列宽基准；digit=金额数字；title=大标题；default=其余）。
type FontConfig struct {
	Normal  string `json:"normal"`
	Digit   string `json:"digit"`
	Title   string `json:"title"`
	Default string `json:"default"`
}

// printCfg 全局打印版配置（默认值=当前标定行为）。
var printCfg = defaultPrintConfig()

func defaultFonts() FontConfig {
	return FontConfig{Normal: "Calibri", Digit: "Noteworthy", Title: "仿宋", Default: "宋体"}
}

func defaultPrintConfig() *PrintConfig {
	cfg := &PrintConfig{Platforms: map[string]PlatformConfig{}}
	// Windows：补偿系数为肉眼标定收敛值（2026-08-28），Mac 恒 1.0
	cfg.Platforms["windows"] = PlatformConfig{ColScale: 1.1075, RowScale: 0.992, Fonts: defaultFonts()}
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

// currentFonts 当前平台的分区域字体。
func currentFonts() FontConfig {
	return platformConfig().Fonts
}

// CurrentConfigSummary 当前平台配置摘要（供 generate 打印确认加载状态）。
func CurrentConfigSummary() string {
	plat := currentPlatform()
	cfg := platformConfig()
	return fmt.Sprintf("平台=%s 列宽系数=%.4f 行高系数=%.4f 字体(normal=%s digit=%s title=%s default=%s)",
		plat, cfg.ColScale, cfg.RowScale, cfg.Fonts.Normal, cfg.Fonts.Digit, cfg.Fonts.Title, cfg.Fonts.Default)
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
		printCfg.Platforms[name] = base
	}
	return nil
}
