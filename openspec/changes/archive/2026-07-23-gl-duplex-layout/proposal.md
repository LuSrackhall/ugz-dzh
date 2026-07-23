## Why

当前总分类账仅使用 FrontStartCol 区域（左侧正面），BackStartCol 区域（右侧反面）预留了列宽但从未写入数据。需要启用反面区域，并修复关联的 year_close/carry-forward/级联删除等问题。

## What Changes

- 偶数页数据写入 BackStartCol+offset 而非 FrontStartCol+offset
- 读取端 hasPageBreakAt 两区域感知（修复余额链断裂风险）
- pageStartRow 偏移更新 + rowIsPageBreak >= 阈值修正
- -f 级联从 >= month 改为 > month，保留 year-close 输出
- year_close 创建空白文件（不复制旧明细数据）
- 上年结转仅 1 月写入内容区首行
- 补充历史余额期初计算

## Capabilities

- `gl-duplex-layout`: 总分类账正反面并排，读取端两区感知，上年结转正确写入

## Impact

- generator/gl_sheet.go: dataCol, writePageHeader colOffset, hasPageBreakAt, pageStartRow, rowIsPageBreak, insertCarryForwardAtRow
- generator/merge_gl_sheet.go: dataCol 同步
- generator/monthly_close.go: WriteMonthClosings pageNum
- generator/print_mark.go: markExistingPageForPrint
- generator/workbook.go: NewWorkbook 加载预存文件, ExtractLastMonthFinals 两区扫描
- generator/generate.go: appendCarryForwardOnly
- balance/balance.go: GetInitBalanceForGenerate 历史余额
- cmd/year_close.go: 创建空白文件
- cmd/generate.go: -f 级联 > month
- scripts/test-e2e.sh: 适配变更
