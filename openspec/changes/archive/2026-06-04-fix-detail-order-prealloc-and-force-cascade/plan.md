# 修复 detailOrder 预分配列与 -f 级联重建 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复 detailOrder 新 Sheet 初始化时未完整展开预分配列，以及 -f 未级联删除后续月份 xlsx 两个缺陷。

**Architecture:** 两处独立修改 — `ensureMLSheet` 新 Sheet 路径直接使用 detailOrder 完整列表作为 initDetails，当月分录中不在配置内的科目按字母序追加到右侧空列；`cmd/generate.go` -f 分支删除 yearDir 下当月及之后所有 .xlsx 文件。

**Tech Stack:** Go 1.21+, excelize/v2

---

### Task 1: 修复 detailOrder 预分配列完整展开

**Files:**
- Modify: `generator/ml_sheet.go:75-101`

- [ ] **Step 1: 修改 ensureMLSheet 新 Sheet 初始化逻辑**

将第 75-101 行的 `detailOrder` 分支改为直接使用完整 `detailOrder` 列表作为 `initDetails`，剩余科目按字母序追加到右侧空列。

当前代码（第 75-95 行）：
```go
	if len(detailOrder) > 0 {
		initDetails = make([]string, 0, mlMaxDetails)
		existingSet := make(map[string]bool)
		for _, d := range details {
			existingSet[d] = true
		}
		for _, d := range detailOrder {
			if d == "" || existingSet[d] {
				initDetails = append(initDetails, d)
				existingSet[d] = false
			}
		}
		var remaining []string
		for _, d := range details {
			if existingSet[d] {
				remaining = append(remaining, d)
			}
		}
		sort.Strings(remaining)
		initDetails = append(initDetails, remaining...)
		newAppended = remaining
```

替换为：
```go
	if len(detailOrder) > 0 {
		// 直接复制 detailOrder 完整列表（含 "" 跳列和未发生科目）
		initDetails = make([]string, len(detailOrder))
		copy(initDetails, detailOrder)

		// 当月分录中不在 detailOrder 中的科目 → 追加到右侧空列
		inOrder := make(map[string]bool)
		for _, d := range detailOrder {
			if d != "" {
				inOrder[d] = true
			}
		}
		var remaining []string
		for _, d := range details {
			if !inOrder[d] {
				remaining = append(remaining, d)
			}
		}
		sort.Strings(remaining)
		initDetails = append(initDetails, remaining...)
		newAppended = remaining
```

- [ ] **Step 2: 运行单元测试验证**

```bash
go test ./generator/... -v
```

预期：所有测试通过。

- [ ] **Step 3: 运行 go vet**

```bash
go vet ./...
```

预期：无错误输出。

- [ ] **Step 4: 提交**

```bash
git add generator/ml_sheet.go
git commit -m "$(cat <<'EOF'
fix: detailOrder 新 Sheet 初始化时完整展开所有预分配列

改为直接使用 detailOrder 完整列表（含空字符串跳列和未发生科目），当月分录中不在配置内的科目按字母序追加到右侧空列。
实现 detailOrder 与科目树完全解耦的原始设计意图。
EOF
)"
```

---

### Task 2: 修复 -f 级联删除后续月份 xlsx

**Files:**
- Modify: `cmd/generate.go:104-109`

- [ ] **Step 1: 修改 -f 分支实现级联删除**

当前代码（第 104-109 行）：
```go
		xlsxPath := filepath.Join(yearDir, month+".xlsx")
		if !force {
			if _, err := os.Stat(xlsxPath); err == nil {
				return fmt.Errorf("%s 已存在，使用 -f 覆盖已有 xlsx", xlsxPath)
			}
		}
```

替换为：
```go
		xlsxPath := filepath.Join(yearDir, month+".xlsx")
		if force {
			// 级联删除当月及之后所有月份的 xlsx
			entries, err := os.ReadDir(yearDir)
			if err == nil {
				for _, entry := range entries {
					if entry.IsDir() {
						continue
					}
					name := entry.Name()
					if strings.HasSuffix(name, ".xlsx") && strings.TrimSuffix(name, ".xlsx") >= month {
						path := filepath.Join(yearDir, name)
						if err := os.Remove(path); err != nil {
							return fmt.Errorf("删除 %s: %w", path, err)
						}
						if verbose {
							fmt.Printf("已删除: %s\n", path)
						}
					}
				}
			}
		} else {
			if _, err := os.Stat(xlsxPath); err == nil {
				return fmt.Errorf("%s 已存在，使用 -f 覆盖已有 xlsx", xlsxPath)
			}
		}
```

- [ ] **Step 2: 运行全量测试**

```bash
go test ./... -v
```

预期：所有测试通过（包括 e2e 测试）。

- [ ] **Step 3: 运行 go vet**

```bash
go vet ./...
```

预期：无错误输出。

- [ ] **Step 4: 提交**

```bash
git add cmd/generate.go
git commit -m "$(cat <<'EOF'
fix: -f 级联删除当月及后续所有月份 xlsx

-f 语义为"从此月起强制重建"，后续月份 xlsx 从旧布局继承，
必须随当月一起重建以避免标题与 detailOrder 冲突。
仅删 xlsx（可再生），JSON 余额和配置不受影响。
EOF
)"
```

---

### Task 3: 端到端手动验证

- [ ] **Step 1: 测试预分配列完整展开**

```bash
# 编译
go build -o /tmp/ledger-e2e .

# 首月生成（获取初始 detailOrder）
rm -rf /tmp/ledger-e2e-test
/tmp/ledger-e2e init -s 2026-01 -o /tmp/ledger-e2e-test
/tmp/ledger-e2e generate -v test/e2e/test_data/2026_01 -o /tmp/ledger-e2e-test

# 编辑 JSON：在某个科目的 detailOrder 中插入空字符串跳列和预分配科目
python3 << 'PYEOF'
import json
with open('/tmp/ledger-e2e-test/2026/2026.json') as f:
    d = json.load(f)
do = d.get('明细列顺序', {})
# 在管理费用的列序中加入跳列和预分配科目
do['管理费用'] = ['办公费', '', '预留科目', '干部报酬', '通讯费', '交通费', '广宣费', '水电费', '修缮费', '其他']
d['明细列顺序'] = do
with open('/tmp/ledger-e2e-test/2026/2026.json', 'w') as f:
    json.dump(d, f, ensure_ascii=False, indent=2)
print('Edited:', do['管理费用'])
PYEOF

# -f 从首月重建
/tmp/ledger-e2e generate -v test/e2e/test_data/2026_01 -o /tmp/ledger-e2e-test -f

# 检查标题行
python3 << 'PYEOF'
from openpyxl import load_workbook
wb = load_workbook('/tmp/ledger-e2e-test/2026/2026-01.xlsx')
ws = wb['多科目明细账-管理费用']
# 读第2行 H-U 标题
headers = [ws.cell(row=2, column=c).value for c in range(8, 22)]
print('H-U headers:', headers)
expected = ['办公费', None, '预留科目', '干部报酬', '通讯费', '交通费', '广宣费', '水电费', '修缮费', '其他', None, None, None, None]
for i, (h, e) in enumerate(zip(headers, expected)):
    status = 'OK' if h == e else f'FAIL (got={h!r}, want={e!r})'
    col_letter = chr(ord('H') + i)
    print(f'  {col_letter}: {status}')
PYEOF
```

预期：H="办公费", I=空(跳列), J="预留科目", K="干部报酬", ...

- [ ] **Step 2: 测试 -f 级联删除**

```bash
# 先生成多个月份
/tmp/ledger-e2e generate -v test/e2e/test_data/2026_02 -o /tmp/ledger-e2e-test
/tmp/ledger-e2e generate -v test/e2e/test_data/2026_03 -o /tmp/ledger-e2e-test
ls /tmp/ledger-e2e-test/2026/*.xlsx

# -f 重新生成 02 月
/tmp/ledger-e2e generate -v test/e2e/test_data/2026_02 -o /tmp/ledger-e2e-test -f

# 检查：03 月 xlsx 应该被级联删除
ls /tmp/ledger-e2e-test/2026/*.xlsx
```

预期：`-f 02` 后，03.xlsx 被删除，01.xlsx 保留。

- [ ] **Step 3: 运行全量测试最终确认**

```bash
go test ./... -v
```

预期：全量通过。
