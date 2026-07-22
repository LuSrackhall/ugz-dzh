## Phase 1: 辅助函数 dataCol + pageNum 传递

- [x] 1.1 在 `gl_sheet.go` 中新增 `dataCol(lay, pageNum, offset)` 函数
- [x] 1.2 编译通过：`go build ./generator/...`

## Phase 2: GL Sheet 偶数页写入

- [x] 2.1 `appendToGLSheet` 数据写入列用 `dataCol(lay, pageNum, offset)` 替换
- [x] 2.2 `writePageHeader` 偶数页写入 BackStartCol 区
- [x] 2.3 `insertCarryForward`、`writePageBreakRow`、`writeCarryForwardRow` 加入 pageNum
- [x] 2.4 编译通过 + `go test ./generator/... -count=1` 全绿

## Phase 3: 月结 + MergeGL 偶数页写入

- [x] 3.1 `monthly_close.go` — `WriteMonthClosings` 写入列用 dataCol 替换
- [x] 3.2 `merge_gl_sheet.go` — `appendToMergeGLSheet` 写入列替换
- [x] 3.3 `WriteMergeGLClosings` / `writeMergeGLClosingRows` 写入列替换
- [x] 3.4 编译通过

## Phase 4: 集成验证

- [x] 4.1 `go test ./generator/... -count=1` 全绿
- [x] 4.2 `go test ./... -count=1` 全绿
- [x] 4.3 `go build -o ledger .`
- [x] 4.4 `bash scripts/test-e2e.sh` 全绿

## Phase 4: 打印标记 + 完整性验证

- [x] 4.1 `print_mark.go` — `markRowForPrint` / `markRowsForPrint` / `markExistingPageForPrint` 加入 pageNum 判断
- [x] 4.2 `go test ./generator/... -count=1` 全绿
- [x] 4.3 `go test ./... -count=1` 全绿
- [x] 4.4 `go build -o ledger .`
- [ ] 4.5 `bash scripts/test-e2e.sh` 全绿
