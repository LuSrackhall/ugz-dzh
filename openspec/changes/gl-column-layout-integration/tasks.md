## Phase 1: 测试先行（TDD）

- [x] 1.1 在 `generator_test.go` 中新增 `TestGLColumnLayoutConsistency`：用 Layout 坐标写入一行完整数据 → 调用 GetRows 验证 `row[BindingLeftCols+2]` = 摘要，`row[BindingLeftCols+6]` = 余额
- [x] 1.2 确认当前所有测试全绿：`go test ./... -count=1`

## Phase 2: GL Sheet 列映射替换

- [x] 2.1 `gl_sheet.go` — `appendToGLSheet` 内所有 `cellName(1-8, row)` 替换为 `cellName(lay.FrontStartCol+0~7, row)`；编译通过
- [x] 2.2 `gl_sheet.go` — `insertCarryForward`、`writePageBreakRow`、`writeCarryForwardRow` 写入列替换；编译通过
- [x] 2.3 `gl_sheet.go` — 读取端辅助函数索引偏移（`lastPageBalance`、`lastRowIsOrphanBreak`、`lastBreakTotals`、`pageStartRow`、`rowIsPageBreak`、`pageHasBreakRow`）；编译通过
- [x] 2.4 运行 `go test ./generator/... -count=1` 全绿

## Phase 3: 月结 + 合并 GL + ML 列映射替换

- [x] 3.1 `workbook.go` — `ExtractLastMonthFinals` 读取索引偏移；编译通过
- [x] 3.2 `monthly_close.go` — `WriteMonthClosings` + `nextDataRowAfterBreak` 替换；编译通过
- [x] 3.3 `merge_gl_sheet.go` — `appendToMergeGLSheet` + `WriteMergeGLClosings` + `writeMergeGLClosingRows` 替换；编译通过
- [x] 3.4 `ml_sheet.go` — `appendToMLSheet` + `writeMLPageBreakRow` + `writeMLCarryForwardRow` + `lastBreakDetailTotals` 替换（注意：ML 扩展列 H-U 不动）；编译通过
- [x] 3.5 `print_mark.go` — `markRowForPrint` 列号由 8 → `lay.FrontStartCol + len(lay.ExcelColumns)`；编译通过
- [x] 3.6 运行 `go test ./... -count=1` 全绿

## Phase 4: 测试断言更新

- [x] 4.1 `merge_gl_sheet_test.go` — 更新硬编码行号断言（Row 4→5 偏移，列索引+BindingLeftCols）
- [x] 4.2 运行 `go test ./... -count=1` 全绿

## Phase 5: 集成验证

- [ ] 5.1 编译：`go build -o ledger .`
- [ ] 5.2 e2e 测试：`go test ./test/e2e/... -count=1 -timeout 180s`
- [ ] 5.3 跨年生成：`bash scripts/test-e2e.sh --skip-test`
- [ ] 5.4 确认 TDD 测试用例通过（Phase 1.1 的断言成立）
