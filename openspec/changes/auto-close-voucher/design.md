## Context

用户要求结转凭证自动生成但不污染手工凭证目录，选定"输出目录下 closing/ 专属子目录"方案。

## Goals / Non-Goals

**Goals:** gen-close 生成结转凭证到 closing/；generate 自动并入；本年收益入权益类；幂等（已结转跳过）。

**Non-Goals:** 不自动执行结转（用户确认后 generate 生效）；不改手工凭证目录；不做自动结转"静默入账"。

## Decisions

**D1 存放位置**：`<output>/{year}/closing/记字第X号 年末损益结转.md`——JSON 同级、账本体系内、git 可见；手工凭证目录零写入。

**D2 gen-close 逻辑**：
- 年份：从 `-j` 路径文件名（`2025.json` → 2025）。
- 余额来源：`cfg.Tree` 中 `{year}-12` 的 Balances（无 12 月则取 year 内最大月）。
- 已结转科目：扫描 closing/ 已有凭证的条目 GeneralAccount 集合，跳过。
- 分录生成（按余额符号）：借余（final>0）→ 借 本年收益/贷 科目；贷余（final<0）→ 借 科目/贷 本年收益。
- 编号：closing/ 已有文件数+1 起，直到无重名。
- 日期：{year}-12-31。

**D3 generate 并入**：cmd/generate.go 解析凭证后，追加 `<output>/{year}/closing/*.md` 的解析结果（同解析器），提示"已并入 N 张自动结转凭证"。

**D4 科目类别**：`accountTypes["本年收益"] = "权益"`（结转目标科目）。

**D5 year-close 提示**：损益未结转告警追加"可用 ledger gen-close 生成结转凭证"。

## Risks / Trade-offs

- **closing 目录不存在时**：generate 静默跳过（无提示）；gen-close 自动创建。
- **编号冲突**：自动递增规避覆盖；用户改名/新增文件后编号继续递增。
- **结转凭证格式**：复用标准凭证 md（解析器兼容）；"本年收益"明细为空。
