## Context

test-e2e.sh 固定 `rm -rf out + init` 全流程，无法续跑"完善 JSON 后重新生成"两阶段工作流；用户需要脚本作为验证 CLI 安全行为（幂等/期初/映射，Change 1/2/3）的测试工具，而不是替代 CLI 安全机制。

## Goals / Non-Goals

**Goals:**
- `--keep-json`：跳过 rm+init，从已有 JSON 续跑
- 配置化完善（mappings.json / adjustments.json）幂等执行
- 完整复现"init→生成→完善→重新生成"链路

**Non-Goals:**
- 不改 CLI 命令（脚本只调用，安全靠 CLI 内建）
- 不提交具体测试数据（配置文件可选、可为空）
- 不做打印版相关改动

## Decisions

**D1 参数解析**：`--keep-json` 与 `--skip-test` 并列解析；`--keep-json` 仅控制阶段 2（rm+init）分支，其余流程结构不变（生成 2025-10~12、year-close、2026-01~06）。

**D2 续跑前置校验**：`--keep-json` 时若 `out/2025/2025.json` 不存在 → 报错退出（提示先跑完整流程）。

**D3 配置化完善（幂等）**：生成前（keep-json 模式）读取：
- `test/e2e/mappings.json`：`{"旧名":"新名",...}` → 内联 python 检查 `全局设置.科目映射表` 已存在则跳过，否则 `ledger map add`；
- `test/e2e/adjustments.json`：`[{"科目","生效月","金额","说明"}]` → 检查 `手动调整科目` 已有相同科目+生效月则跳过，否则 `ledger add-manual`。
配置文件不存在则静默跳过（保持向后兼容）。

**D4 生成仍用 -f**：续跑重建必须安全（Change 1 D8 已保证 -f 期初基线正确；Change 3 幂等判定对 -f 走删除重建路径）。

## Risks / Trade-offs

- **脚本复杂度**：内联 python 做幂等检查增加脚本长度；以函数注释分节，保持可读。
- **year-close 重复执行**：keep-json 续跑会再次 year-close（覆盖创建空 2026-01.xlsx），随后 -f 重建——幂等安全。
- **配置文件格式错误**：python 解析失败时给出明确错误并退出（set -e）。
