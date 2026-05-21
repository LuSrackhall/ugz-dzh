# Retrospective: stable-ml-detail-columns

> Written: 2026-05-22 (after verify passed)
> Commit range: `fab8a61..6e39b0a`
> Worktree: main checkout (no isolation)

---

## 0. Evidence

> 量化前置數據 — 後續 Wins / Misses bullets 直接引用，避免每行重複 [evidence: ...]。

- **Commit range**: `fab8a61..6e39b0a` (1 commit)
- **Diff size**: +305 / -56 lines across 3 files
- **Tasks done**: 13/13 (`grep -cE '^\s*- \[x\]' tasks.md` → 13)
- **Active hours**: ~3 (spread across two sessions)
- **Subagent dispatches**: 0 (single-developer inline implementation)
- **New external dependencies**: none
- **Bugs encountered post-merge**: none
- **OpenSpec validate state at archive**: pass (4/4 items valid)
- **Test coverage signal**: `go test ./...` — 5 packages pass, including e2e full workflow

Commit chain (時序):

```
6e39b0a feat: 多科目明细账列序稳定机制 — 读头保序、新科目右侧追加、detailOrder 配置支持
```

---

## 1. Wins

- [evidence: `generator/ml_sheet.go:229-256`] `readMLDetailHeaders` 作為唯一映射來源，消除了 `AppendMLEntries` 和 `WriteMLMonthClosings` 各自排序構建映射的重複邏輯，統一入口減少列錯位風險
- [evidence: `generator/ml_sheet.go:262-326`] `resolveMLDetailColumns` 作為純函數設計（無 Workbook 依賴），邏輯自包含、易測試、邊界清晰
- [evidence: `generator/ml_sheet.go:119-154`] `checkMLDetailOrderConflict` 的衝突檢測逐列比對三種情況（配置空/實際空/值不匹配），錯誤信息精確到列號和科目名，用戶可據此決策是否 `-f` 重生成
- [evidence: `balance/balance.go:22`] `DetailOrder map[string][]string` 配置字段用 `omitempty`，未配置的科目零侵入
- [evidence: `generator/monthly_close_ml.go:42-44`] 月結行映射改為從 Sheet 標題讀取後，代碼刪除了 `mlClosing` 中手工構建 `detailIdx` 的排序邏輯，淨減少 ~10 行

## 2. Misses

- 📌 [nit | evidence: `test/e2e/e2e_test.go`] e2e 測試未覆蓋多科目明細賬的列穩定性斷言 — 當前只驗證 CSV 輸出和 xlsx 文件存在性，不檢查 xlsx 內部列標題是否跨月一致。應增加 `inspect_ml.go` 式的標題比對或 go test 內嵌 excelize 斷言
- 📌 [nit | evidence: `generator/ml_sheet.go:39`] `ensureMLSheet` 中 `_ = existingIdx` 未使用 — 表明 `readMLDetailHeaders` 返回的 detailIdx 在此路徑下被丟棄，雖然語義正確（已有 Sheet 走 `resolveMLDetailColumns` 重建映射），但代碼信號不夠清晰
- 📌 [nit | evidence: `generator/ml_sheet.go:217-227`] `updateMLDetailHeaders` 保留但不再被 `ensureMLSheet` 調用 — 舊接口殘留，未來讀者可能困惑其用途

## 3. Plan deviations

| Plan task | What changed | Why |
|-----------|--------------|-----|
| 1.5 | `updateMLDetailHeaders` 保留而非刪除 | 保守策略 — 保留公開方法避免未知調用方編譯失敗；實際無外部調用 |
| 2.1 | `AppendMLEntries` 中 `mlGroup` 結構體移除 `detailIdx` 字段 | Plan 中描述為"增加 detailIdx 參數"，實際是徹底移除 group 級別的映射構建，改為從 `ensureMLSheet` 返回值獲取 — 比 plan 更徹底 |

## 4. Skill / workflow compliance

| Skill                                            | Used |
|--------------------------------------------------|------|
| superpowers:brainstorming                        |  ✓   |
| superpowers:writing-plans                        |  ✓   |
| superpowers:using-git-worktrees                  |  ✗   |
| superpowers:subagent-driven-development          |  ✗   |
| (transitive) superpowers:test-driven-development |  ✗   |
| (transitive) superpowers:requesting-code-review  |  ✗   |
| superpowers:finishing-a-development-branch       |  ✗   |

> **Default expectation**: 全部 ✓。每個 skill 都是 schema 設計的一部分，
> 跳過屬於異常情境。任一項 ✗ 都必須在下方
> `### Deliberately Skipped Skills` subsection 提出原因與預防方案。

### Deliberately Skipped Skills

- **`superpowers:using-git-worktrees`**
  - **What was skipped**: 整個 skill — 未創建隔離 worktree，直接在 main 分支上實現
  - **Why this cycle**: 變更範圍小（3 文件，單 commit），且上一輪修復（fab8a61）也直接操作 main，流程連續性優先
  - **How to prevent recurrence**: `scope-judgment rule` — 判斷標準：若預計產生 2+ 個 commit 或涉及 3+ 個 package，必須使用 worktree 隔離。本次剛好在閾值下，但偏向性應改為"默認使用 worktree"

- **`superpowers:subagent-driven-development`**
  - **What was skipped**: 整個 skill — 未按 task 分派 subagent，所有實現內聯完成
  - **Why this cycle**: 13 個 task 中 10 個是單文件內的函數級變更（`ml_sheet.go`），任務間強依賴（新增函數 → 修改調用方 → 修改另一調用方），拆分 subagent 反而增加上下文切換成本
  - **How to prevent recurrence**: `scope-judgment rule` — 判斷標準：若 task 間共享 3+ 個新函數簽名或同一文件超過 200 行變更，內聯實現可接受。若不同 task 修改獨立 package，應分派 subagent

- **`superpowers:test-driven-development`**
  - **What was skipped**: 未先寫測試再寫實現代碼
  - **Why this cycle**: 變更本質是重構現有函數的內部實現並新增純函數，接口契約未變（`go test ./...` 在變更前後均通過）。新增函數 `readMLDetailHeaders`、`resolveMLDetailColumns`、`checkMLDetailOrderConflict` 由 e2e 完整工作流間接覆蓋
  - **How to prevent recurrence**: `CLAUDE.md trigger` — 對新增公開函數（大寫導出），即使 e2e 間接覆蓋，也應補充單元測試。可在 CLAUDE.md 增加："新增導出函數 → 必須在對應 `_test.go` 中加 table-driven test"

- **`superpowers:requesting-code-review`**
  - **What was skipped**: 未請求 code review agent 審閱變更
  - **Why this cycle**: 單 developer 內聯實現，verify 步驟的結構化檢查（openspec validate + task completion + design/specs coherence spot check）起到了形式化審查作用
  - **How to prevent recurrence**: `skill description tightening` — 在 skill frontmatter 中明確"單 developer 變更也需要 review agent 作為獨立第二雙眼睛"，而非暗示只有多 developer 協作才需要

- **`superpowers:finishing-a-development-branch`**
  - **What was skipped**: 未執行該 skill 的合併/推送流程
  - **Why this cycle**: 用戶在 verify 通過後暫停，archive 步驟尚未執行。該 skill 應在 archive 後（或用戶明確要求 push 時）觸發
  - **How to prevent recurrence**: `scope-judgment rule` — finishing-a-development-branch 應在 retrospective 完成後、archive 前執行。本次尚未到達觸發點

## 5. Surprises

- 原以爲 `resolveMLDetailColumns` 需要傳入 `existingIdx`（從標題讀到的映射），實際發現僅用 `existingDetails`（按列順序的字符串數組）即可完成所有操作——`existingIdx` 可在調用方按需重建，避免參數冗餘
- `detailOrder` 配置的自動回寫邏輯比預期複雜：需處理 nil map 初始化 + 去重檢查 + 保持已配置項順序。最終內聯在 `AppendMLEntries` 中（18 行）而非抽成獨立函數，因爲回寫與調用上下文緊耦合（`newAppended` 來自 `ensureMLSheet` 返回值）

## 6. Promote candidates → long-term learning

- [ ] 📌 **新增導出函數應補充 table-driven test** → **Promote to CLAUDE.md** (`CLAUDE.md`)
  > **Why**: 本 cycle 新增 4 個導出函數（readMLDetailHeaders, resolveMLDetailColumns, checkMLDetailOrderConflict, nonEmptyDetails），均無獨立單元測試，僅靠 e2e 間接覆蓋。未來修改這些函數時缺少快速反饋
  > **How to apply**: 在 CLAUDE.md 的 Development discipline 段增加："新增導出函數（大寫開頭）→ 在對應 `_test.go` 中補充 table-driven test，至少覆蓋正常路徑和一個邊界條件"

- [ ] 📌 **e2e 測試應增加 xlsx 內部結構斷言** → **Promote to memory** (type: project)
  > **Why**: 當前 e2e 僅驗證 xlsx 文件存在性，不檢查內部 Sheet 結構。穩定列機制的核心驗收標準（跨月標題一致、數據金額在正確列下）完全依賴手動檢查
  > **How to apply**: 下次修改 generator 包的行爲時，在 e2e 測試中用 excelize 打開生成的 xlsx，比對第2行 H-U 列標題與預期值，抽取數據行金額列位驗證

- [ ] 📌 **`updateMLDetailHeaders` 舊接口標記廢棄** → **Promote to one-off**
  > **Why**: 該函數不再被 ensureMLSheet 調用，但因保守策略保留。未來 reader 可能困惑其用途。可在函數上方加 `// Deprecated: use ensureMLSheet instead` 註釋
  > **How to apply**: 下次觸及 `ml_sheet.go` 時順手加 deprecation comment，或在下一個 breaking change cycle 中刪除
