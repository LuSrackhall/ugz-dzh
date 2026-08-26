## Why

第三轮专家审查（agent-6ad11b15 / agent-612211a3）+ 复核实测确认：

1. **D1（必修，已实测复现）**：科目**既直接记账（无明细）又配置在合并总账科目**时——① 同一 sheet 写两组月结行（实测 6 行：5000 与 8000）；② `ExtractLastMonthFinals` 读到合并期末污染下月期初（实测 2026-02 期初=8000，应为 5000）；③ 合并期初叠加父级自身 initials 虚增。
2. **F1（建议）**：`year-close` 不做上年末平衡校验、最近月非 12 月可静默结转、损益类科目漏结转无告警。
3. **N1（建议）**：同一行借贷双填不校验（本月合计发生额虚增）。
4. **N2（建议）**：全角括号红字 `（500）` 未识别（H2 只覆盖半角 `(500)`）。

## What Changes

1. **D1a 禁止合并父级直接分录**：`GenerateWorkbook` 开头校验——`合并总账科目` 中的科目出现**无明细直接分录** → 报错"请使用子科目记账"（CLI 内建安全）。
2. **D1b 期初不被合并视图污染**：`ExtractLastMonthFinals` 跳过 `合并总账科目` 对应的 sheet（合并视图非账页，不进 prevFinals）。
3. **D1c 合并期初只聚叶子**：`WriteMergeGLClosings` 的 `parentInitial` 去掉 `+= initials[general]`，仅由子科目期初聚合。
4. **F1 跨年结转校验**：`year-close` 增加——① 最近月非 12 月告警；② 最近月期初试算平衡校验（不平报错）；③ 收入/费用类科目最近月末余额非 0 告警（疑似漏结转）。
5. **N1 单行双填提示**：`ValidateVoucherBalance` 对同行借贷双非零输出 warning。
6. **N2 全角括号红字**：`parseAmountToCents` 支持全角括号 `（500）` → 负数。

## Capabilities

### New Capabilities
- `merge-conflict-guard`: 合并总账与直接记账冲突防护（禁止+期初隔离+聚合修正）、跨年结转校验、双填/全角红字输入防线

## Impact

- `generator/generate.go`（D1a）、`generator/workbook.go`（D1b）、`generator/merge_gl_sheet.go`（D1c）
- `cmd/year_close.go`（F1）
- `voucher/balance_check.go`（N1）、`voucher/parser.go`（N2）
- 测试：D1 实测回归、F1 校验、N1/N2 单测
