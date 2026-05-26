## Design Summary

所有通过 `SetCellValue` 写入 xlsx 的金额当前使用 `centsToYuanStr()` 函数，该函数返回 `string` 类型，导致 Excel 将金额单元格识别为文本而非数字。用户无法在 Excel 中直接使用 SUM 求和、按数值筛选或创建图表。

根因：`centsToYuanStr(c int64) string` 返回字符串，`excelize.SetCellValue` 对 string 类型写入文本单元格。

改进方案：将 `centsToYuanStr` 改为 `centsToYuan`，返回 `float64` 类型，使 `SetCellValue` 写入数字单元格。所有调用点无需修改（Go 的 `float64` 满足 `interface{}` 参数，`SetCellValue` 原生支持 float64 并写入为数字）。

## Alternatives Considered

### 方案 A：改造 `centsToYuanStr` → 返回 `float64`

- **做法**：将 `centsToYuanStr(c int64) string` 改为 `centsToYuan(c int64) float64`，实现从 `fmt.Sprintf` 变为 `float64(c) / 100`。函数名去掉 `Str` 后缀以反映返回类型变化。所有 33 处调用点代码无需修改。
- **优点**：改动最小（1 个函数签名 + 实现），效果覆盖全部调用点，零遗漏风险
- **缺点**：`float64(c)/100` 可能丢失手写格式 `fmt.Sprintf("%d.%02d", c/100, c%100)` 在某些边界情况下的精确控制（如负数显示），但当前代码的 sign 处理逻辑仅用于 workaround excelize 旧版负数显示问题
- **为何采用**：改动面最小、风险最低、效果全面

### 方案 B：删除函数，每个调用处内联 `float64(x)/100`

- **做法**：删除 `centsToYuanStr`，在 33 处调用点逐一替换为 `float64(value)/100`
- **优点**：减少一层间接调用
- **缺点**：33 处都要改，容易遗漏；改动面大，回滚麻烦
- **为何未采用**：改动面远大于方案 A，收益不足以抵消风险

### 方案 C：新旧函数并存，逐步迁移

- **做法**：新增 `centsToYuan(c int64) float64`，保留旧函数，逐个调用点迁移
- **优点**：无
- **缺点**：增加过渡期维护负担，两套 API 共存造成困惑
- **为何未采用**：纯增负担，无实际收益

## Agreed Approach

采用方案 A：将 `centsToYuanStr` 改为 `centsToYuan`，返回 `float64` 而非 `string`。

1. 修改 `generator/workbook.go` 中的函数定义：签名从 `func centsToYuanStr(c int64) string` 改为 `func centsToYuan(c int64) float64`，实现从字符串拼接改为 `float64(c) / 100`
2. 所有 33 处调用点自动适应（函数名变化需同步重命名，但参数和赋值目标不变）
3. 更新测试 `generator/generator_test.go` 中的调用和断言类型

## Key Decisions

- **范围**：所有金额列均改为数字，包括总分类账、多科目明细账、期初表、期末表的借贷金额、余额、明细科目金额
- **统一转换**：使用 `float64(c)/100` 作为唯一的分→元转换方式，不再用手写 `fmt.Sprintf` 拼接
- **零值行为**：`float64(0)/100 = 0.00`，写入 Excel 后显示为 `0`（数字格式），与原来 `"0.00"`（文本）不同，但语义正确

## Open Questions

- 生成 4 个月数据后，需要用验证脚本确认所有金额列在 Excel 中为数字格式（非文本），可在实现后通过 `go run scripts/verify_ml_closings.go` 和手动抽查确认
