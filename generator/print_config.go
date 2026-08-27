// Package generator — 打印版配置文件（print-config.json，可选）。
//
// 用途：把跨平台标定系数与各区域字体从代码硬编码中解放出来，用户/部署时
// 直接改 JSON 即可调整（无需改代码重新编译发版）。
// 缺省使用默认值 = 当前标定行为（Windows 列宽×1.1075/行高×0.992 等）。
//
// 配置示例（generate --config print-config.json）：
//
//	{
//	  "平台": {
//	    "windows": { "列宽系数": 1.1075, "行高系数": 0.992 },
//	    "mac":     { "列宽系数": 1.0,    "行高系数": 1.0 }
//	  },
//	  "字体": {
//	    "基准": "Calibri",     // Normal 默认字体（列宽基准）
//	    "数字": "Noteworthy",  // 数据区金额数字
//	    "标题": "仿宋",        // 大标题（总分类账/明细分户帐）
//	    "默认": "宋体"         // 表头/标签/摘要等其余区域
//	  }
//	}
package generator

import (
	"encoding/json"
	"fmt"
	"os"
)

// PrintConfig 打印版配置（字段名英文，JSON 键为中文）。
type PrintConfig struct {
	平台 PlatformConfig `json:"平台"`
	字体 FontConfig     `json:"字体"`
}

// PlatformConfig 各平台补偿系数。
type PlatformConfig struct {
	Windows ScaleConfig `json:"windows"`
	Mac     ScaleConfig `json:"mac"`
}

// ScaleConfig 单平台补偿系数。
type ScaleConfig struct {
	列宽系数 float64 `json:"列宽系数"`
	行高系数 float64 `json:"行高系数"`
}

// FontConfig 分区域字体（基准=Normal 列宽基准；数字=金额数字；标题=大标题；默认=其余）。
type FontConfig struct {
	基准 string `json:"基准"`
	数字 string `json:"数字"`
	标题 string `json:"标题"`
	默认 string `json:"默认"`
}

// printCfg 全局打印版配置（默认值=当前标定行为）。
var printCfg = defaultPrintConfig()

func defaultPrintConfig() *PrintConfig {
	cfg := &PrintConfig{}
	// 平台补偿系数：Windows 为肉眼标定收敛值（2026-08-28），Mac 恒 1.0
	cfg.平台.Windows.列宽系数 = 1.1075
	cfg.平台.Windows.行高系数 = 0.992
	cfg.平台.Mac.列宽系数 = 1.0
	cfg.平台.Mac.行高系数 = 1.0
	// 分区域字体：与打印版生成代码现状一致
	cfg.字体.基准 = "Calibri"
	cfg.字体.数字 = "Noteworthy"
	cfg.字体.标题 = "仿宋"
	cfg.字体.默认 = "宋体"
	return cfg
}

// LoadPrintConfig 从 JSON 文件加载打印版配置（可选，缺省用默认值）。
// 未配置的字段保持默认值不变。
// 注意：Go encoding/json 不支持中文 struct tag/字段名匹配（Go 1.17+），
// 故解析用 map[string]any 手动取中文字段（map 键支持任意 UTF-8 字符串）。
func LoadPrintConfig(path string) error {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取打印版配置 %s: %w", path, err)
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("解析打印版配置 %s: %w", path, err)
	}
	// 平台补偿系数
	if plat, ok := root["平台"].(map[string]any); ok {
		if win, ok := plat["windows"].(map[string]any); ok {
			if v, ok := win["列宽系数"].(float64); ok && v != 0 {
				printCfg.平台.Windows.列宽系数 = v
			}
			if v, ok := win["行高系数"].(float64); ok && v != 0 {
				printCfg.平台.Windows.行高系数 = v
			}
		}
		if mac, ok := plat["mac"].(map[string]any); ok {
			if v, ok := mac["列宽系数"].(float64); ok && v != 0 {
				printCfg.平台.Mac.列宽系数 = v
			}
			if v, ok := mac["行高系数"].(float64); ok && v != 0 {
				printCfg.平台.Mac.行高系数 = v
			}
		}
	}
	// 分区域字体
	if f, ok := root["字体"].(map[string]any); ok {
		if v, ok := f["基准"].(string); ok && v != "" {
			printCfg.字体.基准 = v
		}
		if v, ok := f["数字"].(string); ok && v != "" {
			printCfg.字体.数字 = v
		}
		if v, ok := f["标题"].(string); ok && v != "" {
			printCfg.字体.标题 = v
		}
		if v, ok := f["默认"].(string); ok && v != "" {
			printCfg.字体.默认 = v
		}
	}
	return nil
}

// printPlatformConfig 按目标平台返回补偿系数。
func printPlatformConfig(plat string) ScaleConfig {
	if plat == "windows" {
		return printCfg.平台.Windows
	}
	return printCfg.平台.Mac
}
