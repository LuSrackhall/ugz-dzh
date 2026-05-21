## Context

多科目明细账（Multi-ledger Detailed Accounts）每月以累积方式写入 xlsx：`NewWorkbook` 复制上月工作簿后，当月 `AppendMLEntries` 追加数据行，`WriteMLMonthClosings` 追加月结行。每月分组时按字母序重建 `detailIdx`（明细科目→列偏移），并调用 `updateMLDetailHeaders` 更新第2行标题。但历史数据行从未被重写——当月标题变动后，历史行的值停留在原列位，与新标题脱节。

约束：
- 固定 14 列（H-U），mlDetailStartCol=8，mlMaxDetails=14
- xlsx 文件跨月累积，不可回退重写历史月份
- 使用 excelize v2 库操作

## Goals / Non-Goals

**Goals:**
- 明细科目列位一旦分配即不可变，新增科目仅追加到右侧空列
- 支持用户通过 `detailOrder` JSON 配置强制列顺序和跳列
- `detailOrder` 与科目树解耦：预配置科目即使无发生额也展示在标题中
- 新科目自动追加到 `detailOrder` 配置
- 现有 xlsx 与配置冲突时明确报错
- `detailIdx` 统一从 Sheet 标题读取，不再各自独立构建

**Non-Goals:**
- 不重写历史数据行
- 不回收消失科目的列位
- 不支持超过 14 列的 Sheet（物理纸张限制）
- 不修改 GL（总账/明细账）的列布局

## Decisions

### D1: 读头保序策略
从 Sheet 第2行 H-U 读取现有标题，构建 `detailName → colIndex` 映射。传入的 `details` 集合与此映射求差集得到新科目，追加到右侧第一个空列（H-U 范围内标题为空的列）。

**依据**：历史数据行保持原列位不变，唯一真相来源是第2行标题，避免内存中维护额外状态。

### D2: 新科目追加顺序
若 `detailOrder` 配置存在 → 新科目按配置中未占列的项依次填入（包括空字符串占位列），剩余按字母序；若不存在 → 纯字母序。回写时增量追加到配置末尾。

**依据**：兼容无配置场景，同时满足用户完全控制的需求。

### D3: 冲突检测时机
`ensureMLSheet` 发现 Sheet 存在时，读取第2行标题与 `detailOrder` 配置逐列比对。不匹配 → `return error`，提示 `-f` 重新生成。

**依据**：早期失败优于静默错列。

### D4: 统一 detailIdx 来源
新增 `readMLDetailHeaders(sheet) → (detailIdx map, details []string)` 函数。`AppendMLEntries` 和 `WriteMLMonthClosings` 均调用此函数获取列映射，替换各自的独立排序逻辑。

**依据**：消除重复逻辑，保证全链路一致。

### D5: 配置回写方式
JSON 反序列化 → 修改内存结构 → 序列化写回。增量追加，不重排已有项。

**依据**：用户用 git 管理配置文件，回写错误可 revert。

## Risks / Trade-offs

| Risk | Mitigation |
|---|---|
| 列上限 14 更易触发（预分配+跳列占用） | 清晰报错，列出已占列和可用列数，由用户调整配置 |
| 自动回写 JSON 可能产生用户不期望的变更 | 用户用 git 管理，回写后 `git diff` 即可审查 |
| `detailOrder` 中科目名拼写错误导致数据填入错误列 | 程序无法校验语义，属于用户配置责任；冲突检测能捕获部分顺序问题 |
| 旧 xlsx（改动前生成）标题与配置不一致 | `-f` 从首月全部重新生成 |
