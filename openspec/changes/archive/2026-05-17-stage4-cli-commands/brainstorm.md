## Design Summary

将当前单文件 `main.go`（273行，flag包）重构为 `cmd/` 包 + cobra 子命令体系，支持 `project.md` 中规划的 6 个子命令。保持零外部框架依赖，引入 `spf13/cobra` 作为唯一新增依赖。

## Alternatives Considered

### 方案 A：保持 flag 包 + 子命令枚举
- **做法**：在现有 main.go 中用 `os.Args[1]` 判断子命令，flag 包解析各自参数
- **优点**：零新依赖，改动最小
- **缺点**：长尾维护成本高（help 文本、参数验证、错误处理全是手写），无法生成 shell completion
- **为何未采用**：6 个子命令的手写路由和 help 代码量将超过 cobra 方案，且体验差

### 方案 B：引入 cobra + cmd 包
- **做法**：`go get github.com/spf13/cobra`，在 `cmd/` 下为每个子命令创建独立文件，共用逻辑抽到 `cmd/common.go`
- **优点**：标准 CLI 框架（Kubernetes、Hugo 等大量项目使用），自动 help/completion，参数验证内置
- **缺点**：新增一个依赖
- **为何采用**：6 个子命令天然适合 cobra，cobra 是 Go 生态事实标准，依赖极轻（单包）

### 方案 C：自研子命令路由
- **做法**：手写一个轻量子命令路由器
- **优点**：零依赖
- **缺点**：重复造轮子，help/completion/错误处理全要手写，不如直接引入 cobra
- **为何未采用**：得不偿失

## Agreed Approach

方案 B — 引入 `spf13/cobra`，创建 `cmd/` 包。

子命令设计：

```
ledger generate  -month YYYY-MM -voucherDir ./vouchers -json config.json -output ./output
ledger check     -json config.json
ledger reset     -json config.json -month YYYY-MM
ledger add-manual -account 科目 -month YYYY-MM -amount 100.00 -note 说明 -json config.json
ledger init      -start-month YYYY-MM -output .
ledger year-close -json config.json -output .
```

## Key Decisions

1. **cobra 作为唯一新增依赖** — Go CLI 生态事实标准，引入成本极低
2. **`cmd/` 包结构** — 每个子命令一个文件：`generate.go`, `check.go`, `reset.go`, `add_manual.go`, `init.go`, `year_close.go`，共用逻辑放 `cmd/common.go`
3. **主入口 `main.go` 瘦身** — 仅调用 `cmd.Execute()`，原 CSV/XLSX 输出逻辑移入 `cmd/generate.go`
4. **`ledger check` 验证科目树完整性** — 调用 `balance.ValidateAccountTree`
5. **`ledger reset` 重置打印标记** — 清除指定月 xlsx 中所有"需打印"标记
6. **`ledger add-manual` 手动调整科目** — 调用 `balance.AddManualAdjustment`
7. **`ledger init` 系统初始化** — 创建初始 `科目余额总览.json`
8. **`ledger year-close` 跨年结转** — 执行年末结账逻辑

## Open Questions

无 — 所有关键决策已明确。
