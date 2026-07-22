## Context

当前总分类账的分页运行正常（过次页/承前页/余额链正确），但有两个问题：
- `writeGLTitle` 写死"分第 1 页"，跨月多页后所有页都显示第 1 页
- 第一页以外没有代码写入的标题行，依赖 Excel 打印重复行

change 1 已完成 Layout 坐标统一，分页的数据逻辑正常。change 2 专注补全：后续页标题和页码。

## Goals / Non-Goals

**Goals:**
- 新增 `writePageHeader` 函数，过次页后写入新页标题（分第 n 页、总分类账、科目名称、列标题）
- 实现页码动态递增：从已有过次页行数推导 pageNum
- 同步更新 `pageStartRow` 偏移量适应标题行占用
- MergeGL 页码同步（ML 延期）

**Non-Goals:**
- 不改变增量追加数据模型
- 不改变过次页/承前页的余额计算逻辑
- 不改 ML 分页（ML 无页码需求）
- 不改变 LayoutSpec 定义

## Decisions

### 决策 1：writePageHeader 实现

复用 `writeGLTitle` 中的样式代码，提取为 `writePageHeader(sheet, pageNum, account)` 函数：

```
Row N+0: 分第 n 页（右侧，绿色+数字红色）
Row N+1: 总    分    类    账（居中，绿色双下划线）| 科目名称（右侧）
Row N+2: 科目名称（右侧，印章红）
Row N+3: [空行]
Row N+4: 列标题（日期│凭证号│摘要│借方金额│贷方金额│方向│余额│金额分栏）
```

调用时机：`appendToGLSheet` 中，过次页行 + 承前页行写入后，nextDataRow 返回新行号时调用。

### 决策 2：`pageStartRow` 偏移量更新

当前：`return i + 3`（过次页行号 + 标题行数）。
更新后：`return i + 1 + lay.TitleRowCount + 1`（承前页 1 行 + 标题 TitleRowCount 行 + 列标题 1 行）。

`pageStartRow` 的返回值决定 `rowIsPageBreak` 的阈值，直接影响分页判断。

### 决策 3：页码计数

在 `appendToGLSheet` 开头遍历 GetRows 统计已有过次页行数：`pageNum = count + 1`。
每触发一次过次页：`pageNum++`。

### 决策 4：MergeGL 同步

MergeGL 的 `appendToMergeGLSheet` 使用与 GL 相同的分页辅助函数（`rowIsPageBreak`、`writePageBreakRow`、`writeCarryForwardRow`），因此 `pageStartRow` 的偏移更新会自动影响 MergeGL。MergeGL 不需要额外新增 `writePageHeader`（页码为非必须）。

## Risks / Trade-offs

| 风险 | 缓解 |
|---|---|
| `pageStartRow` 偏移量改错影响分页判断 | 编译+测试全绿验证；新增多页测试用例 |
| 标题行写入位置错误与数据重叠 | writePageHeader 调用于承前页之后下一行，通过 nextDataRow 获取行号 |
| 跨月追加时页码正确 | 从 GetRows 已有过次页行数统计，天然正确 |
