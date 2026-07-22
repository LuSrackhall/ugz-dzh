## Why

当前总分类账每页标题区（分第 n 页、总分类账、科目名称、列标题）仅在首次创建 Sheet 时写入。跨月多页后，后续页依赖 Excel 打印重复行显示标题，无法显示逐页变化的页码（所有页均为"分第 1 页"）。需要在过次页后代码写入标题行，并实现页码动态递增。

## What Changes

- 新增 `writePageHeader` 函数，每页过次页后写入标题行（5 行：页码+总分类账+科目名称+空行+列标题）
- 页码从已有过次页行数统计：`pageNum = 过次页数 + 1`
- 更新 `pageStartRow` 偏移量：从 `i + 3` 改为 `i + 1 + TitleRowCount + 1`
- MergeGL 同步受 `pageStartRow` 偏移影响自动适配

## Capabilities

### New Capabilities
- `page-header-rewrite`: 分页标题重写，过次页后代码写入标题行，页码动态递增

### Modified Capabilities
- `excel-generation`: 总分类账分页逻辑——过次页后写入新页标题行替代打印重复行依赖

## Impact

- generator/gl_sheet.go: 新增 writePageHeader；appendToGLSheet 增加页码逻辑
- generator/monthly_close.go: nextDataRowAfterBreak 受 pageStartRow 偏移影响
- generator/merge_gl_sheet.go: 自动适配 pageStartRow 偏移
- generator/print_mark.go: 不变（页码不影响打印标记）
