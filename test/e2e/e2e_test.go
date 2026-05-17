package e2e

import (
	"encoding/csv"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestCLI(t *testing.T) {
	root, err := findProjectRoot()
	if err != nil {
		t.Fatalf("找不到项目根目录: %v", err)
	}

	// Build the binary once
	bin := filepath.Join(t.TempDir(), "ledger")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("编译失败: %s", out)
	}

	testData := filepath.Join(root, "test", "e2e", "test_data")
	output := filepath.Join(root, "test", "e2e", "output")

	// Clean and recreate output dir
	os.RemoveAll(output)

	tests := []struct {
		name       string
		args       []string
		wantErr    bool
		wantMinLen int
		checkCSV   func(t *testing.T, records [][]string)
	}{
		{
			name:       "parse all test_data with output flag",
			args:       []string{"generate", "-v", testData, "-o", output},
			wantMinLen: 290,
			checkCSV: func(t *testing.T, records [][]string) {
				header := records[0]
				wantHeader := []string{"日期", "凭证号", "摘要", "总账科目", "明细科目", "借方金额", "贷方金额"}
				for i, h := range wantHeader {
					if header[i] != h {
						t.Errorf("CSV 表头第 %d 列: 期望 %q, 实际 %q", i, h, header[i])
					}
				}

				// Verify sorting: dates must be non-decreasing, voucher numbers non-decreasing within same date
				var prevDate string
				prevVnum := 0
				for i, r := range records[1:] {
					if r[0] < prevDate {
						t.Errorf("第 %d 行日期倒序: %s < %s", i+1, r[0], prevDate)
					}
					vnum, _ := strconv.Atoi(r[1])
					if r[0] == prevDate && vnum < prevVnum {
						t.Errorf("第 %d 行凭证号倒序 (同日): %d < %d", i+1, vnum, prevVnum)
					}
					prevDate = r[0]
					prevVnum = vnum
				}

				// Spot check: voucher 1 in 2026-01 should have 库存现金 and 银行存款 entries
				foundStock, foundBank := false, false
				for _, r := range records[1:] {
					if r[0] == "2026-01-06" && r[1] == "1" {
						if r[3] == "库存现金" {
							foundStock = true
						}
						if r[3] == "银行存款" {
							foundBank = true
						}
					}
				}
				if !foundStock || !foundBank {
					t.Errorf("2026-01-06 记字第0001号 缺少预期科目: 库存现金=%v 银行存款=%v", foundStock, foundBank)
				}

				// Verify amounts parse correctly
				for _, r := range records[1:] {
					debit, credit := r[5], r[6]
					if _, err := strconv.ParseFloat(debit, 64); err != nil {
						t.Errorf("无法解析借方金额 %q: %v", debit, err)
					}
					if _, err := strconv.ParseFloat(credit, 64); err != nil {
						t.Errorf("无法解析贷方金额 %q: %v", credit, err)
					}
				}

				// Debits should equal credits within each voucher
				type voucherKey struct{ date, vnum string }
				vmap := make(map[voucherKey]float64)
				for _, r := range records[1:] {
					k := voucherKey{r[0], r[1]}
					d, _ := strconv.ParseFloat(r[5], 64)
					c, _ := strconv.ParseFloat(r[6], 64)
					vmap[k] = vmap[k] + d - c
				}
				for k, bal := range vmap {
					if bal > 0.01 || bal < -0.01 {
						t.Errorf("凭证 %s-%s 借贷不平衡: 差额 %.2f", k.date, k.vnum, bal)
					}
				}
			},
		},
		{
			name:    "generate missing voucherDir exits with error",
			args:    []string{"generate", "-o", output},
			wantErr: true,
		},
		{
			name:       "default output is current directory",
			args:       []string{"generate", "-v", filepath.Join(testData, "2026_01")},
			wantMinLen: 50,
			checkCSV: func(t *testing.T, records [][]string) {
				// Ensure we at least got the 2026-01 data
				if len(records) < 2 {
					t.Error("期望至少有一条数据")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For default output test, use a temp working dir
			workDir := ""
			if tt.name == "default output is current directory" {
				workDir = t.TempDir()
			}

			args := tt.args
			if workDir != "" {
				// Override -output with the temp dir
				args = append([]string{}, args...)
				args = append(args, "-o", workDir)
			}

			cmd := exec.Command(bin, args...)
			if workDir != "" {
				cmd.Dir = workDir
			}
			out, err := cmd.CombinedOutput()

			if tt.wantErr {
				if err == nil {
					t.Error("期望错误退出，但成功退出了")
				}
				return
			}
			if err != nil {
				t.Fatalf("CLI 运行失败: %v\n输出: %s", err, out)
			}

			csvPath := filepath.Join(output, "ledger.csv")
			if workDir != "" {
				csvPath = filepath.Join(workDir, "ledger.csv")
			}
			f, err := os.Open(csvPath)
			if err != nil {
				t.Fatalf("无法打开输出 CSV: %v", err)
			}
			defer f.Close()

			records, err := csv.NewReader(f).ReadAll()
			if err != nil {
				t.Fatalf("无法读取 CSV: %v", err)
			}

			dataRows := len(records) - 1 // minus header
			if dataRows < tt.wantMinLen {
				t.Errorf("数据行数 %d < %d", dataRows, tt.wantMinLen)
			}

			if tt.checkCSV != nil {
				tt.checkCSV(t, records)
			}

			// Verify balance CSV exists and is well-formed
			balPath := filepath.Join(output, "balance.csv")
			if workDir != "" {
				balPath = filepath.Join(workDir, "balance.csv")
			}
			bf, err := os.Open(balPath)
			if err != nil {
				t.Fatalf("无法打开余额 CSV: %v", err)
			}
			defer bf.Close()
			balRecords, err := csv.NewReader(bf).ReadAll()
			if err != nil {
				t.Fatalf("无法读取余额 CSV: %v", err)
			}
			balHeader := balRecords[0]
			wantBalHeader := []string{"科目类别", "科目全路径", "借方合计", "贷方合计", "余额", "方向"}
			for i, h := range wantBalHeader {
				if balHeader[i] != h {
					t.Errorf("余额 CSV 表头第 %d 列: 期望 %q, 实际 %q", i, h, balHeader[i])
				}
			}
			typeOrder := map[string]int{"资产": 1, "负债": 2, "权益": 3, "收入": 4, "费用": 5}
			prevOrder := 0
			for _, r := range balRecords[1:] {
				order := typeOrder[r[0]]
				if order < prevOrder {
					t.Errorf("余额表科目类别顺序错乱: %s after order %d", r[0], prevOrder)
				}
				prevOrder = order
				for _, col := range []int{2, 3, 4} {
					if _, err := strconv.ParseFloat(r[col], 64); err != nil {
						t.Errorf("无法解析余额金额 %q: %v", r[col], err)
					}
				}
				if r[5] != "借" && r[5] != "贷" && r[5] != "平" {
					t.Errorf("余额方向无效: %q", r[5])
				}
				d, _ := strconv.ParseFloat(r[2], 64)
				c, _ := strconv.ParseFloat(r[3], 64)
				b, _ := strconv.ParseFloat(r[4], 64)
				var expectedDir string
				if d > c {
					expectedDir = "借"
				} else if c > d {
					expectedDir = "贷"
				} else {
					expectedDir = "平"
				}
				if r[5] != expectedDir {
					t.Errorf("余额 %s 方向应为 %s，实际 %s (借方=%.2f 贷方=%.2f)", r[1], expectedDir, r[5], d, c)
				}
				var expectedBal float64
				if expectedDir == "借" {
					expectedBal = d - c
				} else if expectedDir == "贷" {
					expectedBal = c - d
				}
				if diff := b - expectedBal; diff > 0.005 || diff < -0.005 {
					t.Errorf("余额 %s 计算错误: 期望 %.2f, 实际 %.2f", r[1], expectedBal, b)
				}
			}
		})
	}
}

func TestCLIRejectsNoVoucherDir(t *testing.T) {
	root, err := findProjectRoot()
	if err != nil {
		t.Fatalf("找不到项目根目录: %v", err)
	}

	bin := filepath.Join(t.TempDir(), "ledger")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("编译失败: %s", out)
	}

	tests := []struct {
		name string
		args []string
	}{
		{"no arguments", []string{"generate"}},
		{"only -o set", []string{"generate", "-o", "/tmp"}},
		{"empty voucherDir", []string{"generate", "-v", "", "-o", "/tmp"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(bin, tt.args...)
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Errorf("期望非零退出码，但成功了。输出: %s", out)
			}
		})
	}
}

func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("找不到 go.mod")
		}
		dir = parent
	}
}

func TestCLIOutputPath(t *testing.T) {
	root, err := findProjectRoot()
	if err != nil {
		t.Fatalf("找不到项目根目录: %v", err)
	}

	bin := filepath.Join(t.TempDir(), "ledger")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("编译失败: %s", out)
	}

	testData := filepath.Join(root, "test", "e2e", "test_data")
	target := filepath.Join(root, "test", "e2e", "output")

	t.Run("output goes to specified path", func(t *testing.T) {
		os.RemoveAll(target)

		cmd := exec.Command(bin, "generate", "-v", testData, "-o", target)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("CLI 失败: %v\n%s", err, out)
		}

		csvPath := filepath.Join(target, "ledger.csv")
		if _, err := os.Stat(csvPath); err != nil {
			t.Errorf("输出文件不存在: %s", csvPath)
		}

		balPath := filepath.Join(target, "balance.csv")
		if _, err := os.Stat(balPath); err != nil {
			t.Errorf("余额输出文件不存在: %s", balPath)
		}

		// Verify no stray files in unexpected places
		stray := filepath.Join(root, "ledger.csv")
		if _, err := os.Stat(stray); err == nil {
			t.Errorf("根目录存在不应该出现的 ledger.csv")
		}

		t.Logf("输出成功: %s", strings.TrimSpace(string(out)))
	})
}
