## 1. 改造 centsToYuan 函数

- [x] 1.1 修改 `generator/workbook.go`: 将 `centsToYuanStr` 重命名为 `centsToYuan`，返回类型从 `string` 改为 `float64`，实现从 `fmt.Sprintf` 改为 `float64(c) / 100`
- [x] 1.2 全局替换所有调用点: `centsToYuanStr(` → `centsToYuan(` (33处)
- [x] 1.3 更新 `generator/generator_test.go` 中 `centsToYuanStr` 的测试引用和断言类型

## 2. 验证

- [x] 2.1 运行 `go build ./...` 确保编译通过
- [x] 2.2 运行 `go test ./...` 确保所有测试通过
- [x] 2.3 运行 4 个月 e2e 管线 `go run scripts/verify_ml_closings.go ...` 确保月结数值不变
- [x] 2.4 手动抽查生成的 xlsx 中金额单元格为数字类型（非文本）
