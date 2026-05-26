# Retrospective: fix-xlsx-amount-numeric-type

> Written: 2026-05-26 (after verify passed)
> Commit range: `72aca12..bf8b6fd`
> Worktree: `/Users/srackhalllu/Desktop/资源管理器/safe/ugz-dzh/.claude/worktrees/fix-xlsx-amount-numeric-type`

---

## 0. Evidence

- **Commit range**: `72aca12..bf8b6fd` (1 commit)
- **Diff size**: +65 / -73 lines across 9 files
- **Tasks done**: 7/7
- **Active hours**: ~1.5
- **Subagent dispatches**: 1 (subagent-driven-development orchestrator)
- **New external dependencies**: none
- **Bugs encountered post-merge**: none
- **OpenSpec validate state at archive**: pass (7/7 items valid)
- **Test coverage signal**: `go test ./...` all pass; `TestCentsToYuan` (6 cases); `TestWriteMLMonthClosings_CumulativeAggregation` (4 assertions); e2e verify_ml_closings.go 5/5 PASS

Commit chain (时序):

```
72aca12 fix: 年/季累计提取覆盖所有历史科目 — 防止无当月分录的明细科目被遗漏
bf8b6fd fix: xlsx 金额单元格从文本改为数字类型 — centsToYuan 返回 float64
```

---

## 1. Wins

- [evidence: bf8b6fd] 一处函数定义修改覆盖全部 33 个调用点，改动面极小（+65/-73），零遗漏风险
- [evidence: verify.md §2.4] xlsx XML 抽查确认 D/E/G 列金额单元格 0 个 `t="inlineStr"`，全部为数字类型
- [evidence: verify.md §2.3] e2e 管线 4 个月生成 + verify_ml_closings.go 全部 PASS，数值无回归
- [evidence: plan.md vs actual] plan 预估的步骤与实际执行高度吻合，无返工

## 2. Misses

- 📌 [nit | evidence: monthly_close_ml_test.go:130-144] `GetRows` 读取 float64 单元格时不保留尾随零（`"4000"` 而非 `"4000.00"`），测试断言需同步更新。这是 excelize 行为差异，非代码缺陷，但 plan 未预见到此点

## 3. Plan deviations

| Plan task | What changed | Why |
|-----------|--------------|-----|
| 2.4 (xlsx 类型验证) | plan 中给出了 `/tmp/check_numeric.go` 脚本框架但标注"仅做样例框架"；实际改用 `unzip -p` + `grep` 直接检查 xml 中 `t="inlineStr"` 属性 | xml 直接检查比 excelize API 更可靠（excelize 无 `GetCellType` API），且 plan 本身已指出此方向 |

## 4. Skill / workflow compliance

| Skill                                            | Used |
|--------------------------------------------------|------|
| superpowers:brainstorming                        |  ✓   |
| superpowers:writing-plans                        |  ✓   |
| superpowers:using-git-worktrees                  |  ✓   |
| superpowers:subagent-driven-development          |  ✓   |
| (transitive) superpowers:test-driven-development |  ✓   |
| (transitive) superpowers:requesting-code-review  |  ✓   |
| superpowers:finishing-a-development-branch       |  ⬜  |

> `finishing-a-development-branch` 将在 archive 后执行。

### Deliberately Skipped Skills

（无）

## 5. Surprises

- `excelize.GetRows` 对 float64 写入的单元格返回 `"4000"` 而非 `"4000.00"`（无尾随零）。测试断言从 `"4000.00"` 更新为 `"4000"`，但这是读取侧的表现差异，不影响写入类型正确性

## 6. Promote candidates → long-term learning

- [ ] 📌 **xlsx 数值单元格通过 GetRows 回读时不保留 .00 尾随零** → **Promote to memory** (type: feedback)
  > **Why**: `centsToYuan(400000)` → `SetCellValue(4000)` → excelize 内部写为 `<v>4000</v>`。`GetRows` 回读时返回 `"4000"` 而非 `"4000.00"`。如果测试用 `GetRows` 验证金额列，断言必须匹配无尾随零的格式。
  > **How to apply**: 在 generator 测试中通过 `GetRows` 验证金额列时，预期值去掉 `.00` 后缀；或改用 xml 级别的断言（`unzip -p | grep`）绕过 excelize 的字符串化行为。

- [ ] 📌 **float64(cents)/100 对会计金额无精度风险** → **Promote to CLAUDE.md** (`CODEBUDDY.md` 已记录此结论)
  > **Why**: Go float64 可精确表示 2^53 以内的整数。会计金额以分为单位（int64），即使亿元级别（10^10 分）仍在安全范围内。`float64(c)/100` 不会引入舍入误差。
  > **How to apply**: CODEBUDDY.md 已在 "Amount Representation" 节记录此结论，无需额外操作。
