## Why

Stage 3 是手工账电子化系统的核心输出层。目前 voucher 解析和 balance 余额管理已就绪，但缺少将数据生成为完整累计 Excel 工作薄的能力。需要创建 generator 包，实现总分类账和多科目明细账两种账页格式的自动生成。

## What Changes

- 新建 `generator/` 包，实现累计 xlsx 工作薄生成器
- 为每个叶子科目生成独立的总分类账 Sheet（日期/凭证号/摘要/借方/贷方/方向/余额）
- 为有明细的总账科目生成多科目明细账 Sheet（基础列 A-G + 扩展列 H-U 对应各明细科目）
- 多科目明细账中每个总账科目汇总其下所有明细
- 每页固定 20 行数据行 + 过次页行
- 自动处理期初结转（新科目首行插入"上年结转"）、月结、余额计算
- 余额公式：页首行引用上页过次页余额或期初，后续行 = 上行余额 + 借方 - 贷方
- 打印标记：新写入数据行标记为"需打印"
- 生成流程：读取上月 xlsx → 提取期末 → 生成本月期初表 → 逐笔追加分录 → 月末结账
- main.go 集成 generator 调用，在 CSV 输出之外生成完整 xlsx

## Capabilities

### New Capabilities

- `excel-generation`: 累计 Excel 工作薄生成，包含总分类账 Sheet、多科目明细账 Sheet、期初表 Sheet，支持分页、月结、余额计算、打印标记。多科目明细账中按总账科目汇总其下所有明细科目。

### Modified Capabilities

<!-- No existing specs to modify -->

## Impact

- 新建 `generator/` 包（workbook.go, workbook_test.go）
- 修改 `main.go` 集成 generator 调用
- 依赖 `excelize/v2` 库
- 依赖 `balance` 包的 `LeafSummary`、`ComputeSummariesWithParents`
- 依赖 `voucher` 包的 `Entry`
