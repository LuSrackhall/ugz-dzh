## Why

用户工作流：**init → generate（自动识别科目写入 JSON）→ 人工完善 JSON（map add 科目映射防 OCR 错字意外科目、add-manual 补期初）→ 从完善后 JSON 重新生成账本**。但 `test-e2e.sh` 每次 `rm -rf out + init` 从零开始，无法续跑"完善后重新生成"的完整链路，也无法一键验证 CLI 的幂等/期初/映射修复（Change 1/2/3）在真实两阶段工作流下的安全性。

## What Changes

1. **新增 `--keep-json` 参数**：跳过 `rm -rf "$OUT"` 与 `init`，校验已有 `out/2025/2025.json` 存在后直接从"生成"阶段续跑（生成带 `-f`，配合 Change 1 D8 安全重建）。
2. **配置化"人工完善"（可选，幂等）**：若 `test/e2e/mappings.json`（科目映射表）与 `test/e2e/adjustments.json`（期初调整列表）存在，在续跑生成前自动执行 `map add` / `add-manual`（已存在的条目跳过，保证重复执行安全）。
3. 用途：一键复现"init→生成→完善→重新生成"两阶段工作流，验证 CLI 幂等保护、期初调整生效、科目映射合并。

## Capabilities

### New Capabilities
- `e2e-resume`: e2e 脚本两阶段工作流——`--keep-json` 从已有 JSON 续跑、配置化科目映射与期初调整（幂等）

### Modified Capabilities
- 无（`scripts/test-e2e.sh` 为测试工具，非 CLI 产物）

## Impact

- `scripts/test-e2e.sh`：参数解析（--keep-json）、阶段 2 分支（跳过/保留 rm+init）、映射/期初配置化执行（幂等）
- 可选新增 `test/e2e/mappings.json`、`test/e2e/adjustments.json`（示例配置，不提交数据时可为空）
- 不变：CLI 命令本身（依赖 Change 1/2/3 的安全行为）
