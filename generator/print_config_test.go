package generator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPrintConfig(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "print-config.json")
	// 仅覆盖 windows 平台的部分字段，mac 保持默认
	content := `{"platforms":{"windows":{"colScale":1.2,"rowScale":1.1,"fonts":{"normal":"Arimo","digit":"宋体"}}}}`
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	printCfg = defaultPrintConfig()
	if err := LoadPrintConfig(p); err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	// windows 被覆盖
	if got := printCfg.Platforms["windows"].ColScale; got != 1.2 {
		t.Errorf("windows colScale = %v, want 1.2", got)
	}
	if got := printCfg.Platforms["windows"].Fonts.Normal; got != "Arimo" {
		t.Errorf("windows fonts.normal = %v, want Arimo", got)
	}
	// windows 未配置字段保持默认
	if got := printCfg.Platforms["windows"].Fonts.Title; got != "仿宋" {
		t.Errorf("windows fonts.title = %v, want 仿宋", got)
	}
	// mac 完全保持默认
	if got := printCfg.Platforms["mac"].ColScale; got != 1.0 {
		t.Errorf("mac colScale = %v, want 1.0", got)
	}
}

func TestCurrentFontsByPlatform(t *testing.T) {
	printCfg = defaultPrintConfig()
	// windows 平台独立字体
	printCfg.Platforms["windows"] = PlatformConfig{
		ColScale: 1.1075, RowScale: 0.992,
		Fonts: FontConfig{Normal: "Calibri", Digit: "宋体", Title: "仿宋", Default: "宋体"},
	}
	PrintPlatform = "windows"
	if got := currentFonts().Digit; got != "宋体" {
		t.Errorf("windows digit = %v, want 宋体", got)
	}
	PrintPlatform = "mac"
	if got := currentFonts().Digit; got != "Noteworthy" {
		t.Errorf("mac digit = %v, want Noteworthy", got)
	}
	PrintPlatform = "auto"
}
