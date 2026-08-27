package generator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPrintConfig(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "print-config.json")
	content := `{"平台":{"windows":{"列宽系数":1.2,"行高系数":1.1}},"字体":{"基准":"Arimo","数字":"宋体"}}`
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	printCfg = defaultPrintConfig()
	if err := LoadPrintConfig(p); err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	if printCfg.平台.Windows.列宽系数 != 1.2 || printCfg.平台.Windows.行高系数 != 1.1 {
		t.Errorf("windows 系数未生效: %+v", printCfg.平台.Windows)
	}
	if printCfg.字体.基准 != "Arimo" || printCfg.字体.数字 != "宋体" {
		t.Errorf("字体未生效: %+v", printCfg.字体)
	}
	// 未配置项保持默认
	if printCfg.平台.Mac.列宽系数 != 1.0 || printCfg.字体.标题 != "仿宋" || printCfg.字体.默认 != "宋体" {
		t.Errorf("默认值被破坏: 平台=%+v 字体=%+v", printCfg.平台, printCfg.字体)
	}
}
