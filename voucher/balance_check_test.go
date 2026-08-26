package voucher

import (
	"strings"
	"testing"
)

func TestValidateVoucherBalance(t *testing.T) {
	cases := []struct {
		name        string
		entries     []Entry
		wantErr     bool
		errContains string
		minWarnings int
	}{
		{
			name: "平衡凭证通过",
			entries: []Entry{
				{Date: "2026-01-05", VoucherNum: 3, DebitCents: 50000, CreditCents: 0},
				{Date: "2026-01-05", VoucherNum: 3, DebitCents: 0, CreditCents: 50000},
			},
			wantErr: false,
		},
		{
			name: "不平衡凭证被拒且含差额",
			entries: []Entry{
				{Date: "2026-01-05", VoucherNum: 3, DebitCents: 50000, CreditCents: 0},
				{Date: "2026-01-05", VoucherNum: 3, DebitCents: 0, CreditCents: 30000},
			},
			wantErr:     true,
			errContains: "记字第3号",
		},
		{
			name: "红字按绝对值口径通过且提示",
			entries: []Entry{
				{Date: "2026-01-05", VoucherNum: 3, DebitCents: 50000, CreditCents: 0},
				{Date: "2026-01-05", VoucherNum: 3, DebitCents: 0, CreditCents: -50000},
			},
			wantErr:     false,
			minWarnings: 1,
		},
		{
			name: "红字不修改原值",
			entries: []Entry{
				{Date: "2026-01-05", VoucherNum: 3, DebitCents: 50000, CreditCents: 0},
				{Date: "2026-01-05", VoucherNum: 3, DebitCents: 0, CreditCents: -50000},
			},
			wantErr: false,
		},
		{
			name: "凭证号未解析跳过分组校验",
			entries: []Entry{
				{Date: "2026-01-05", VoucherNum: 0, DebitCents: 10000, CreditCents: 0},
			},
			wantErr:     false,
			minWarnings: 1,
		},
		{
			name: "不同凭证互不干扰",
			entries: []Entry{
				{Date: "2026-01-05", VoucherNum: 1, DebitCents: 10000, CreditCents: 0},
				{Date: "2026-01-05", VoucherNum: 1, DebitCents: 0, CreditCents: 10000},
				{Date: "2026-01-06", VoucherNum: 2, DebitCents: 20000, CreditCents: 0},
				{Date: "2026-01-06", VoucherNum: 2, DebitCents: 0, CreditCents: 20000},
			},
			wantErr: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			warnings, err := ValidateVoucherBalance(c.entries)
			if c.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if c.errContains != "" && !strings.Contains(err.Error(), c.errContains) {
					t.Errorf("error = %q, want contains %q", err.Error(), c.errContains)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(warnings) < c.minWarnings {
				t.Errorf("warnings = %d, want >= %d", len(warnings), c.minWarnings)
			}
		})
	}
}

func TestValidateVoucherBalanceRedEntryUnmodified(t *testing.T) {
	entries := []Entry{
		{Date: "2026-01-05", VoucherNum: 3, DebitCents: 50000, CreditCents: 0},
		{Date: "2026-01-05", VoucherNum: 3, DebitCents: 0, CreditCents: -50000},
	}
	if _, err := ValidateVoucherBalance(entries); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 红字必须保持原值（不折入对侧）
	if entries[1].CreditCents != -50000 || entries[1].DebitCents != 0 {
		t.Errorf("红字被修改: entry[1] = %+v, want CreditCents=-50000", entries[1])
	}
}
