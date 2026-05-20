# 多科目明细账修复设计

## 范围

修复 `generator/ml_sheet.go` 中多科目明细账的列布局、分页、月结、打印标记等问题，不修改总分类账逻辑。

## 背景

当前多科目明细账存在以下缺陷：
- 打印标记占 H 列，与第一个明细科目列冲突
- 无左右页拆分（应左页4列 + 右页10列）
- 过次页/承前页只写 A-G 列，明细列全留空
- 缺月结行（本月合计/本季合计/本年累计/期末余额）
- 缺期初结转行
- 无限明细科目数限制

## 设计决策

### 列布局（固定）

```
A     日期         基础列
B     凭证号       基础列
C     摘要         基础列
D     借方金额     基础列（总计）
E     贷方金额     基础列（总计）
F     方向         基础列
G     余额         基础列（总计）
H-K   明细科目1-4  左页（前一页背面）
L-U   明细科目5-14 右页（第二页正面）
V     "是否需打印"  右页外侧，隐藏标记列
```

- 列号常量：`mlDetailStartCol = 8` (H列)，`mlPrintMarkCol` = 8 + numDetails + 1
- 明细科目固定上限 14，超限阻塞报错

### 过次页/承前页

A-G 列同总分类账；H-U 各列填本页该明细科目的累计发生额（借-贷净额）。

### 月结行

新增 `WriteMLMonthClosings`，对每个多科目明细账 Sheet 追加：
- **本月合计** — A-G 总计（借/贷总额）+ H-U 各明细当月发生额
- **本季合计**（仅季末 3/6/9/12）— 同上，本季累计
- **本年累计** — 同上，本年累计
- **期末余额** — G 列总余额，H-U **留空**（明细列余额无会计意义）

月结行明细列数据来源：从当月分录按明细科目汇总。

### 打印标记

`markRowForPrint` 改为写入 V 列（`mlPrintMarkCol`），不占用明细列。

### 期初结转

新出现且期初 ≠ 0 的科目，首行插入"上年结转"行（与总分类账一致）。

### 明细科目数量检查

在 `AppendMLEntries` 中检查：任一总账的明细科目数 > 14 则报错，阻塞生成。

## 实现清单

1. 添加常量：`mlMaxDetails = 14`、`mlDetailStartCol = 8`、`mlLeftPageCols = 4`
2. 添加打印标记列计算函数
3. 修改 `writeMLTitle` — 正确写入 H-U 列标题
4. 修改 `writeMLPageBreakRow` — 明细列填本页累计
5. 修改 `writeMLCarryForwardRow` — 明细列填上页累计
6. 修改 `writeMLParentSummary` — 明细列填净额
7. 修改 `appendToMLSheet` — 每行分录明细列写入正确列号
8. 新增 `WriteMLMonthClosings` — 月结行
9. 新增 `writeMLYearCarryForward` — 期初结转行
10. 修改 `markRowForPrint` — 支持多科目明细账的 V 列
11. 新增明细科目数检查 — > 14 报错
12. 修改 `generate.go` — 调用 `WriteMLMonthClosings`
13. 更新测试
