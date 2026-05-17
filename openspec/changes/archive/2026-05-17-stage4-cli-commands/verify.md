# Verify: Stage 4 CLI 子命令

## 变更概述

将 `main.go` 从 273 行 flat flag-based CLI 重构为 `cmd/` 包，包含 6 个 cobra 子命令。

## 验证结果

### 测试

| 包 | 状态 |
|---|---|
| `ledger/balance` | ✅ PASS |
| `ledger/cmd` (10 tests) | ✅ PASS |
| `ledger/generator` | ✅ PASS |
| `ledger/test/e2e` (5 test cases) | ✅ PASS |
| `ledger/voucher` | ✅ PASS |

### 子命令

| 命令 | 状态 | 说明 |
|---|---|---|
| `generate` | ✅ | `-v voucherDir` 必填，`-o output` `-m month` `-j json` 可选 |
| `init` | ✅ | `-s startMonth` 必填，拒绝覆盖已有配置 |
| `check` | ✅ | `-j json` 必填，调用 ValidateAccountTree |
| `add-manual` | ✅ | `-a account` `-m month` `-j json` 必填，`-n amount` `-t note` 可选 |
| `reset` | ✅ | `-m month` 必填，清除"需打印"标记 |
| `year-close` | ✅ | `-j json` 必填，跨年结转 |

### Spec 场景覆盖

- ✅ `ledger` 裸调用打印帮助
- ✅ `ledger generate --help` 打印子命令帮助
- ✅ `generate -voucherDir` 必填，缺失时非零退出
- ✅ `generate` 输出 ledger.csv + balance.csv + xlsx（e2e 验证）
- ✅ `generate` with `-json` + `-month` 生成完整累计工作薄
- ✅ `init` 创建配置，拒绝覆盖（TestInitSubcommandOverwriteProtection）
- ✅ `check` 调用 ValidateAccountTree
- ✅ `add-manual` 添加手动调整
- ✅ `reset` 清除打印标记
- ✅ `year-close` 跨年结转

## 结论

所有 14 项任务完成，全量测试通过，e2e 覆盖正常路径和错误路径。通过验证。
