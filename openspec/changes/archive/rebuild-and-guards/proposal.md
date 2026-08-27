## Why

用户要求全部挂起事项完成。本批（Change 12）为工程增强：

1. **全量重建脚本**（投产评估【强烈建议】）：一键从 JSON + 凭证目录逐月 `-f` 重建全年（现需手工逐月）。
2. **check xlsx 漂移比对**（建议）：check 对比 xlsx 期末与 JSON Balances，防手工改表漂移。
3. **凭证号重号检测**（记录项）：同目录两个文件同名"记字第X号" → 告警（防漏账/错分组）。
4. **红字口径文档**（设计专家建议）：README 说明红字表达（带符号进借贷列，等价于同侧红字）。

**范围说明**：日记账打印版不加位格——日记账为流水账（逐笔+日结），查看版普通表格可直接打印，位格拆位主要用于 GL/ML 手写习惯，本批以 README 说明"日记账直接打印查看版"。

## What Changes

1. `scripts/rebuild.sh`：读 `{year}.json` 启动月 → 逐月 `generate -f`（凭证目录支持 `{YEAR}_{MM}` / `{YEAR}/{MM}` 两种形态）。
2. `cmd/check.go`：加载最新月 xlsx，读 `{month}期末` sheet 科目余额，与 `cfg.Tree.Balances[month].Final` 对比，不一致输出"漂移"告警。
3. `cmd/generate.go`：凭证目录内文件名"记字第X号"重复 → 告警（重号）。
4. `README.md`：红字表达章节（带符号 `-500`/`(500)` 进借贷列 = 会计同侧红字语义）+ 日记账打印说明。

## Capabilities

### New Capabilities
- `rebuild-script`: 全量重建脚本
- `xlsx-drift-check`: check 漂移比对
- `voucher-number-dup-warning`: 凭证号重号告警
- `red-ink-doc`: 红字口径文档

## Impact

- `scripts/rebuild.sh`（新）、`cmd/check.go`、`cmd/generate.go`、`README.md`
- 测试：rebuild 脚本 e2e 验证、check 漂移比对、重号告警；e2e 回归
