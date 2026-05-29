# 合并总分类账模式 设计文档

> 状态: 已确认 | 日期: 2026-05-30

## 背景

现有总分类账按叶子科目（如 `固定资产-电脑`）各生成独立 Sheet。对于固定资产等物品级明细量大的科目，子科目各占一页不够紧凑。

## 目标

为指定父级科目生成一个"合并总分类账" Sheet，将其下所有子科目的分录按发生时间序归入同一帐页。**纯增量功能，不影响原有逻辑。**

---

## 配置

在 `科目余额总览.json` 的 `全局设置` 中新增三个可选字段，均为 `string[]`，默认空数组：

```json
{
  "全局设置": {
    "合并总账科目": ["固定资产"],
    "总分类账忽略科目": ["内部往来"],
    "多科目明细账忽略科目": ["固定资产"]
  }
}
```

| 字段 | 作用 | 缺省 |
|------|------|------|
| `合并总账科目` | 为父级科目生成合并 GL Sheet | `[]` |
| `总分类账忽略科目` | 抑制父级下子科目的叶子 GL Sheet | `[]` |
| `多科目明细账忽略科目` | 抑制父级下子科目的 ML Sheet | `[]` |

**三个字段完全解耦**，可任意组合。

### 典型场景

| 场景 | 配置 |
|------|------|
| 只出合并 GL，其它不要 | 三项全配 `["固定资产"]` |
| 要合并 GL + 保留叶子 GL | `合并总账科目: ["固定资产"]` |
| ML 科目不需要叶子 GL | `总分类账忽略科目: ["应收款"]` |
| GL 科目不需要 ML | `多科目明细账忽略科目: ["固定资产"]` |

---

## 数据结构变更

### `balance/balance.go` — `GlobalSettings`

```go
type GlobalSettings struct {
    StartMonth            string            `json:"启动月"`
    Order                 []string          `json:"科目顺序"`
    AccountMap            map[string]string `json:"科目映射表"`
    MergeGLAccounts       []string          `json:"合并总账科目,omitempty"`
    GLSuppressAccounts    []string          `json:"总分类账忽略科目,omitempty"`
    MLSuppressAccounts    []string          `json:"多科目明细账忽略科目,omitempty"`
}
```

---

## 文件变更清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `balance/balance.go` | 修改 | `GlobalSettings` 加三个字段 |
| `generator/merge_gl_sheet.go` | **新增** | 合并 GL 生成逻辑（~150 行） |
| `generator/generate.go` | 修改 | 流程中插入合并 GL 调用 + 过滤逻辑 |

其余文件**零改动**。

---

## 生成流程

```
① WriteInitialSheet           → 不变
② AppendEntries               → 过滤：跳过 GL忽略科目 的子科目
②.5 AppendMergeEntries        → 新增：为 合并总账科目 生成合并 GL
③ AppendMLEntries             → 过滤：跳过 ML忽略科目 的子科目
④ WriteMonthClosings          → 不变（被忽略的 sheet 不存在，自然跳过）
⑤ WriteMLMonthClosings        → 不变（同上）
⑥ WriteFinalSheet             → 不变
```

---

## 合并 GL Sheet 规范

### Sheet 命名

`总分类账-<父级科目>`，与叶子 GL `总分类账-<父级科目>-<子科目>` 天然不冲突。

### 列结构

沿用 GL 七列：`日期 | 凭证号 | 摘要 | 借方金额 | 贷方金额 | 方向 | 余额`

### 摘要格式

摘要列填入 `[子科目名] 原摘要`，无明细科目时分录则不加前缀。

### 余额计算

方向/余额按父级科目汇总累计：每行余额 = 上一行余额 + 本行借方 - 本行贷方。跨子科目连续计算。

### 上年结转

若父级科目在期初初始化中有余额（各子科目期初之和），首行写入"上年结转"行，余额为子科目期初合计。

### 分页

复用现有 `pageSize=20` 逻辑，到页容量时插入"过次页"/"承前页"行。

### 金额格式

所有金额列（4, 5, 7）调用 `setMoneyStyle`，应用 `#,##0.00` 格式。

### 月末关账

数据追加完毕后，按现有规则写入：
- **本月合计**：汇总当月所有子科目借/贷合计
- **本季合计**（仅季末 3/6/9/12）
- **本年累计**
- **期末余额**：期初 + 本月借 - 本月贷

方向/余额的期末计算与现有 `WriteMonthClosings` 一致。

---

## 过滤规则

### `AppendEntries` 过滤

在分组前，检查分录的 `GeneralAccount` 是否在 `GLSuppressAccounts` 中：若在，则**该分录不参与叶子 GL 生成**。

### `AppendMLEntries` 过滤

在分组前，检查分录的 `GeneralAccount` 是否在 `MLSuppressAccounts` 中：若在，则**该分录不参与 ML 生成**。

### 月结适配

`CollectChangedSheets` 需要感知过滤，被抑制的 sheet 不应出现在 changedSheets 中。或者不改 CollectChangedSheets，而是在 WriteMonthClosings / WriteMLMonthClosings 中，对不存在的 sheet 静默跳过（现有逻辑已通过 `nextDataRowAfterBreak` 返回空行处理）。

---

## 测试策略

1. **单元测试**：新增 `generator/merge_gl_sheet_test.go`
   - 基础：单父级、单子科目、单分录
   - 多子科目：验证 `[子科目] 摘要` 格式
   - 余额累计：跨子科目验证方向/余额正确
   - 分页：验证过次页/承前页
   - 月结：验证关账行
   - 金额格式：验证 `setMoneyStyle` 调用

2. **过滤测试**：扩展现有测试
   - 配置 GL忽略 → 对应叶子 GL sheet 不存在
   - 配置 ML忽略 → 对应 ML sheet 不存在

3. **回归测试**：运行 `go test ./generator/...` 和 `go test ./...`，确保 14/14 现有测试不受影响

---

## 边界情况

| 情况 | 处理 |
|------|------|
| 合并科目无分录 | 跳过，不生成空白 sheet |
| 合并科目分录无明细科目 | 摘要不加 `[子科目]` 前缀 |
| 仅配置忽略但未配置合并 | 原科目消失，无替代 → 合法，用户自行承担 |
| 合并科目同时也是叶子科目（有直接分录） | 直接分录摘要不加前缀，正常参与余额累计 |
| 上年无数据（首月） | 不写"上年结转"行 |
