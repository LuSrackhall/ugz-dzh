# 明细科目独立账页（明细账独立科目）

## Why

ML（多科目明细账）按总账科目合并立页，明细科目以"金额分析"列展开。少数用户需要对**某个明细科目**单独核算（≈三级科目 privately 细分），希望为它单独开辟一张账页（如 `明细账-管理费用-办公费`），此时页内无需明细列，用三栏式（借/贷/余）最自然。

系统现状无法满足：明细只能挤在合并 ML 页的列里，无法独立成页。

## What Changes

1. **新配置** `全局设置.明细账独立科目`：叶子科目全路径数组（如 `["管理费用-办公费"]`）。**默认空 = 完全不生效**（零回归；对仗"合并总账科目"先例——少数人需求配置驱动）。
2. **新账页类型** `明细账-{全路径}`：三栏式，完整复用 GL 第 2 层页内结构（20+1 行、过次页/承前页、月结四行、横向双面装订），仅标题文字改"明 细 账"、会计科目=全路径。
3. **路由矩阵（加法而非搬移）**：
   - GL 叶子页：照常生成（期初/分录/月结全不动）——两账页体系互不影响，不提示不联动；
   - 合并 ML 页：命中名单的明细**从明细列排除**（列收缩，非留空列）；余额列口径不变（仍=总账科目全口径）；
   - 独立明细页：期初行 + 分录 + 月结 + finalize 全套；
   - JSON 余额链 / 报表套件 / 试算平衡 / 回写 / gen-close：**完全不动**（拆的是页不是账）。
4. **存量收敛**：名单移除后孤儿独立页在 generate 时删除（对仗"总分类账忽略科目"存量清理）；存量合并 ML 页仍含名单明细列时报错提示 `-f` 重建（对仗 detailOrder 冲突先例）。
5. **打印版**：独立页走 GL 打印变换（12 列拆位同构）；sheet 排序新增独立区块（GL→合并→明细账→ML）。

## Impact

- **代码**：`balance/balance.go`（Settings 加字段）、`generator/gl_sheet.go`（标题参数化 + 追加本体拆分 + finalize 前缀）、`generator/monthly_close.go`（月结循环体抽出共享）、`generator/ml_sheet.go`（明细列排除 + 冲突检测）、`generator/generate.go`（新步骤 + 存量清理）、`generator/workbook.go`（前缀常量 + 页面布局分支）、`generator/sheet_order.go`（新区块）、`generator/print_transform.go` + `print_font.go`（前缀分支）。新文件 `generator/detail_ledger_sheet.go`。
- **文档**：`.agents/skills/ledger-accounting/` 与 `embedded/ledger-accounting/` 双目录逐字同步（embed_test.go 守护）。
- **兼容**：默认空名单 = 现行为逐字节不变；JSON/报表/回写零改动，无迁移。
