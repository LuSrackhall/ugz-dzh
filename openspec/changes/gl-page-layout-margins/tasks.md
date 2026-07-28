## 1. 常量定义与行偏移调整

- [x] 1.1 在 `generator/gl_sheet.go` 中新增 `TopMarginRows = 3` 和 `BottomMarginRows = 3` 常量
- [x] 1.2 更新 `generator/layout/gl_layout.go` 中的 `DataStartRow`（5 → 8）
- [x] 1.3 更新 `generator/layout/gl_layout.go` 中的 `HeaderRow`（3 → 6）、`SubHeaderRow`（4 → 7）
- [x] 1.4 同步调整 `generator/gl_sheet.go` 中所有使用行偏移的函数（`writeGLTitle`、`pageStartRow`、`nextDataRow` 等）
- [x] 1.5 同步调整 `generator/monthly_close.go` 中的 `nextDataRowAfterBreak` 和所有 `WriteMonthClosings` 行引用
- [x] 1.6 同步调整 `generator/merge_gl_sheet.go` 中的行引用
- [x] 1.7 更新 `writePageHeader` 中后续页的行偏移量（子表头行 `row + 4` → 同步变化）

## 2. A4 列宽换算与页面设置

- [x] 2.1 在 `generator/layout/gl_layout.go` 中将 `GLMMToExcelColWidth` 的 `pxPerColUnit` 从 3.5 改为 7.0
- [x] 2.2 在 `generator/workbook.go` 中添加 `SetPageLayout`（A4 横向、PaperSize=9）
- [x] 2.3 在 `generator/workbook.go` 中添加 `SetPageMargins`（上/下/左/右均为 0）

## 3. 外侧边缘红色双线

- [ ] 3.1 在 `generator/gl_sheet.go` 中新增 `setNoBorder` 辅助函数
- [ ] 3.2 更新 `finalizeAllGLSheets`：遍历每页时判断奇偶，正面页左侧/背面页右侧应用 `setRedDoubleBorder`
- [ ] 3.3 更新 `finalizeAllGLSheets`：对面侧（正面页右侧/背面页左侧）应用 `setNoBorder` 清理残留边框
- [ ] 3.4 确保红色双线纵向覆盖从上边距第 1 行到底边距最后 1 行

## 4. 验证

- [ ] 4.1 `go build ./...` 无编译错误
- [ ] 4.2 `go test ./... -count=1` 全部通过
- [ ] 4.3 `bash scripts/test-e2e.sh --skip-test` 生成成功
- [ ] 4.4 目测验证正/背面页外边缘边框正确
- [ ] 4.5 目测验证打印预览符合 A4 横向
