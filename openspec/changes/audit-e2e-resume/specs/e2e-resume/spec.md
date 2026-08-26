# e2e-resume Spec

## ADDED Requirements

### Requirement: --keep-json 从已有 JSON 续跑
`test-e2e.sh --keep-json` 跳过 `rm -rf "$OUT"` 与 `init`，校验 `out/2025/2025.json` 存在后从"生成"阶段续跑；JSON 不存在时报错并提示先运行完整流程。生成仍带 `-f`（配合 CLI 安全重建）。

#### Scenario: 续跑成功
- **WHEN** 已运行完整流程（out/2025/2025.json 存在），执行 `--keep-json`
- **THEN** 不执行 rm -rf / init，直接生成 2025-10~12、year-close、2026-01~06，全部成功

#### Scenario: 无 JSON 报错
- **WHEN** out/2025/2025.json 不存在，执行 `--keep-json`
- **THEN** 报错"请先运行完整流程（不带 --keep-json）"，退出非 0

### Requirement: 配置化科目映射与期初调整（幂等）
若 `test/e2e/mappings.json` 存在，循环执行 `map add`（跳过已存在映射）；若 `test/e2e/adjustments.json` 存在，循环执行 `add-manual`（跳过已存在条目）。重复执行不报错。

#### Scenario: 映射与期初配置生效
- **WHEN** mappings.json = {"管埋费用":"管理费用"}、adjustments.json = [{"科目":"银行存款-工商银行","生效月":"2025-10","金额":100000,"说明":"建账期初"}]，执行 `--keep-json`
- **THEN** 生成前自动执行 map add 与 add-manual；重复执行第二次不报错（跳过已存在）
