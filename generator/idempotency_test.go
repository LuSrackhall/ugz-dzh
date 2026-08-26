package generator

import (
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestAlreadyGenerated(t *testing.T) {
	dir := t.TempDir()

	// 含"本月合计"的账本 → true
	with := filepath.Join(dir, "with.xlsx")
	f := excelize.NewFile()
	f.SetCellValue("Sheet1", "A1", "本月合计")
	if err := f.SaveAs(with); err != nil {
		t.Fatalf("save: %v", err)
	}
	f.Close()
	got, err := AlreadyGenerated(with)
	if err != nil {
		t.Fatalf("AlreadyGenerated(with): %v", err)
	}
	if !got {
		t.Error("含本月合计应返回 true")
	}

	// 空工作薄（year-close 预生成）→ false
	empty := filepath.Join(dir, "empty.xlsx")
	f2 := excelize.NewFile()
	if err := f2.SaveAs(empty); err != nil {
		t.Fatalf("save: %v", err)
	}
	f2.Close()
	got, err = AlreadyGenerated(empty)
	if err != nil {
		t.Fatalf("AlreadyGenerated(empty): %v", err)
	}
	if got {
		t.Error("空工作薄应返回 false")
	}

	// 不存在的文件 → 返回错误
	if _, err := AlreadyGenerated(filepath.Join(dir, "nope.xlsx")); err == nil {
		t.Error("不存在的文件应返回错误")
	}
}
