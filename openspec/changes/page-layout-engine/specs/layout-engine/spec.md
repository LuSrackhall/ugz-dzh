# layout-engine Specification

## Purpose
提供 Spec 驱动的物理布局计算引擎，将纸张尺寸、装订边、每页行数等物理约束映射为 Excel 行列坐标，确保所有渲染代码不依赖硬编码行列号。

## Requirements

### Requirement: LayoutSpec 定义
系统 SHALL 提供 `LayoutSpec` 结构体，包含物理约束参数和列比例定义。

#### Scenario: 默认总分类账配置
- **GIVEN** 新建 LayoutSpec
- **WHEN** 不传任何参数
- **THEN** 默认值应为 A4 横向（297×210mm）、装订边 20mm、页间隙 5mm、每页 20 行数据、12 位金额分栏

#### Scenario: 列比例定义
- **GIVEN** 一个 LayoutSpec
- **WHEN** 修改日期列从 1 列拆为 2 列（月/日）
- **THEN** 日期区总宽度不变，仅内部比例重分

### Requirement: ComputeLayout 计算
系统 SHALL 提供 `ComputeLayout(spec)` 纯函数，接收 LayoutSpec，输出 Layout。

#### Scenario: 列坐标计算
- **GIVEN** A4 横向 / 装订边 20mm / 页间隙 5mm
- **WHEN** ComputeLayout 执行
- **THEN** 正面区域起始 mm = 20，反面区域起始 mm = 正面结束 + 5mm 间隙

#### Scenario: Excel 映射
- **WHEN** ComputeLayout 执行
- **THEN** 输出中应包含 ExcelFrontStartCol（正面第 1 列）、每列的 Excel 列号和列宽（Excel 单位）

### Requirement: 渲染器隔离
系统 SHALL 确保渲染代码（Excel 写入器）只消费 Layout 坐标，不知晓物理 mm 单位和分配逻辑。

#### Scenario: 无硬编码列号
- **WHEN** 渲染 gl_sheet.go
- **THEN** 不允许出现 `printGLColDate = 1` 等硬编码常量，所有列号通过 Layout.Columns 获取

### Requirement: 区域独立性
系统 SHALL 保证任意区域内列/行拆分不影响其他区域坐标。

#### Scenario: 金额位数变化
- **GIVEN** 金额分栏 12 位，每列宽 = W
- **WHEN** 改为 14 位
- **THEN** 每列宽 = W × 12 / 14，金额区总宽不变，左侧日期区/凭证号区坐标不动
