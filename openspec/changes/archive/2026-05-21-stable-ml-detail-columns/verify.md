# Verification Report

> 此檔案由 `openspec-verify-change` skill 在 apply 完成後產生，用以確認實作
> 與 specs / design / tasks 的一致性。失敗的檢查須返回對應 artifact 修正後
> 再重跑 verify。

**Change**: `stable-ml-detail-columns`
**Verified at**: `2026-05-22 01:12`
**Verifier**: `Claude Code (deepseek-expert)`

---

## 1. Structural Validation (`openspec validate --all --json`)

- [x] 全數 items `"valid": true`

**結果**：

```text
items: 4
passed: 4
failed: 0

cli-commands (spec):        valid ✓
excel-generation (spec):    valid ✓
stable-ml-detail-columns (change): valid ✓
voucher-parsing (spec):     valid ✓
```

無失敗項目。

---

## 2. Task Completion (`tasks.md`)

- [x] 所有 `- [ ]` 已變為 `- [x]`

**統計**: 13 completed / 0 remaining

無未完成任務。

---

## 3. Delta Spec Sync State

對每個 `openspec/changes/<name>/specs/` 下的 capability 目錄，與
`openspec/specs/<capability>/spec.md` 比對：

| Capability | Sync 狀態 | 備註 |
|---|---|---|
| excel-generation | ✗ 待 sync | Base spec exists at `openspec/specs/excel-generation/spec.md`. Delta adds: cumulative stable column strategy, new-detail-append scenario, detail-disappear scenario, monthly-close column alignment scenario. Base lacks `detailOrder` support and `readMLDetailHeaders` requirement. |
| ml-detail-order-config | N/A (new capability) | No base spec exists. Entire spec is new — detailOrder parsing, decoupling from tree, empty-string skip columns, auto-writeback. |
| ml-stable-columns | N/A (new capability) | No base spec exists. Entire spec is new — read-head-preserve-order, new-detail-right-append, 14-column limit, unified mapping source, conflict detection. |

---

## 4. Design / Specs Coherence Spot Check

抽樣比對 `design.md` 的決策是否反映在 `specs/*.md` 的 Requirements 與
Scenarios 中：

| 抽樣項 | design 描述 | specs 對應 | 差距 |
|---|---|---|---|
| 读头保序 | design §2.1: 从第2行读取现有列映射并保持不可变 | ml-stable-columns: Requirement "读头保序" + Scenario "Sheet 已存在，标题匹配" | 無 |
| 新科目右追 | design §2.2: 新科目追加到右侧第一个空列 | ml-stable-columns: Requirement "新科目右追" | 無 |
| detailOrder 配置 | design §3: JSON `detailOrder` 支持空字符串跳列、自动回写 | ml-detail-order-config: 4 Requirements covering parsing, decoupling, skip-columns, writeback | 無 |
| 冲突检测 | design §3.2: 逐列比对标题与配置，不匹配报错 | ml-stable-columns: Requirement "冲突检测" + 2 Scenarios | 無 |
| 统一映射来源 | design §4: AppendMLEntries 和 WriteMLMonthClosings 统一用 readMLDetailHeaders | ml-stable-columns: Requirement "统一映射来源" + 2 Scenarios | 無 |

**漂移警告**（非阻塞）：

- 無

---

## 5. Implementation Signal

- [x] Worktree 內無未 staged 的檔案
- [x] 所有相關 commit 已提交

**Commit 範圍**：`fab8a61..6e39b0a`

```
6e39b0a feat: 多科目明细账列序稳定机制 — 读头保序、新科目右侧追加、detailOrder 配置支持
```

變更文件：
- `balance/balance.go` — 新增 `DetailOrder map[string][]string` 配置字段
- `generator/ml_sheet.go` — readMLDetailHeaders, resolveMLDetailColumns, checkMLDetailOrderConflict, 重构 ensureMLSheet/AppendMLEntries/appendToMLSheet
- `generator/monthly_close_ml.go` — 月结行映射统一从 Sheet 标题读取

---

## 6. Front-Door Routing Leak Detector（warning,非阻塞）

設計產出不應落在 `docs/superpowers/specs/` (brainstorm artifact 的
output redirection 會把它導到 `openspec/changes/<name>/brainstorm.md`)。

偵測:

```bash
ls docs/superpowers/specs/*.md 2>/dev/null
```

- [x] 存在的檔案是 schema 安裝前的合法存留

**洩漏清單**：

| 檔案 | 內容是否已 captured 進 change | 建議動作 |
|---|---|---|
| `docs/superpowers/specs/2026-05-20-ml-sheet-fix-design.md` | 是 — 該設計文檔對應的是前一輪修復（列佈局/分頁/月結），非本次 stable-ml-detail-columns | 保留或手動清理，不阻塞本次 archive |

> 該檔案日期為 2026-05-20，早於 superpowers-bridge schema 安裝，非本次 cycle 的洩漏。

---

## 7. Deferred Manual Dogfood vs Automated Test Equivalence

對 plan.md 中標 `[~]` deferred 的手動 dogfood / smoke task，逐項列出
等價的自動化測試覆蓋。

**結果**: plan.md 中無 `[~]` 標記的 deferred task。本節空白即 PASS。

---

## Overall Decision

- [x] ✅ PASS — 可進入 finishing-a-development-branch 與 archive

**下一步**：

所有檢查通過。可執行 `/opsx:archive` 歸檔此變更，或繼續處理 retrospective artifact。
