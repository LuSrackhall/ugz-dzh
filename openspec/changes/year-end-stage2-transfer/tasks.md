# 任务：年末二段结转

## 1. 实现

- [ ] 1.1 `cmd/gen_close.go`：抽出纯函数（pnl 结转求和 / postFinal 重算 / 二段方向判定），主流程接入二段（触发条件、早退放宽、编号顺延、科目自动登记、凭证写出）
- [ ] 1.2 `--no-transfer` flag 与 Long 帮助文案更新
- [ ] 1.3 单测 `cmd/gen_close_test.go`：方向两态（净收益/净亏损）、postFinal 三运行态同值、hasTransfer 防重、早退放宽、科目自动登记
- [ ] 1.4 集成链路单测（进程内 runCmd，参照 gen_close_strict_test.go）：gen-close → generate -f → 断言 损益=0、本年收益=0、目标科目=净额 → 幂等重跑 gen-close 不出新凭证

## 2. e2e

- [ ] 2.1 `test/e2e/` 新增年末全流程场景（风格参照 flush_flat_repro_test.go）：建账→数月→gen-close→-f 12月→断言损益全 0、本年收益=0、收益分配=净额、资产负债表权益列口径（无"未结转损益"行、收益分配行在列）→year-close→新年 1 月期初断言
- [ ] 2.2 e2e 断言混合月态（结转历史 + 当年未结转 → 权益列两行并存）

## 3. 文档

- [ ] 3.1 AGENTS.md 1.4 更正（year_close 现行为）
- [ ] 3.2 技能双目录同步：commands.md / json-schema.md / SKILL.md（gen-close 二段、--no-transfer、权益列口径、年末流程顺序），embed_test.go 守护通过

## 4. 验证与发版

- [ ] 4.1 `go test ./... -count=1` 全绿
- [ ] 4.2 `bash scripts/test-e2e.sh` 全流程全绿
- [ ] 4.3 CHANGELOG 人话节 + SKILL.md version 双目录 bump
- [ ] 4.4 干净树提交（中文 commit message）
- [ ] 4.5 发版六步之 push/tag——**须用户显式批准后执行**
- [ ] 4.6 完工更新 .workbuddy/memory/（MEMORY.md 对应小节 + 当天日志）
