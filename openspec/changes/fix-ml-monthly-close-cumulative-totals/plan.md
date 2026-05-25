# 修复多科目明细账月结行 A-G 列累计值聚合 — Implementation Plan

> **For agentic workers:** Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复 `WriteMLMonthClosings` 中"本季合计"和"本年累计"A-G 列（总借/贷合计）用错误的 key（总账名）查找全路径 map，导致累计值丢失前几个月数据。

**Architecture:** 仅改 `generator/monthly_close_ml.go` 一个文件，将两处 `qtdDebit[general]` / `ytdDebit[general]` 单 key 查找替换为遍历 `details` 列表用 `general + "-" + detail` 全路径聚合。`details` 已在函数开头通过 `readMLDetailHeaders` 获取（第 42 行），直接复用。

**Tech Stack:** Go 1.21+, excelize/v2

---

### Task 1: 修复本季合计和本年累计的 A-G 列累计值聚合

**Files:**
- Modify: `generator/monthly_close_ml.go:103-104` (本季合计)
- Modify: `generator/monthly_close_ml.go:139-140` (本年累计)

- [ ] **Step 1: 修改"本季合计"A-G 列聚合逻辑**

将第 103-104 行的单 key 查找替换为遍历 `details` 聚合：

```go
// 替换前 (lines 103-104):
				qtDebit += qtdDebit[general]
				qtCredit += qtdCredit[general]

// 替换为:
				for _, d := range details {
					if d != "" {
						key := general + "-" + d
						qtDebit += qtdDebit[key]
						qtCredit += qtdCredit[key]
					}
				}
```

- [ ] **Step 2: 修改"本年累计"A-G 列聚合逻辑**

将第 139-140 行的单 key 查找替换为遍历 `details` 聚合：

```go
// 替换前 (lines 139-140):
			cumDebit += ytdDebit[general]
			cumCredit += ytdCredit[general]

// 替换为:
			for _, d := range details {
				if d != "" {
					key := general + "-" + d
					cumDebit += ytdDebit[key]
					cumCredit += ytdCredit[key]
				}
			}
```

- [ ] **Step 3: 运行 go vet 静态分析**

```bash
cd /Users/srackhalllu/Desktop/资源管理器/safe/ugz-dzh && go vet ./...
```

Expected: 无错误输出，退出码 0

- [ ] **Step 4: 运行现有单元测试**

```bash
cd /Users/srackhalllu/Desktop/资源管理器/safe/ugz-dzh && go test ./generator/... -v
```

Expected: 所有测试 PASS

- [ ] **Step 5: 运行全量测试确保无回归**

```bash
cd /Users/srackhalllu/Desktop/资源管理器/safe/ugz-dzh && go test ./... -v
```

Expected: 所有测试 PASS

- [ ] **Step 6: 提交修改**

```bash
cd /Users/srackhalllu/Desktop/资源管理器/safe/ugz-dzh
git add generator/monthly_close_ml.go
git commit -m "$(cat <<'EOF'
fix: 多科目明细账月结行 A-G 列累计值用全路径聚合

本季合计和本年累计的 A-G 列原本用仅有总账名的 key 去查以
全路径为 key 的累计 map，导致查找始终返回 0，累计值只含当月
发生额。改为遍历该 general 下所有明细科目，用全路径聚合。
EOF
)"
```

---

### Task 2: 端到端手动验证

- [ ] **Step 1: 在已有数据上重新生成季末月份，验证本季合计**

```bash
cd /Users/srackhalllu/Desktop/资源管理器/safe/ugz-dzh
# 假设已有数月数据，对季末月份（如 03）执行 -f 重建
go run . generate -v <voucherDir> -o <outputDir> -f
```

打开生成的 xlsx，找到多科目明细账 Sheet，检查"本季合计"行的 A-G 列（D 列借方、E 列贷方）是否聚合了该季度所有月份的数据，而非仅当月。

- [ ] **Step 2: 验证本年累计**

在同一 xlsx 中，检查"本年累计"行的 A-G 列（D 列借方、E 列贷方）是否聚合了从 1 月到当月的所有月份数据，而非仅当月。

---

## Self-Review

### Spec Coverage
- MODIFIED `月结处理` Requirement → Task 1 steps 1-2 实现聚合逻辑修正
- Scenario "多科目明细账本季合计 A-G 列聚合所有明细" → Task 2 step 1 手动验证
- Scenario "多科目明细账本年累计 A-G 列聚合所有明细" → Task 2 step 2 手动验证

### Placeholder Scan
无 TBD/TODO/占位符。所有步骤有具体代码、命令和预期结果。

### Type Consistency
复用的 `details` 变量已在函数第 42 行通过 `readMLDetailHeaders` 获取，类型为 `[]string`，空字符串代表空列。Task 1 中的 `d` 和 `key` 均为新局部变量，无命名冲突。
