# 打印版位格输出（print-digit-columns）

## Why

真实手工账使用人民币位格（小格分位）书写金额。系统当前只输出查看版 xlsx（金额为 `#,##0.00` 单元格），无法直接打印装订成符合手工账习惯的账本。用户要求在查看版输出完全不变的前提下，新增打印版输出到 `{output}/{year}/print/{year}-MM.xlsx`。

## What Changes

- 新增**打印版转换器**：`GenerateWorkbook` 落盘查看版后，读取该文件副本，将 GL/MergeGL/ML 三类账页的金额栏位格化后另存到 `{output}/{year}/print/{year}-MM.xlsx`
- **金额列拆 12 小列**（GL 借/贷/余额；ML 借/贷/余额/明细1-14）：每个金额列右侧插入 11 列、总宽守恒，数字按人民币位拆分填入（十亿…元角分），个位对齐「元」，高位留空，0 留空
- 金额恒按非负处理：余额正负由「借或贷」方向列表达；ML 明细净额为负取绝对值
- **小列表头**「十 亿 千 百 十 万 千 百 十 元 角 分」写入现有表头区空位行（GL SubHeaderRow 金额区 / ML h4 行），不新增行；大标题跨 12 小列合并
- **分组边框**：组界竖线（十亿|亿、百万|十万、千|百）绿色加粗；元|角红色单细线；其余小列间普通绿细线
- 结构修复：插列后手动重建垂直分页符（GL +33 列 / ML +77 列）；列宽 ÷12；ML 标题区合并按新坐标重设
- `cmd/generate.go` 调用点接入；打印版生成失败仅告警不中断主流程
- `-f` 级联删除同步清理 `print/` 子目录中晚于当月的文件（补现有缺口）
- 查看版代码路径零改动，查看版 xlsx 输出不受影响

## Capabilities

### New Capabilities
- `print-digit-output`: 打印版位格 xlsx 输出——转换触发时机与失败策略、Sheet 识别范围（总分类账-/多科目明细账- 前缀）、金额拆位规则、表头与小列表头布局、分组边框规范、结构副作用修复（分页符/列宽/合并区）、print/ 子目录级联清理

### Modified Capabilities
<!-- 查看版生成行为不变；cli-commands 仅新增调用点与级联清理，属实现细节，需求层面由 print-digit-output 统一覆盖 -->

## Impact

- **新增代码**：`generator/print_convert.go`（入口 ConvertToPrint + splitCNY + 边框常量表）、`generator/print_gl.go`、`generator/print_ml.go`
- **修改代码**：`cmd/generate.go`（少量行：调用点 + print/ 级联清理）
- **依赖**：excelize v2.10.1（已确认 InsertCols 平移合并单元格但不平移分页符——需手动重建）
- **风险**：InsertCols 在复杂 Sheet 上的行为需 e2e 抽查兜底；÷12 后 Excel 列宽单位非严格线性存在亚毫米渲染漂移；小格字号（6~7pt）需实机打印校准
