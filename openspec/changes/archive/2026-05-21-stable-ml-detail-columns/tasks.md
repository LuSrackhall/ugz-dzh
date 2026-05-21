## 1. Core: 读头保序与稳定列机制

- [x] 1.1 新增 `readMLDetailHeaders(sheet) → (detailIdx map[string]int, details []string, err)` — 从第2行 H-U 读取现有标题，构建 detailName→colIndex 映射，返回已分配列位和空列位信息
- [x] 1.2 新增 `resolveMLDetailColumns(existingDetails, newDetails, detailOrder) → (details []string, detailIdx map[string]int, newAppended []string, err)` — 合并已有列映射与新科目，按 detailOrder 优先/字母序追加新科目到空列，超14列返回错误，返回完整列序、映射、新增科目列表
- [x] 1.3 修改 `ensureMLSheet` — 已有 Sheet 时调用 `readMLDetailHeaders` 读取现有映射，再调用 `resolveMLDetailColumns` 合并新科目，更新标题行（新科目追加到空列），返回最终 `detailIdx`
- [x] 1.4 修改 `writeMLTitle` — 全新 Sheet 时若存在 detailOrder 配置，按配置顺序初始化列（含空字符串占位）；否则按字母序（已内联在 ensureMLSheet 中）
- [x] 1.5 删除或重构 `updateMLDetailHeaders` — 不再全量覆写标题，改为仅更新空列的新科目标题（已由 ensureMLSheet 内联处理，updateMLDetailHeaders 保留但不再被 ensureMLSheet 调用）

## 2. 统一映射来源

- [x] 2.1 修改 `AppendMLEntries` — 移除内部排序逻辑（lines 146-150），改为调用 `ensureMLSheet` 返回的 `detailIdx`；`appendToMLSheet` 签名增加 `detailIdx` 参数
- [x] 2.2 修改 `WriteMLMonthClosings` — 移除独立排序逻辑（lines 44-48），改为调用 `readMLDetailHeaders` 获取映射；明细列写入使用此映射

## 3. JSON detailOrder 配置

- [x] 3.1 扩展配置结构体 — 新增 `DetailOrder map[string][]string` 字段（json tag: `detailOrder`），含空字符串支持
- [x] 3.2 冲突检测 — `ensureMLSheet` 中，当 Sheet 已存在且配置了 detailOrder 时，逐列比对第2行非空标题与配置顺序，不匹配则报错提示 `-f`
- [x] 3.3 自动回写 — `AppendMLEntries` 检测到新追加科目后，将其增量追加到 `DetailOrder`，由 generate.go 统一保存

## 4. 验证与测试

- [x] 4.1 运行 `go test ./...` 确保现有测试通过
- [x] 4.2 清理 `test/e2e/out/2026/*.xlsx`，用 `-f` 从 1月重新生成到 4月，验证明细列不窜位
- [x] 4.3 手动检查生成的 xlsx：每月第2行标题一致（列序稳定），数据行金额在正确列下，月结行明细列对齐
