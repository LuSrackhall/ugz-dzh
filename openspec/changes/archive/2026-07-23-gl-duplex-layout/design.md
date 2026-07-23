## Context

brainstorm-spec.md 覆盖了高层设计。本文补充实施中遇到的关联问题修复。

## Decisions

### 决策 1：dataCol 与 colOffset

`dataCol(lay, pageNum, offset)` 处理奇偶页列偏移。`writePageHeader` 用 `colOffset = BackStartCol - FrontStartCol` 偏移偶数页标题。

### 决策 2：hasPageBreakAt 双区扫描

读取端不能假设过次页在 Front 区。`hasPageBreakAt` 同时检查 `BindingLeftCols+2`（Front）和 `BackStartCol+1`（Back）。

### 决策 3：year_close 简化

不再复制 12 月 xlsx 数据到新年度。只创建空白文件 + JSON 配置。上年结转由 generate 自动处理（`insertCarryForwardAtRow`），仅在 1 月触发。

### 决策 4：-f 级联保护

`>= month` 改为 `> month`，保留 year-close 输出。重新生成时需要先 `rm` 目标月文件或重新 year-close。
