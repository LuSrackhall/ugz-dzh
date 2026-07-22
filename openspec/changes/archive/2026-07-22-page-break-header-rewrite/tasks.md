## Phase 1: pageStartRow 偏移更新

- [x] 1.1 更新 `pageStartRow` 偏移量：`return i + 3` → `return i + 3 + lay.DataStartRow`
- [x] 1.2 确认 `rowIsPageBreak` 和 `pageHasBreakRow` 不受影响（它们调用 pageStartRow，自动适配）
- [x] 1.3 编译通过：`go build ./generator/...`

## Phase 2: writePageHeader 实现

- [x] 2.1 从 `writeGLTitle` 提取标题区写入逻辑为 `writePageHeader(sheet, pageNum, account)`（复用现有 Layout 坐标和样式代码）
- [x] 2.2 编译通过

## Phase 3: appendToGLSheet 集成

- [x] 3.1 `appendToGLSheet` 增加页码初始化逻辑（遍历 GetRows 统计过次页数）
- [x] 3.2 过次页/承前页后调用 `writePageHeader`
- [x] 3.3 编译通过

## Phase 4: 测试 + 验证

- [x] 4.1 运行 `go test ./... -count=1` 全绿（除了 e2e 测试数据丢失）
- [x] 4.2 编译：`go build -o ledger .`
- [ ] 4.3 e2e：`go test ./test/e2e/... -count=1 -timeout 180s`
- [ ] 4.4 跨年生成：`bash scripts/test-e2e.sh --skip-test`
