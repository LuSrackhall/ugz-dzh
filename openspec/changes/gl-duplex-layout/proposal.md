## Why

当前总分类账仅使用 FrontStartCol 区域（左侧正面），BackStartCol 区域（右侧反面）预留了列宽但从未写入数据。需要启用反面区域，使偶数页数据写入 BackStartCol 区，实现正反面并排，适配双面打印。

## What Changes

- 偶数页（pageNum 偶数）数据写入 BackStartCol+offset 而非 FrontStartCol+offset
- 偶数页标题（writePageHeader）写入 BackStartCol 区
- 新增 `dataCol` 辅助函数根据 pageNum 奇偶决定写入列
- 影响：gl_sheet.go、merge_gl_sheet.go、monthly_close.go、print_mark.go

## Capabilities

### New Capabilities
- `gl-duplex-layout`: 总分类账正反面并排，奇数页正面 FrontStartCol，偶数页反面 BackStartCol

### Modified Capabilities
- `excel-generation`: 总分类账存储位置——不再是单区，而是奇偶页分 Front/Back 两区

## Impact

- generator/gl_sheet.go: appendToGLSheet 数据写入 + writePageHeader 标题 + writePageBreakRow/writeCarryForwardRow/insertCarryForward
- generator/merge_gl_sheet.go: appendToMergeGLSheet 数据写入
- generator/monthly_close.go: WriteMonthClosings 关账行写入
- generator/print_mark.go: 打印标记列
