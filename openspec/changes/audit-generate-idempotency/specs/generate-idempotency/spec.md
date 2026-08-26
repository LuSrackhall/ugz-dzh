# generate-idempotency Spec

## ADDED Requirements

### Requirement: 已生成判定基于"本月合计"行
`generate`（不带 `-f`）当月 xlsx 存在时，检测工作薄是否含"本月合计"行：含 → 拒绝生成并要求 `-f`；不含（如 year-close 预生成的空工作薄）→ 允许直接生成。

#### Scenario: 同月重跑被拒
- **WHEN** 2026-03.xlsx 已生成（含"本月合计"行），再次 `generate` 不带 `-f`
- **THEN** 报错"已生成过（含本月合计行），使用 -f 覆盖重建"，不追加任何数据

#### Scenario: year-close 空文件放行
- **WHEN** year-close 已预生成空 2026-01.xlsx，`generate` 2026-01 不带 `-f`
- **THEN** 允许生成（加载空文件），不要求 -f

### Requirement: 无活动月份补月结行
科目当月无分录但期初（上月末余额）≠0 且存在 GL Sheet 时，`WriteMonthClosings` 补写"本月合计 0/0 + 本年累计 + 期末余额=期初"行，使账本逐月连续清晰。

#### Scenario: 无活动科目补月结行
- **WHEN** 科目 X 期初 1000、当月无分录（存在 GL Sheet）
- **THEN** X 的 Sheet 追加"本月合计 0.00/0.00"行与"期末余额 1000.00"行

#### Scenario: 无余额科目不补
- **WHEN** 科目 X 期初=0 且当月无分录
- **THEN** 不补月结行（无余额无存在感，避免噪声）
