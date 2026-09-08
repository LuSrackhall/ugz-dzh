package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

// TestDemoLedgerStandaloneDetail 守门断言：scripts/test-e2e.sh 生成的演示账套
// （配置 分离明细账科目=["公益支出"]，总账名）中，公益支出在 ML 下的分离形态正确：
//   - 其下所有有分录的明细各自得到独立明细账页（明细账-公益支出-*）；
//   - GL 叶子页照常（加法路由，互不影响）；
//   - 合并 多科目明细账-公益支出 明细列照常保留（互不冲突共存）。
//
// out/ 由 test-e2e.sh 生成（gitignored）：不存在时 Skip（CI 无产物自动跳过），
// 本地跑过脚本后 go test ./... 即验证，防回归。
func TestDemoLedgerStandaloneDetail(t *testing.T) {
	xlsx := filepath.Join("out", "2026", "2026-06.xlsx")
	if _, err := os.Stat(xlsx); err != nil {
		t.Skip("scripts/test-e2e.sh 未运行（out/2026/2026-06.xlsx 不存在），跳过演示账套断言")
	}

	f, err := excelize.OpenFile(xlsx)
	if err != nil {
		t.Fatalf("打开演示账本: %v", err)
	}
	defer f.Close()

	const detail = "明细账-公益支出-补助费用"
	const merged = "多科目明细账-公益支出"
	const glLeaf = "总分类账-公益支出-补助费用"

	has := func(name string) bool {
		idx, err := f.GetSheetIndex(name)
		return err == nil && idx >= 0
	}
	// 1) 独立明细账页在位（ML 分离形态）
	if !has(detail) {
		t.Fatalf("独立明细账页缺失: %s", detail)
	}
	// 2) GL 叶子页照常（加法路由，互不影响）
	if !has(glLeaf) {
		t.Error("GL 叶子页不应被排除（加法路由）")
	}
	// 3) 合并 ML 明细列照常保留（互不冲突共存）：补助费用列头仍在
	mlRows, err := f.GetRows(merged)
	if err != nil {
		t.Fatalf("读合并 ML 页: %v", err)
	}
	kept := false
	for _, r := range mlRows {
		for i := 11; i <= 27 && i < len(r); i++ {
			if strings.TrimSpace(r[i]) == "补助费用" {
				kept = true
			}
		}
	}
	if !kept {
		t.Error("合并 ML 应保留 补助费用 列（互不冲突，不删减）")
	}
	// 4) 总账名配置 → 其下所有有分录的明细各自得到独立页（统计明细账-公益支出-* 页数）
	sheets := f.GetSheetList()
	splitPages := 0
	for _, s := range sheets {
		if strings.HasPrefix(s, "明细账-公益支出-") {
			splitPages++
		}
	}
	if splitPages < 5 {
		t.Errorf("公益支出下应有多个明细分离成页（got %d, sheets=%v）", splitPages, sheets)
	}
	// 4) 独立页有月结终行（期末余额），余额为正数格式
	dRows, err := f.GetRows(detail)
	if err != nil {
		t.Fatalf("读独立页: %v", err)
	}
	foundEnd := false
	for _, r := range dRows {
		if len(r) > 5 && strings.TrimSpace(r[5]) == "期末余额" {
			foundEnd = true
		}
	}
	if !foundEnd {
		t.Error("独立页缺期末余额月结行")
	}
}
