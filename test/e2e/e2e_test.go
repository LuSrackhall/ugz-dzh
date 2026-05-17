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

	bin := filepath.Join(t.TempDir(), "ledger")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("编译失败: %s", out)
	}

	testData := filepath.Join(root, "test", "e2e", "test_data")
	output := filepath.Join(root, "test", "e2e", "output")

	os.RemoveAll(output)

	run := func(args ...string) (string, error) {
		c := exec.Command(bin, args...)
		out, err := c.CombinedOutput()
		return string(out), err
	}

	// Init first — required for generate to find JSON config
	out, err := run("init", "-s", "2026-01", "-o", output)
	if err != nil {
		t.Fatalf("init 失败: %v\n%s", err, out)
	}

	tests := []struct {
		name       string
		args       []string
		wantErr    bool
		wantMinLen int
		checkCSV   func(t *testing.T, records [][]string)
	}{
		{
			name:       "parse 2026-01 test_data with output flag",
			args:       []string{"generate", "-v", filepath.Join(testData, "2026_01"), "-o", output},
			wantMinLen: 50,
			checkCSV: func(t *testing.T, records [][]string) {
				header := records[0]
				wantHeader := []string{"日期", "凭证号", "摘要", "总账科目", "明细科目", "借方金额", "贷方金额"}
				for i, h := range wantHeader {
					if header[i] != h {
						t.Errorf("CSV 表头第 %d 列: 期望 %q, 实际 %q", i, h, header[i])
					}
				}

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

				for _, r := range records[1:] {
					debit, credit := r[5], r[6]
					if _, err := strconv.ParseFloat(debit, 64); err != nil {
						t.Errorf("无法解析借方金额 %q: %v", debit, err)
					}
					if _, err := strconv.ParseFloat(credit, 64); err != nil {
						t.Errorf("无法解析贷方金额 %q: %v", credit, err)
					}
				}

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
			name:    "reject mixed months in voucher dir",
			args:    []string{"generate", "-v", testData, "-o", output},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(bin, tt.args...)
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

			// 新路径: {output}/{year}/ledger.csv
			yearDir := filepath.Join(output, "2026")
			csvPath := filepath.Join(yearDir, "ledger.csv")
			f, err := os.Open(csvPath)
			if err != nil {
				t.Fatalf("无法打开输出 CSV %s: %v", csvPath, err)
			}
			defer f.Close()

			records, err := csv.NewReader(f).ReadAll()
			if err != nil {
				t.Fatalf("无法读取 CSV: %v", err)
			}

			dataRows := len(records) - 1
			if dataRows < tt.wantMinLen {
				t.Errorf("数据行数 %d < %d", dataRows, tt.wantMinLen)
			}

			if tt.checkCSV != nil {
				tt.checkCSV(t, records)
			}

			// 验证余额 CSV
			balPath := filepath.Join(yearDir, "balance.csv")
			bf, err := os.Open(balPath)
			if err != nil {
				t.Fatalf("无法打开余额 CSV %s: %v", balPath, err)
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

	t.Run("output goes to year subdirectory", func(t *testing.T) {
		os.RemoveAll(target)

		run := func(args ...string) (string, error) {
			c := exec.Command(bin, args...)
			out, err := c.CombinedOutput()
			return string(out), err
		}

		// Init required before generate
		out, err := run("init", "-s", "2026-01", "-o", target)
		if err != nil {
			t.Fatalf("init 失败: %v\n%s", err, out)
		}

		out, err = run("generate", "-v", filepath.Join(testData, "2026_01"), "-o", target)
		if err != nil {
			t.Fatalf("CLI 失败: %v\n%s", err, out)
		}

		// 新路径: {target}/{year}/
		yearDir := filepath.Join(target, "2026")
		csvPath := filepath.Join(yearDir, "ledger.csv")
		if _, err := os.Stat(csvPath); err != nil {
			t.Errorf("输出文件不存在: %s", csvPath)
		}

		balPath := filepath.Join(yearDir, "balance.csv")
		if _, err := os.Stat(balPath); err != nil {
			t.Errorf("余额输出文件不存在: %s", balPath)
		}

		xlsxPath := filepath.Join(yearDir, "2026-01.xlsx")
		if _, err := os.Stat(xlsxPath); err != nil {
			t.Errorf("月度 xlsx 不存在: %s", xlsxPath)
		}

		t.Logf("输出成功: %s", strings.TrimSpace(string(out)))
	})
}

// TestCLIFullWorkflow 完整多月份流程:
// init → 3x generate → check → add-manual → duplicate rejection → year-close
func TestCLIFullWorkflow(t *testing.T) {
	root, err := findProjectRoot()
	if err != nil {
		t.Fatalf("找不到项目根目录: %v", err)
	}

	testData := filepath.Join(root, "test", "e2e", "test_data")
	workDir := t.TempDir()
	outputDir := filepath.Join(workDir, "output")

	bin := filepath.Join(workDir, "ledger")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("编译失败: %s", out)
	}

	run := func(args ...string) (string, error) {
		c := exec.Command(bin, args...)
		out, err := c.CombinedOutput()
		return string(out), err
	}

	// 1. Init → {outputDir}/2026/2026.json
	out, err := run("init", "-s", "2026-01", "-o", outputDir)
	if err != nil {
		t.Fatalf("init 失败: %v\n%s", err, out)
	}
	configPath := filepath.Join(outputDir, "2026", "2026.json")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatal("init 后配置文件不存在: " + configPath)
	}

	// 2. Init overwrite protection
	_, err = run("init", "-s", "2026-01", "-o", outputDir)
	if err == nil {
		t.Error("第二次 init 应该失败（覆盖保护）")
	}

	yearDir := filepath.Join(outputDir, "2026")

	// 3. Generate 2026-01
	_, err = run("generate", "-v", filepath.Join(testData, "2026_01"), "-o", outputDir)
	if err != nil {
		t.Fatalf("generate 2026-01 失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(yearDir, "2026-01.xlsx")); err != nil {
		t.Error("2026-01.xlsx 未生成")
	}

	// 4. Generate 2026-02
	_, err = run("generate", "-v", filepath.Join(testData, "2026_02"), "-o", outputDir)
	if err != nil {
		t.Fatalf("generate 2026-02 失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(yearDir, "2026-02.xlsx")); err != nil {
		t.Error("2026-02.xlsx 未生成")
	}

	// 5. Generate 2026-03
	_, err = run("generate", "-v", filepath.Join(testData, "2026_03"), "-o", outputDir)
	if err != nil {
		t.Fatalf("generate 2026-03 失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(yearDir, "2026-03.xlsx")); err != nil {
		t.Error("2026-03.xlsx 未生成")
	}

	// 6. Check
	_, err = run("check", "-j", configPath)
	if err != nil {
		t.Errorf("check 失败: %v", err)
	}

	// 7. Add manual adjustment
	_, err = run("add-manual", "-a", "银行存款-工商银行", "-m", "2026-03", "-n", "100000.00", "-t", "补记上年余额", "-j", configPath)
	if err != nil {
		t.Errorf("add-manual 失败: %v", err)
	}

	// 8. Duplicate rejection
	_, err = run("add-manual", "-a", "银行存款-工商银行", "-m", "2026-03", "-n", "100000.00", "-t", "补记上年余额", "-j", configPath)
	if err == nil {
		t.Error("重复 add-manual 应该失败")
	}

	// 9. Year-close (uses outputDir as root for year subdirectories)
	_, err = run("year-close", "-j", configPath, "-o", outputDir)
	if err != nil {
		t.Fatalf("year-close 失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "2027", "2027-01.xlsx")); err != nil {
		t.Error("year-close 未生成 2027-01.xlsx")
	}

	// 10. All output files present (in year dir)
	for _, f := range []string{"ledger.csv", "balance.csv", "ledger.xlsx", "balance.xlsx"} {
		if _, err := os.Stat(filepath.Join(yearDir, f)); err != nil {
			t.Errorf("输出文件缺失: %s", f)
		}
	}

	t.Logf("完整工作流测试通过")
}
