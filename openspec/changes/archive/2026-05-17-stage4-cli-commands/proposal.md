## Why

当前 `main.go` 使用 `flag` 标准包，所有功能（CSV/XLSX 输出、generator 调用）都揉在单一的 `main()` 函数中。随着 `project.md` 规划的 6 个子命令（generate/check/reset/add-manual/init/year-close）逐步落地，flag 包的手写路由、help 文本、参数验证将失控。现在阶段 1-3 的核心模块（voucher/balance/generator）已经完成，是引入标准 CLI 框架的最佳时机——后续工作将直接受益于清晰的命令结构和自动生成的帮助信息。

## What Changes

将 `main.go` 从单一 flag 入口重构为 `cmd/` 包 + cobra 子命令体系。

**main.go**
- From: 273 行单体文件，包含全部逻辑（CSV/XLSX 输出、generator 调用、辅助函数）
- To: 仅调用 `cmd.Execute()`，瘦身为 10 行入口

**新增 `cmd/` 包**
- `cmd/root.go` — cobra 根命令与全局持久化参数
- `cmd/generate.go` — `ledger generate` 子命令（原 main.go 逻辑移入）
- `cmd/check.go` — `ledger check` 验证科目树完整性
- `cmd/reset.go` — `ledger reset` 重置打印标记
- `cmd/add_manual.go` — `ledger add-manual` 手动调整科目
- `cmd/init.go` — `ledger init` 系统初始化（创建初始 JSON）
- `cmd/year_close.go` — `ledger year-close` 跨年结转
- `cmd/common.go` — 共用函数（centsToYuan、cellName 等）

**依赖变更**
- 新增: `github.com/spf13/cobra`

## Capabilities

### New Capabilities

- `cli-commands`: 6 个 cobra 子命令（generate/check/reset/add-manual/init/year-close），统一入口 `ledger`

### Modified Capabilities

无 — 现有功能不变，仅入口重构

## Impact

- `main.go`: 从 273 行瘦身为 ~10 行入口
- 新增 `cmd/` 包（~8 个文件）
- `go.mod`: 新增 `spf13/cobra` 依赖
- 用户使用方式从 `ledger -flag...` 变为 `ledger <command> [flags...]`
- 现有功能无回归（generate 子命令输出与当前一致）
