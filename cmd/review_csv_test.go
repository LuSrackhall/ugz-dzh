package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"ledger/balance"
)

func writeTempCSV(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "建账审核表.csv")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParseReviewCSV(t *testing.T) {
	rows, err := parseReviewCSV(writeTempCSV(t, "\ufeff科目,方向,期初余额,备注\n银行存款-工商银行,借,152300.00,对公户\n\n库存现金,借,2000,\n管理费用,借,,无期初\n"))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("行数 = %d, want 3", len(rows))
	}
	if rows[0].Account != "银行存款-工商银行" || rows[0].Direction != "借" || rows[0].OpeningYuan != 152300 || !rows[0].OpeningSet {
		t.Errorf("rows[0] 解析错误: %+v", rows[0])
	}
	if rows[1].OpeningYuan != 2000 {
		t.Errorf("整数金额解析错误: %+v", rows[1])
	}
	if rows[2].OpeningSet {
		t.Errorf("空期初余额不应置 OpeningSet: %+v", rows[2])
	}
}

func TestParseReviewCSVColumnOrderFree(t *testing.T) {
	rows, err := parseReviewCSV(writeTempCSV(t, "方向,备注,科目,期初余额\n贷,x,资本,111100.00\n"))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if rows[0].Account != "资本" || rows[0].Direction != "贷" || rows[0].OpeningYuan != 111100 {
		t.Errorf("乱序列解析错误: %+v", rows[0])
	}
}

func TestParseReviewCSVErrors(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"缺方向列", "科目,期初余额\n库存现金,100\n"},
		{"空方向（scan 未确认）", "科目,方向\n库存现金,\n"},
		{"方向无效", "科目,方向\n库存现金,unknown\n"},
		{"科目重复", "科目,方向\n资本,贷\n资本,贷\n"},
		{"期初为负", "科目,方向,期初余额\n应付款,贷,-500\n"},
		{"空文件", ""},
		{"仅表头", "科目,方向\n"},
	}
	for _, c := range cases {
		if _, err := parseReviewCSV(writeTempCSV(t, c.content)); err == nil {
			t.Errorf("%s: 期望报错但通过了", c.name)
		}
	}
}

func TestGeneralOf(t *testing.T) {
	if got := generalOf("银行存款-工商银行"); got != "银行存款" {
		t.Errorf("generalOf 带明细 = %q", got)
	}
	if got := generalOf("库存现金"); got != "库存现金" {
		t.Errorf("generalOf 无明细 = %q", got)
	}
}

// TestValidateReviewRowMergeParent 合并总账父级：父级本身拒绝，子科目合法。
func TestValidateReviewRowMergeParent(t *testing.T) {
	cfg := &balance.GlobalConfig{}
	cfg.Settings.MergeGLAccounts = []string{"应付款"}
	if err := validateReviewRow(cfg, reviewRow{Account: "应付款"}); err == nil {
		t.Error("合并父级本身应被拒绝")
	}
	if err := validateReviewRow(cfg, reviewRow{Account: "应付款-养老金"}); err != nil {
		t.Errorf("合并父级的子科目应放行: %v", err)
	}
	if err := validateReviewRow(cfg, reviewRow{Account: "库存现金"}); err != nil {
		t.Errorf("非合并科目应放行: %v", err)
	}
}

func TestSubjectsCommandRegistered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"subjects", "import"})
	if err != nil || cmd == nil || cmd.Name() != "import" {
		t.Fatalf("subjects import 未注册: %v %v", cmd, err)
	}
	if _, _, err := rootCmd.Find([]string{"subjects", "list"}); err != nil {
		t.Errorf("subjects list 未注册: %v", err)
	}
	if _, _, err := rootCmd.Find([]string{"subjects", "export"}); err != nil {
		t.Errorf("subjects export 未注册: %v", err)
	}
}

func TestOpeningCommandRegistered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"opening", "import"})
	if err != nil || cmd == nil || cmd.Name() != "import" {
		t.Fatalf("opening import 未注册: %v %v", cmd, err)
	}
}
