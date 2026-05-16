package voucher

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFile(t *testing.T) {
	tmp := t.TempDir()

	tests := []struct {
		name      string
		content   string
		wantCount int
		wantErr   bool
	}{
		{
			name: "multi-voucher table",
			content: `记字第0001号
2025年07月03日
<table>
<tr><td>摘要</td><td>总账科目</td><td>明细科目</td><td>借方</td><td>贷方</td></tr>
<tr><td>提现</td><td>库存现金</td><td></td><td>10,000.00</td><td></td></tr>
<tr><td>提现</td><td>银行存款</td><td>工行</td><td></td><td>10,000.00</td></tr>
</table>`,
			wantCount: 2,
		},
		{
			name:      "no tables",
			content:   `# 凭证\n\n本月无交易。`,
			wantCount: 0,
		},
		{
			name: "amount with Chinese comma",
			content: `记字第0002号
2025年08月15日
<table>
<tr><td>摘要</td><td>总账科目</td><td>明细科目</td><td>借方</td><td>贷方</td></tr>
<tr><td>购办公用品</td><td>管理费用</td><td>办公费</td><td>1，234.56</td><td></td></tr>
</table>`,
			wantCount: 1,
		},
		{
			name: "file with header only",
			content: `记字第0003号
<table>
<tr><td>摘要</td><td>总账科目</td><td>明细科目</td><td>借方</td><td>贷方</td></tr>
</table>`,
			wantCount: 0,
		},
		{
			name: "voucher number from filename fallback",
			content: `2025年09月01日
<table>
<tr><td>摘要</td><td>总账科目</td><td>明细科目</td><td>借方</td><td>贷方</td></tr>
<tr><td>收押金</td><td>库存现金</td><td></td><td>5,000.00</td><td></td></tr>
</table>`,
			wantCount: 1,
		},
		{
			name: "all zero amounts skipped",
			content: `记字第0004号
<table>
<tr><td>摘要</td><td>总账科目</td><td>明细科目</td><td>借方</td><td>贷方</td></tr>
<tr><td>空行</td><td>库存现金</td><td></td><td>0</td><td></td></tr>
</table>`,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use filename with a number in it for the fallback test
			fname := "test.md"
			if tt.name == "voucher number from filename fallback" {
				fname = "voucher_0042.md"
			}
			path := filepath.Join(tmp, fname)
			if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
				t.Fatalf("write test file: %v", err)
			}

			entries, err := ParseFile(path)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if len(entries) != tt.wantCount {
				t.Errorf("got %d entries, want %d", len(entries), tt.wantCount)
			}

			// Verify specific data for multi-voucher test
			if tt.name == "multi-voucher table" && len(entries) == 2 {
				e0 := entries[0]
				if e0.GeneralAccount != "库存现金" || e0.DebitCents != 1000000 {
					t.Errorf("first entry: account=%q debit=%d, want 库存现金/1000000", e0.GeneralAccount, e0.DebitCents)
				}
				e1 := entries[1]
				if e1.GeneralAccount != "银行存款" || e1.DetailAccount != "工行" || e1.CreditCents != 1000000 {
					t.Errorf("second entry: account=%q detail=%q credit=%d, want 银行存款/工行/1000000", e1.GeneralAccount, e1.DetailAccount, e1.CreditCents)
				}
			}

			// Verify Chinese comma amount
			if tt.name == "amount with Chinese comma" && len(entries) == 1 {
				if entries[0].DebitCents != 123456 {
					t.Errorf("Chinese comma amount: got %d cents, want 123456", entries[0].DebitCents)
				}
			}

			// Verify filename fallback
			if tt.name == "voucher number from filename fallback" && len(entries) == 1 {
				if entries[0].VoucherNum != 42 {
					t.Errorf("filename fallback: got voucher num %d, want 42", entries[0].VoucherNum)
				}
			}
		})
	}
}

func TestParseAmountToCents(t *testing.T) {
	tests := []struct {
		input    string
		want     int64
		wantOk   bool
	}{
		{"1,234.56", 123456, true},
		{"1，234.56", 123456, true},
		{"0", 0, true},
		{"", 0, false},
		{"   ", 0, false},
		{"-500.00", -50000, true},
		{"100", 10000, true},
		{"1.5", 150, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := parseAmountToCents(tt.input)
			if ok != tt.wantOk {
				t.Errorf("ok=%v, want %v", ok, tt.wantOk)
			}
			if got != tt.want {
				t.Errorf("cents=%d, want %d", got, tt.want)
			}
		})
	}
}

func TestCleanDetail(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"  张三  ", "张三"},
		{"李　四", "李 四"},
		{"王  五", "王 五"},
		{"", ""},
		{"  ", ""},
		{"赵六", "赵六"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := cleanDetail(tt.input)
			if got != tt.expected {
				t.Errorf("cleanDetail(%q)=%q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestExtractVoucherNum(t *testing.T) {
	tests := []struct {
		text string
		want int
	}{
		{"记字第0001号", 1},
		{"记字第0042号 1/1", 42},
		{"凭证号码 0005", 5},
		{"无凭证号文本", 0},
		{"", 0},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			got := extractVoucherNum(tt.text)
			if got != tt.want {
				t.Errorf("extractVoucherNum(%q)=%d, want %d", tt.text, got, tt.want)
			}
		})
	}
}
