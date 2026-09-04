package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ledger/balance"
)

// TestCheckUnclassifiedStats v2 §2.5 可观测化：check 输出未分类统计行
// （N 个 + 余额合计）与逐科目余额提示。
func TestCheckUnclassifiedStats(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "2026.json")
	cfg := &balance.GlobalConfig{
		Settings: balance.GlobalSettings{StartMonth: "2026-01"},
		Tree: map[string]balance.AccountNode{
			"库存现金": {Property: "借", Balances: map[string]balance.MonthBalance{"2026-01": {Final: 100000}}},
			"管埋费用": {Property: "未分类", Balances: map[string]balance.MonthBalance{"2026-01": {Final: -5000}}},
		},
		AutoItems:   []balance.AutoItem{{Account: "库存现金", FirstMonth: "2026-01"}},
		ManualItems: []balance.ManualItem{{Account: "管埋费用", EffectiveMonth: "2026-01"}},
	}
	if err := balance.SaveConfig(configPath, cfg); err != nil {
		t.Fatal(err)
	}

	// 捕获 stdout（cmd 输出走 fmt → os.Stdout）；并发排水防大输出阻塞管道
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	var buf bytes.Buffer
	drained := make(chan struct{})
	go func() {
		_, _ = io.Copy(&buf, r)
		close(drained)
	}()
	execErr := runCmd(t, "check", "-j", configPath)
	w.Close()
	os.Stdout = old
	<-drained
	out := buf.String()

	if execErr != nil {
		t.Fatalf("check 失败: %v\n%s", execErr, out)
	}
	if !strings.Contains(out, "未分类科目 1 个") || !strings.Contains(out, "-50.00") {
		t.Errorf("check 缺少未分类统计行:\n%s", out)
	}
	if !strings.Contains(out, "管埋费用") || !strings.Contains(out, "2026-01 余额 -50.00 元") {
		t.Errorf("check 缺少逐科目余额提示:\n%s", out)
	}
}
