## 1. 修复 detailOrder 预分配列完整展开

- [x] 1.1 修改 `ensureMLSheet` 新 Sheet 初始化逻辑（`generator/ml_sheet.go:75-101`）：detailOrder 存在时，`initDetails` 直接复制 `detailOrder` 完整列表（含 `""` 跳列），当月分录中不在 `detailOrder` 的科目按字母序追加到右侧空列
- [x] 1.2 运行 `go test ./generator/... -v` 验证单元测试通过
- [x] 1.3 运行 `go vet ./...` 确保无静态分析错误

## 2. 修复 -f 级联删除后续月份 xlsx

- [x] 2.1 修改 `cmd/generate.go` 的 `-f` 分支：删除 `{yearDir}` 下当月及之后所有月份的 `*.xlsx` 文件
- [x] 2.2 运行 `go test ./... -v` 确保全量测试通过

## 3. 端到端验证

- [ ] 3.1 手动测试：编辑 `detailOrder` 包含跳列和预分配科目，`-f` 从首月重建，检查标题列展开正确
- [ ] 3.2 手动测试：`-f` 非首月后，后续月份 `generate` 列布局一致
