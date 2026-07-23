## Phase 1: 辅助函数 dataCol + pageNum 传递

- [x] 1.1 在 `gl_sheet.go` 中新增 `dataCol(lay, pageNum, offset)` 函数
- [x] 1.2 编译通过：`go build ./generator/...`

## Phase 2: GL Sheet 偶数页写入

- [x] 2.1 `appendToGLSheet` 数据写入列用 `dataCol(lay, pageNum, offset)` 替换
- [x] 2.2 `writePageHeader` 偶数页写入 BackStartCol 区
- [x] 2.3 `insertCarryForward`、`writePageBreakRow`、`writeCarryForwardRow` 加入 pageNum
- [x] 2.4 编译通过 + `go test ./generator/... -count=1` 全绿

## Phase 3: 月结 + MergeGL + 读取端感知

- [x] 3.1 `monthly_close.go` — `WriteMonthClosings` 使用 dataCol
- [x] 3.2 `merge_gl_sheet.go` — `appendToMergeGLSheet` 写入列替换
- [x] 3.3 `WriteMergeGLClosings` / `writeMergeGLClosingRows` 写入列替换
- [x] 3.4 新增 `hasPageBreakAt` — Front/Back 两区感知
- [x] 3.5 `lastPageBalance`、`lastRowIsOrphanBreak`、`lastBreakTotals`、`pageStartRow`、`pageHasBreakRow`、`nextDataRow` 同步适配
- [x] 3.6 `ExtractLastMonthFinals`、`lastBreakDetailTotals`、`markExistingPageForPrint` 同步适配
- [x] 3.7 编译通过 + 测试全绿

## Phase 4: pageStartRow 偏移 + rowIsPageBreak 阈值

- [x] 4.1 pageStartRow: `i+3+DataStartRow` → `i+2+DataStartRow`（承前页计入 20 行）
- [x] 4.2 pageStartRow 首页: `return 3` → `return DataStartRow+1`
- [x] 4.3 rowIsPageBreak: `dataRows > pageSize` → `dataRows >= pageSize`
- [x] 4.4 编译通过 + 测试全绿

## Phase 5: 关账行区域修正

- [x] 5.1 `WriteMonthClosings`: `dataCol(lay, 1, N)` → `dataCol(lay, pageNum, N)`（按实际末页写入）
- [x] 5.2 `writeMergeGLClosingRows`: pageNum 硬编码 1 → 动态计算

## Phase 6: year_close + -f 级联 + 上年结转

- [x] 6.1 year_close: 创建空白文件（不复制旧数据）
- [x] 6.2 `-f` 级联: `>= month` → `> month`
- [x] 6.3 `NewWorkbook`: 支持加载当月预存文件（year_close 输出）
- [x] 6.4 上年结转仅在 1 月 `!isNew && initial != 0` 时写入
- [x] 6.5 `insertCarryForwardAtRow`: 在数据区首行（nextDataRow）写入
- [x] 6.6 `appendCarryForwardOnly`: 对无当月分录但有期初的科目处理
- [x] 6.7 `GetInitBalanceForGenerate`: 从科目树取最近月非零余额
- [x] 6.8 编译通过 + 测试全绿 + 跨年生成验证
