## Context

项目使用 `excelize` 库生成 xlsx 账薄。内部金额统一以 `int64`（分）存储和传递。写入 Excel 单元格时，通过 `centsToYuanStr(c int64) string` 将分转换为元字符串，再通过 `SetCellValue` 写入单元格。

`excelize.SetCellValue` 对 `string` 类型写入文本单元格，对 `float64` 类型写入数字单元格。当前代码全域使用 `centsToYuanStr`，导致所有金额在 Excel 中均为文本格式，用户无法直接使用 SUM 求和、按数值筛选、创建图表等。

## Goals / Non-Goals

**Goals:**
- 所有通过 `centsToYuanStr` 写入 xlsx 的金额单元格均变为数字类型
- 改动集中在函数定义层，调用点无感知
- 不影响现有测试（更新断言后）和生成流程

**Non-Goals:**
- 不改变内部金额存储类型（保持 `int64` 分）
- 不改变 CSV 解析逻辑
- 不改变配置文件中余额的 JSON 结构
- 不添加 Excel 单元格数字格式样式（如千分位分隔符）——那是后续可选优化

## Decisions

### D1：改造 `centsToYuanStr` → `centsToYuan`，返回 `float64`

- **选择**：将函数签名从 `func centsToYuanStr(c int64) string` 改为 `func centsToYuan(c int64) float64`
- **实现**：`float64(c) / 100` 替代手写的 `sign + fmt.Sprintf("%d.%02d", ...)`
- **理由**：改动 1 处定义 + 1 处测试，覆盖全部 33 个调用点。`float64(c)/100` 与 `int64/100` 语义一致，`SetCellValue` 原生支持 `float64` 并写入为数字
- **替代方案已排除**：逐点内联（改动面大）、新旧并存（维护负担）

### D2：保留函数而非内联

- **选择**：保留 `centsToYuan` 函数，不在每个调用处内联 `float64(x)/100`
- **理由**：函数名表达"分→元"的语义意图，内联代码 `float64(x)/100` 失去了这个语义提示

### D3：不添加数字格式样式

- **选择**：仅改变写入类型（string→float64），不通过 `SetCellStyle` 或 `CustomNumFmt` 添加千分位、货币符号等格式
- **理由**：范围外优化。当前目标是让 Excel 识别金额为数字。添加格式样式可以后续单独处理

## Risks / Trade-offs

- **[浮点精度]**：`float64(c)/100` 对于整数分（int64）转换为元，精度完全足够（Go 的 float64 可精确表示 2^53 以内的整数，远大于任何合理金额）。对于 6 位整数分（即万元级别），float64 仍有 >10 位有效数字，不会出现舍入误差 → 实际无风险

- **[零值显示差异]**：原来 `centsToYuanStr(0)` 返回 `"0.00"`（字符串），改为 `float64(0)/100 = 0`（数字）。Excel 对数字 0 默认显示为 `0` 而非 `0.00`。这是 Excel 数字格式行为，不是代码 bug → 可接受，后续可通过数字格式样式统一（非本次范围）

- **[调用点重命名遗漏]**：函数名从 `centsToYuanStr` 改为 `centsToYuan`，如果遗漏调用点会导致编译错误 → 编译时即可发现，零遗漏风险
