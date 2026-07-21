## Why

当前总分类账写入处于「半集成」危险状态：标题区已使用 Layout 坐标，但数据写入和读取仍是硬编码 col 1-7（A~G）。标题与数据列不齐，读取辅助函数通过 GetRows 固定 index 访问数据。任何一方修改都会导致数据错乱。需要将整个系统统一到 Layout 坐标之下。

## What Changes

- 写入端：`cellName(N, row)` → `cellName(lay.FrontStartCol + N-1, row)`
- 读取端：`rows[i][index]` → `rows[i][lay.BindingLeftCols + index]`
- 打印标记列：`cellName(8, row)` → `cellName(lay.FrontStartCol + len(lay.ExcelColumns), row)`
- 受影响：gl_sheet.go, workbook.go, monthly_close.go, merge_gl_sheet.go, ml_sheet.go, print_mark.go
- 新增"写入→读回"测试用例保护列映射正确性

## Capabilities

### New Capabilities
- `gl-column-layout`: GL/MergeGL/ML 数据列与 Layout 坐标系统一映射，所有写入和读取通过 Layout 坐标计算

### Modified Capabilities
- `excel-generation`: 总分类账、合并总分类账、多科目明细账的数据写入方式从硬编码列号改为 Layout 坐标

## Impact

- generator/gl_sheet.go: 7 个写入函数 + 6 个读取辅助函数
- generator/workbook.go: ExtractLastMonthFinals 1 处
- generator/monthly_close.go: WriteMonthClosings 全量写入 + nextDataRowAfterBreak
- generator/merge_gl_sheet.go: appendToMergeGLSheet + 月结写入
- generator/ml_sheet.go: appendToMLSheet + 过次页/承前页写入 + lastBreakDetailTotals
- generator/print_mark.go: 打印标记列号
- generator/merge_gl_sheet_test.go: 行号断言偏移
- generator/generator_test.go: 新增写入→读回测试
- year_close.go: 已知遗留债务，change 4 清理
