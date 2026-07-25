# 手工账电子化生成系统

## 开发规范

实现任何核心模块前，先阅读 `openspec/project.md` 确认方案与总体规划一致。如有不匹配，暂停并提问，不要静默偏离。

## 提交规范

所有 git commit message 使用中文：
```
feat(simulation): 新增朝向系统组件
fix(render): 修复盾牌血条显示问题
docs: 更新设计文档
```

历史变更记录存放在 `openspec/changes/archive/` 和 `openspec/specs/` 中。

---

# 一、数据架构宪法（Data Constitution）

> 本宪法的优先级高于布局宪法。布局服务于数据安全，而不是反过来。

## 1.1 三铁律

**铁律一：历史文件永不修改。**
每月生成时完整复制上月 xlsx 再追加当月数据。一旦保存，已有月份的 xlsx 不得被任何操作覆盖或修改（-f 只能从当月开始级联重建）。

**铁律二：余额链必须连续。**
过次页 → 承前页 → 逐行余额累算 → 期末余额，这条链是核心骨架。破坏连续性等价于数据丢失。

**铁律三：JSON 是余额的唯一权威源。**
xlsx 是生成品，期初调整的唯一入口是 JSON 中的`期初调整额`字段。xlsx 中的期初/期末表可被脚本自动重建，不以 xlsx 为基准。

## 1.2 JSON 配置决定生成行为

JSON `{year}/{year}.json` 中的以下字段直接影响代码路径：

| 配置字段 | 影响函数 | 后果 |
|---|---|---|
| `总分类账忽略科目` | `AppendEntries` → `glSuppress` | 跳过叶子 GL Sheet |
| `多科目明细账忽略科目` | `AppendMLEntries` → `mlSuppress` | 跳过 ML Sheet |
| `合并总账科目` | `AppendMergeEntries` → `mergeSet` | 为父级生成合并 GL |
| `自动识别科目.期初调整额` | `GetInitBalanceForGenerate` | 影响所有期初余额 |
| `手动调整科目.期初调整额` | `GetInitBalanceForGenerate` | 同上 |
| `科目树.余额` 历史 | `ExtractYtdTotals` / `ExtractQuarterlyTotals` | 影响本年累计/本季累计 |
| `明细列顺序` | `ensureMLSheet` → `detailOrder` | 多科目明细账列序 |

三个生成入口（`AppendEntries`、`AppendMergeEntries`、`AppendMLEntries`）共享同一个 `initials` 期初映射，修改生成逻辑时必须三路同步验证。

## 1.3 GetRows 索引契约

**关键约束**：`excelize.GetRows()` 返回 `[][]string`，cell 通过 slice index 访问（不是 Excel 列字母）。引入装订列（`BindingLeftCols`）后，所有依赖固定 index 的读取代码必须同步偏移。

读取索引定义（以 `BindingLeftCols=2, FrontStartCol=3` 为例）：

| 数据 | GetRows index | 含义 |
|---|---|---|
| `row[BindingLeftCols+0]` | `row[2]` | 日期 |
| `row[BindingLeftCols+1]` | `row[3]` | 凭证号 |
| `row[BindingLeftCols+2]` | `row[4]` | 摘要（含"过次页"/"承前页"标记） |
| `row[BindingLeftCols+3]` | `row[5]` | 借方金额 |
| `row[BindingLeftCols+4]` | `row[6]` | 贷方金额 |
| `row[BindingLeftCols+5]` | `row[7]` | 方向 |
| `row[BindingLeftCols+6]` | `row[8]` | 余额 |

所有读取"过次页/承前页"标记（`row[i][2]` → `row[i][BindingLeftCols+2]`）和提取期末余额（`row[i][6]` → `row[i][BindingLeftCols+6]`）的辅助函数都必须通过 `layout.BindingLeftCols` 计算偏移，**不得硬编码**。

涉及以下函数：`lastPageBalance`、`lastRowIsOrphanBreak`、`lastBreakTotals`、`pageStartRow`、`rowIsPageBreak`、`pageHasBreakRow`、`ExtractLastMonthFinals`、`lastBreakDetailTotals`、`nextDataRow`、`nextDataRowAfterBreak`。

**写入端**同理：所有 `cellName(N, row)` 必须替换为 `cellName(lay.FrontStartCol + offset, row)`。

## 1.4 year_close.go 特殊约束

`year_close.go` 直接操作 xlsx 文件（不是通过 Workbook 方法），写 `"上年结转"` 到 A1、余额到 G1。Layout 变更后这些坐标需修正为 Layout 坐标。

## 1.5 modify 修改工作流

任何修改生成逻辑的代码，必须按以下步骤验证：

1. **单元测试全绿**：`go test ./...`
2. **e2e 测试通过**：`bash scripts/test-e2e.sh`
3. **三个生成路径都验证**：GL + MergeGL + ML 各自生成数据不丢失、余额链正确
4. **跨月追加验证**：执行 `bash scripts/test-e2e.sh --skip-test` 生成多月份数据，确认跨月数据连续

---

# 二、页面布局宪法（Layout Constitution）

> 所有账页模板布局的最高设计原则，AI Agent 在处理任何布局需求时必须遵守。

## 2.0 核心概念：两层解耦

本宪法严格区分两个独立层次，**宪法只管辖第 2 层**。第 1 层是编排机制，不是宪法规范。

| 层次 | 名称 | 职责 | 管辖 | 示例 |
|---|---|---|---|---|
| 第 1 层 | **Sheet 编排层** | 将第 2 层定义的一页作为整体，在 Sheet 中安排其位置（并排/上下、装订边、列间隙），方便数据查看和打印 | LayoutEngine | "GL 每页占 8 列，正面在前 8 列，反面在后 8 列" |
| 第 2 层 | **正/反页内部层** | 每张 A4 纸正页或反页内部的完整结构（标题区、表头区、数据区、页脚区），含**过次页/承前页数据流控制**和**每页固定 20 行数据 + 1 行过次页** | **本宪法** | "标题深绿 #006100、14pt 加粗；每页固定 20 行数据后过次页，新页首行承前页" |

**宪法约束第 2 层**：一张 A4 纸的正页和反页，其内部各区域有一个统一的样式标准。不管是第几页、是过次页后的新页还是承前页后的续页，**所有物理页的同一区域遵循同一个标准**。

第 1 层（Sheet 编排层）是可替换的编排机制，不受宪法约束。只要第 2 层标准不变，第 1 层可以自由演进（例如从上下排改为左右并排）。

### 两层协作关系

**过次页/承前页是第 2 层内部的数据流控制**——每页固定 20 行数据行（含分录行、上年结转行、承前页行，全部计入），满 20 行后第 21 行写"过次页"；新页首行承前页承上页余额。第 1 层对此不感知也不参与。

第 1 层的职责很简单：拿到第 2 层定义的一页占多少行多宽，把它当黑盒在 Sheet 中摆好位置，纯粹为了方便查看和打印。

```
第 2 层定义：页内结构（标题、表头、数据、过次页/承前页、列宽比例、样式）
     │
     ▼ 一页的宽高尺寸
第 1 层编排：在 Sheet 中安排正/反页位置（并排/上下、装订边、列间隙）
     │
     ▼ 提供 Layout
Renderer 由第 2 层驱动渲染页内内容，按第 1 层的 Layout 确定写入坐标
```

第 1 层负责"页放在 Sheet 的哪里"，第 2 层负责"页里面长什么样、数据怎么流"。互不依赖，通过 Layout 协作。

## 2.1 最高原则

- **正/反页内部区域优先，不是单元格优先**。修改前先问"这是哪个区域的样式变化"。
- **所有物理页的同一区域共享一个样式标准**。不需要为「第 N 页」或「过次页后」单独设计不同的标题样式。
- **坐标是结果，不是设计输入**。所有行列号/合并区/宽高必须来源于 `ComputeLayout()`，不得人工推导。
- **布局服务于模板设计迭代**，不是运行时引擎。模板最终固定为稳定坐标。

## 2.2 布局模型

```
LayoutSpec → ComputeLayout(spec) → Layout → Renderer
```

- **LayoutSpec** — 物理约束。`每页行数` 属于第 2 层参数。
- **ComputeLayout** — 纯函数，唯一允许计算布局的地方。相同输入 = 相同输出。
- **Layout** — 所有最终坐标的快照。Renderer 只消费，不修改。
- **Renderer** — 将 Layout 输出为 Excel。不参与设计、计算、修正。

### 模型在第 2 层的含义

第 2 层中，LayoutSpec 描述的是**正页或反页内部**各区域的物理约束：
- `每页行数`（DataRowsPerPage）— 一张物理页数据区固定容纳的行数。**20 行数据行 + 1 行过次页 = 21 行/页**。数据行包括分录行、上年结转行、承前页行，全部计入
- 标题区高度、标题字体/字号/颜色
- 表头区列宽比例、列名文字样式
- 数据区行高、金额格式、字体

ComputeLayout 为第 2 层计算出页内坐标，Renderer 按坐标渲染，不自行推导样式。

## 2.3 第 2 层：正/反页内部区域设计

一张 A4 纸的正页或反页，内部按以下区域划分，**所有物理页共享同一标准**，不因翻页次数、过次页/承前页状态而变：

| 区域 | 内容 | 一致性要求 |
|---|---|---|
| 标题区（Header） | 账簿名称、页码、科目名称、合计行标签 | 正页/反页同标准 |
| 表头区（TableHeader） | 列名行（日期、凭证号、摘要、借方、贷方、方向、余额） | 所有页同列宽比例 |
| 数据区（Body） | 分录行、上年结转行、承前页行、过次页行 | 所有页同结构：20 行数据行 + 1 行过次页 = 21 行/页。数据行包括分录、上年结转、承前页，全部计入 20 行内。过次页/承前页是第 2 层内部数据流控制 |
| 页脚区（Footer） | 过次页合计行、承前页承接行（属于数据区首/末行） | 同数据行标准 |

- 区域拥有稳定边界（由 LayoutSpec 定义），区域之间互不依赖。每页固定 21 行（20 数据 + 1 过次页），不因内容多少而变化。
- 区域内部允许自由演进（拆列/合列/调比例），但**区域整体尺寸不变**。
- 区域内比例重分配不得影响其他区域。

## 2.4 AI Agent 行为步骤

```
Step 1: 分析需求 → 判断属于第 1 层（编排方式）还是第 2 层（页内样式）。
Step 2: 第 1 层变更 → 调整 Sheet 编排参数（并排/上下、装订边、列间隙），仅影响正/反页在 Sheet 中的摆放位置。
Step 3: 第 2 层变更 → 调整 LayoutSpec 中页内样式参数（`每页行数`、标题字体/颜色、列宽比例、行高）。
Step 4: 重新 ComputeLayout()，检查影响范围 → 仅影响目标层则继续，否则重调。
Step 5: 修改 Renderer 消费新 Layout。
```

**禁止直接改 Renderer。** 优先调整 LayoutSpec。

## 2.5 禁止事项

- 人工维护大量坐标 / Renderer 自行计算或修正布局
- 一个区域依赖另一个区域的内部结构 / 因局部需求修改大量无关区域
- 未修改 LayoutSpec 就直接改坐标 / 先改 Excel 再反推布局
- **为特定翻页场景编写不同的标题样式代码** — 所有物理页的同一区域共享一个标准

## 2.6 设计哲学

```
Sheet 编排（第 1 层）→ 页内样式标准（第 2 层）→ 坐标 → 渲染
```
固定的是第 2 层标准。可演进的是第 1 层编排方式（并排/上下/单页）和第 2 层的具体样式参数。
