## Why

会计专家 + 审计专家二审（agent-6ad11b15 / agent-612211a3）发现 3 个高危数据正确性问题，均经复核实锤（H1/H2 实测复现、H3 代码确认）：

1. **H1 期初回退翻旧账**：`GetInitBalanceForGenerate` 回退条件 `mb.Final != 0` 跳过"期末=0"的最近月 → 年末结平科目跨年首月凭空复活更早月余额（实测：11月末 500 / 12月末 0 → 2026-01 期初错误为 500）。
2. **H2 红字解析失真**：`parseAmountToCents` 白名单 `[^0-9.\-]` 剥掉括号 `(500)`、全角减号 `－`、Unicode 减号 `−` 的符号 → 红字被当正数（实测三种格式均解析为 +500）。配套：打印版 `splitCNY` 对负数取绝对值、无红色标记 → 红字与蓝字打印无差别。
3. **H3 合并总账累计漏计**：`WriteMergeGLClosings` 本年/本季累计只遍历 `activity`（当月有分录科目）→ 当月无分录但有历史累计的子科目被漏 → 父级累计 < 叶子之和。

## What Changes

1. **H1**：`GetInitBalanceForGenerate` 回退取**最近月份期末（含 0）**，去掉 `Final != 0` 条件。
2. **H2a 解析**：`parseAmountToCents` 支持四种红字格式（ASCII `-500`、括号 `(500)`、全角 `－500`、Unicode `−500`），统一转负数；括号形式剥括号后若无负号则取反。
3. **H2b 打印版红字标记**：金额格写入时若为负数（红字），数字用**红色字体**（#CC0000，与系统红线一致）标记；拆位仍按绝对值。查看版保持负数列示（数字格式自带 `-`）。
4. **H3**：`WriteMergeGLClosings` 本年/本季累计遍历 **Tree 全部叶子**（`isChildOf`），不再限于 activity。

## Capabilities

### New Capabilities
- `red-ink-handling`: 红字全链路——解析格式归一、打印版红色字体标记、负数数据一致性

### Modified Capabilities
- `initial-balance`: 期初回退取最近期末（含 0），结平科目不翻旧账
- `excel-generation`: 合并总账本年/本季累计口径修正（全叶子）；打印版红字红色字体

## Impact

- `balance/balance.go`：GetInitBalanceForGenerate 回退条件
- `voucher/parser.go`：parseAmountToCents 红字格式
- `generator/print_common.go`：amountSubStyle 加 red 参数、缓存 key 含 red、金额格红字标记
- `generator/merge_gl_sheet.go`：WriteMergeGLClosings 累计遍历全叶子
- 测试：H1 结平回退、H2 解析四格式、H3 合并累计（单测 + e2e）
- 不变：查看版负数显示（已正确）、平衡校验净额口径（已正确）、其余逻辑
