# 任务：明细科目独立账页

## 1. 配置与命名
- [x] 1.1 `balance/balance.go`：`GlobalSettings` 加 `DetailStandaloneAccounts []string`（json `明细账独立科目,omitempty`）。
- [x] 1.2 `generator/workbook.go`：`sheetPrefixDetail = "明细账-"` 常量 + `sheetNameDetail()` + `wb.detailStandalone()` 判定。

## 2. 账页写入（gl_sheet.go + 新文件）
- [x] 2.1 `writeGLTitle` → `writeLedgerTitle(sheet, title)` 参数化（前缀剥离、标题文字）；GL 调用点传原标题。
- [x] 2.2 `writePageHeader` 标题文字按 sheet 前缀切换。
- [x] 2.3 `appendToGLSheet` 拆共享本体 `appendToLedgerSheetBody`；新增 `ensureDetailSheet` + `appendToDetailSheet`（新文件 `generator/detail_ledger_sheet.go`）。
- [x] 2.4 `AppendDetailLedgerEntries`（generate.go 步骤 6.2）：分录路由 + 仅期初建页 + 合并父级进名单报错。

## 3. 月结与 changedSheets
- [x] 3.1 `WriteMonthClosings` 循环体抽 `writeLedgerClosings`；新增独立页月结循环。
- [x] 3.2 `CollectChangedSheets` 为名单 path 追加独立页键。

## 4. ML 排除与存量收敛
- [x] 4.1 `AppendMLEntries`：构建 details 时排除名单明细（列收缩）+ 存量残留列报错。
- [x] 4.2 `generate.go` 存量清理：孤儿 `明细账-` 页删除。

## 5. 排序 / finalize / 打印
- [x] 5.1 `sheet_order.go`：sectionDetail 区块（GL→合并→明细账→ML）。
- [x] 5.2 `finalizeAllGLSheets` 前缀扩展；`setAllSheetPageLayout` GL 分支扩展。
- [x] 5.3 `print_transform.go` case + `isLedgerSheet` 扩展。

## 6. 测试
- [x] 6.1 单测：判定/ML 排除/冲突报错/期初标签/存量清理/父级校验。
- [x] 6.2 e2e：多名单明细混跑数月全断言（GL 在、独立页链连续、ML 无列、JSON/报表不变、排序、-f 收敛）。
- [x] 6.3 `go test ./... -count=1` + `bash scripts/test-e2e.sh` 全绿。

## 7. 文档
- [x] 7.1 双目录技能文档：json-schema.md（新配置字段+语义+存量收敛）、commands.md（generate 节）、SKILL.md（配置示例）。
- [x] 7.2 embed_test.go 守护通过（双目录逐字一致）。
