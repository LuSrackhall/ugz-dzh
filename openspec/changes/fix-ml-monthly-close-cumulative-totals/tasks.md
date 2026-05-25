## 1. 修复多科目明细账月结行 A-G 列累计值聚合

- [x] 1.1 修改 `generator/monthly_close_ml.go` "本季合计"行（第 92-104 行区域）：将 `qtDebit += qtdDebit[general]` / `qtCredit += qtdCredit[general]` 替换为遍历 `details` 列表，用 `general + "-" + detail` 全路径聚合 `qtdDebit` / `qtdCredit`
- [x] 1.2 修改 `generator/monthly_close_ml.go` "本年累计"行（第 129-140 行区域）：将 `cumDebit += ytdDebit[general]` / `cumCredit += ytdCredit[general]` 替换为遍历 `details` 列表，用 `general + "-" + detail` 全路径聚合 `ytdDebit` / `ytdCredit`
- [x] 1.3 运行 `go vet ./...` 确保无静态分析错误
- [x] 1.4 运行 `go test ./generator/... -v` 确保现有测试通过

## 2. 自动化测试验证

- [x] 2.1 编写自动化单元测试 `TestWriteMLMonthClosings_CumulativeAggregation`：模拟两明细科目（工行/建行）多月份历史数据，对季末月份 03 生成月结行，验证"本季合计" D/E 列 = 当月发生额 + 截至上月全路径聚合的季度累计
- [x] 2.2 同一测试验证"本年累计" D/E 列 = 当月发生额 + 截至上月全路径聚合的年度累计
- [x] 2.3 运行 `go test ./generator/... -v -run TestWriteMLMonthClosings_CumulativeAggregation` — PASS
- [x] 2.4 运行 `go test ./...` 全量测试 — 无回归
- [x] 2.5 运行 `go vet ./...` — 无静态分析错误
