## Context

当前总分类账所有数据（标题+数据行）全部写入 `FrontStartCol` 区域（列 C~J，装订列之后）。反面区域（`BackStartCol` 起的列）已经预留了列宽，但未被写入任何内容。正反面并排的 Layout 已经就绪，只差 Renderer 按 pageNum 奇偶性选择写入区域。

之前 design.md 已确定"反面页二期实现"，现在正式实施。

实施过程中发现并修复了以下关联问题：
- 读取端辅助函数未感知偶数页（Back 区），过次页标记读不到 → 余额链断裂风险
- `-f` 级联删除逻辑 `>= month` 误删当年文件 → year-close 输出丢失
- year_close 复制旧年度明细数据到新年度文件
- 上年结转定位错误（标题区 Row 3 vs 内容区首行）
- 上年结转每月重复生成（未限制仅 1 月）

## Goals / Non-Goals

**Goals:**
- 奇数页数据写入 FrontStartCol 区域（不变）
- 偶数页数据写入 BackStartCol 区域（新增）
- 偶数页标题写入 BackStartCol 区
- 页码连续：正反交替（1=正, 2=反, 3=正, 4=反…）
- 读取端感知两区域（`hasPageBreakAt` 同时检查 Front/Back）
- `-f` 级联改为 `> month`，保留当月文件（如 year-close 输出）
- year_close 创建空白新文件（不复制旧数据）
- 上年结转仅 1 月写入内容区首行
- 每页严格 20 行数据 + 过次页
- 完整 e2e 验证（含跨年）

**Non-Goals:**
- 不改 ML 正反面（change 4）
- 不改 LayoutSpec

## Decisions

### 决策 1：写入区域选择

`appendToGLSheet` 和 `writePageHeader` 中，根据 pageNum 奇偶选择写入起始列：`dataCol` 函数。

### 决策 2：读取端两区域感知

新增 `hasPageBreakAt` 同时检查 Front 和 Back 两区的过次页标记。所有 6 个读取端辅助函数（`lastPageBalance`、`lastRowIsOrphanBreak`、`lastBreakTotals`、`pageStartRow`、`pageHasBreakRow`、`nextDataRow`）+ `ExtractLastMonthFinals` + `lastBreakDetailTotals` + `markExistingPageForPrint` 同步适配。

### 决策 3：pageStartRow 偏移

过次页后页：`i + 2 + DataStartRow`（承前页为数据行 1）。首页：`DataStartRow + 1`。`rowIsPageBreak` 使用 `>= pageSize` 精确触发。

### 决策 4：上年结转仅在 1 月写入

`!isNew && initial != 0 && strings.HasSuffix(wb.Month, "-01")`。通过 `insertCarryForwardAtRow` 在数据区首行写入。

### 决策 5：year_close 创建空白文件

不复制旧数据，由 generate 自动创建 GL Sheet 和上年结转。

### 决策 6：`-f` 级联保护

`>= month` → `> month`，避免误删当月 year-close 预生成文件。`NewWorkbook` 支持加载当月已存在文件（如 year-close 输出）。

### 决策 7：`GetInitBalanceForGenerate` 支持历史余额

从 JSON 科目树取最近月份的非零期末余额作为期初，不依赖当月 xlsx 的期末行。

## Risks / Trade-offs

| 风险 | 缓解 |
|---|---|
| 偶数页过次页读取端未同步适配 | 宪法合规审计发现，`hasPageBreakAt` 已修复 6+ 函数 |
| year-close 与 generate 流程脱节 | year-close 只建空白文件，上年结转由 generate 自动处理 |
| 回退到旧版需要重新 year-close | year-close 为轻量操作（仅创建空白文件 + JSON 配置） |
