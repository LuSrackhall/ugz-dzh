## Why

当前 xlsx 生成中行列坐标通过硬编码常量控制（如 `printGLColDate = 1`），每页布局依靠 Excel 页眉/页脚和打印重复行。这种方式无法满足手工账打印还原度的要求：双下划线、金额分栏、页码动态变化、正/反面并排同 Sheet、过次页/承前页的精确控制。任何列宽/行高调整都需要修改多处写入逻辑，容易破坏布局。

需要建立一套物理驱动、Spec 驱动的布局计算架构，将物理约束（A4 尺寸、装订边、每页行数）与渲染逻辑解耦。

## What Changes

1. **新增 LayoutEngine 包**（`generator/layout/`）—— `LayoutSpec` 定义物理约束，`ComputeLayout(spec)` 计算所有行列坐标
2. **重写总分类账每页写入逻辑**——不再依赖 Excel 打印重复行和页眉/页脚，每次过次页后由代码写入标题行（分第 n 页、总分类账、科目名称）
3. **正/反面同 Sheet 并排**——正面在左、反面在右，选区打印
4. **移除现有硬编码打印常量**——所有坐标通过 `ComputeLayout` 计算
5. **更新 CLAUDE.md**——已添加页面布局宪法章节

## Capabilities

### New Capabilities
- `layout-engine`: Spec 驱动的物理布局计算引擎，将纸张尺寸、装订边、每页行数等物理约束映射为 Excel 行列坐标

### Modified Capabilities
- `excel-generation`: 总分类账 Sheet 的生成方式变更——不再使用打印重复行和页眉/页脚，改为代码逐页写入标题

## Impact

- 新增 `generator/layout/` 包，含 `LayoutSpec`、`ComputeLayout` 等
- 重写 `generator/gl_sheet.go` 中的 `writeGLTitle`、`appendToGLSheet`、页面断行相关逻辑
- 不变：`generate.go` 整体流程、`balance`、`voucher`、`monthly_close.go` 计算逻辑
- 不变：多科目明细账、期初表、期末表等 Sheet
- CLAUDE.md 已更新宪法章节
