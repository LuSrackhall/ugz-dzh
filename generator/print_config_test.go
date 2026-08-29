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

func TestSheetConfigGLML(t *testing.T) {
	printCfg = defaultPrintConfig()
	// windows 平台：GL 专用系数/字体；ML 不配 → 回退平台级
	printCfg.Platforms["windows"] = PlatformConfig{
		ColScale: 1.1075, RowScale: 0.992,
		Fonts: FontConfig{Normal: "Calibri", Digit: "Noteworthy", Title: "仿宋", Default: "宋体"},
		GL: &SheetConfig{
			ColScale: 1.2,
			Fonts:    FontConfig{Digit: "宋体", Title: "黑体"},
		},
	}
	PrintPlatform = "windows"

	// GL：专用系数 + 字体（未填字段回退平台级）
	printSheetType = "gl"
	if c, r := sheetCompensate(); c != 1.2 || r != 0.992 {
		t.Errorf("GL 系数 = (%v,%v), want (1.2,0.992)", c, r)
	}
	f := currentFonts()
	if f.Digit != "宋体" || f.Title != "黑体" || f.Normal != "Calibri" {
		t.Errorf("GL 字体 = %+v, want digit=宋体 title=黑体 normal=Calibri(回退测试平台级)", f)
	}

	// ML：无专用配置 → 平台级
	printSheetType = "ml"
	if c, r := sheetCompensate(); c != 1.1075 || r != 0.992 {
		t.Errorf("ML 系数 = (%v,%v), want (1.1075,0.992)", c, r)
	}
	if got := currentFonts().Digit; got != "Noteworthy" {
		t.Errorf("ML digit = %v, want Noteworthy(平台级)", got)
	}

	// 非 GL/ML：平台级
	printSheetType = ""
	if c, _ := sheetCompensate(); c != 1.1075 {
		t.Errorf("平台级系数 = %v, want 1.1075", c)
	}
	printSheetType = ""
	PrintPlatform = "auto"
}

func TestLoadPrintConfigGLML(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "print-config.json")
	content := `{"platforms":{"windows":{"colScale":1.1,"gl":{"colScale":1.25,"fonts":{"digit":"宋体"}},"ml":{}}}}`
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	printCfg = defaultPrintConfig()
	if err := LoadPrintConfig(p); err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	w := printCfg.Platforms["windows"]
	if w.GL == nil || w.GL.ColScale != 1.25 || w.GL.Fonts.Digit != "宋体" {
		t.Errorf("GL 配置未生效: %+v", w.GL)
	}
	if w.GL.RowScale != 0 || w.GL.Fonts.Normal != "宋体" {
		t.Errorf("GL 未填字段应回退平台级默认: %+v", w.GL)
	}
	if w.ML != nil {
		t.Errorf("ml:{} 应视为未配置: %+v", w.ML)
	}
}

func TestFrontBackColScale(t *testing.T) {
	printCfg = defaultPrintConfig()
	printCfg.Platforms["windows"] = PlatformConfig{
		ColScale: 1.1075, RowScale: 0.992, Fonts: defaultFonts(),
		ML: &SheetConfig{ColScale: 1.1075, FrontColScale: 1.0, BackColScale: 1.2},
	}
	PrintPlatform = "windows"
	printSheetType = "ml"
	if f, b := sheetColScales(); f != 1.0 || b != 1.2 {
		t.Errorf("ML 正反面系数 = (%v,%v), want (1.0,1.2)", f, b)
	}
	// GL 未配正反面 → 0（用账本级）
	printSheetType = "gl"
	if f, b := sheetColScales(); f != 0 || b != 0 {
		t.Errorf("GL 无正反面配置应为 0: (%v,%v)", f, b)
	}
	printSheetType = ""
	PrintPlatform = "auto"
}

func TestLoadPrintConfigFrontBack(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "print-config.json")
	content := `{"platforms":{"windows":{"ml":{"colScale":1.1,"frontColScale":1.05,"backColScale":1.15}}}}`
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	printCfg = defaultPrintConfig()
	if err := LoadPrintConfig(p); err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	ml := printCfg.Platforms["windows"].ML
	if ml == nil || ml.FrontColScale != 1.05 || ml.BackColScale != 1.15 || ml.ColScale != 1.1 {
		t.Errorf("ML 正反面系数未生效: %+v", ml)
	}
}

func TestWidthDeltas(t *testing.T) {
	printCfg = defaultPrintConfig()
	printCfg.Platforms["windows"] = PlatformConfig{
		ColScale: 1.1075, RowScale: 0.992, Fonts: defaultFonts(),
		ML: &SheetConfig{ColScale: 1.1075, BackSummaryDelta: 5, FrontOuterDelta: -2},
	}
	PrintPlatform = "windows"
	printSheetType = "ml"
	wd := sheetWidthDeltas()
	if wd.BackSummary != 5 || wd.FrontOuter != -2 || wd.FrontSummary != 0 || wd.BackBinding != 0 {
		t.Errorf("ML 微调值 = %+v, want BackSummary=5 FrontOuter=-2 其余 0", wd)
	}
	// GL 未配 → 全 0（走原有逻辑，零影响）
	printSheetType = "gl"
	if wd := sheetWidthDeltas(); wd != (widthDeltas{}) {
		t.Errorf("GL 未配微调应为全 0: %+v", wd)
	}
	printSheetType = ""
	PrintPlatform = "auto"
}

func TestFrontDigitDelta(t *testing.T) {
	printCfg = defaultPrintConfig()
	printCfg.Platforms["windows"] = PlatformConfig{
		ColScale: 1.1075, RowScale: 0.992, Fonts: defaultFonts(),
		ML: &SheetConfig{ColScale: 1.1075, FrontDigitDelta: map[string]float64{"k9": 3, "base": 1}},
	}
	PrintPlatform = "windows"
	printSheetType = "ml"
	fd := sheetFrontDigitDelta()
	if fd == nil || fd["k9"] != 3 || fd["base"] != 1 || fd["k0"] != 0 {
		t.Errorf("frontDigitDelta = %v, want k9=3 base=1 k0=0", fd)
	}
	// GL 未配 → nil（不启用）
	printSheetType = "gl"
	if fd := sheetFrontDigitDelta(); fd != nil {
		t.Errorf("GL 未配 frontDigitDelta 应为 nil: %v", fd)
	}
	printSheetType = ""
	PrintPlatform = "auto"
}

func TestLoadPrintConfigFrontDigit(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "print-config.json")
	content := `{"platforms":{"windows":{"ml":{"frontDigitDelta":{"k4":2}}}}}`
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	printCfg = defaultPrintConfig()
	if err := LoadPrintConfig(p); err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	ml := printCfg.Platforms["windows"].ML
	if ml == nil || ml.FrontDigitDelta["k4"] != 2 {
		t.Errorf("frontDigitDelta 未生效: %+v", ml)
	}
}
