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
	loaded := &PrintConfig{}
	if err := json.Unmarshal(data, loaded); err != nil {
		return fmt.Errorf("解析打印版配置 %s: %w", path, err)
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
