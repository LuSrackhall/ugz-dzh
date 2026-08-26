# Verification Report

**Change**: `audit-initial-balance`
**Verified at**: `2026-08-26 18:40`

---

## 1. Structural Validation

- [x] All items `"valid": true`

## 2. Task Completion

- [x] All `- [ ]` changed to `- [x]`（tasks.md 全勾选）

## 3. Delta Spec Sync State

| Capability | Status | Notes |
|---|---|---|
| initial-balance | synced | 新 capability，spec 覆盖 8 项 Requirement |
| cli-commands | N/A | check 增强为增量行为，无 spec 变更 |
| excel-generation | N/A | 期初行摘要为展示层变更 |

## 4. Design / Specs Coherence

| Item | design/specs description | specs requirement | Drift |
|---|---|---|---|
| 调整额优先级 | D1：生效月/首次月==当前月直取，其余续链 | Requirement 1/2 | 无 |
| 跨年不覆盖 | D2：2026-01 期初=2025-12 期末 | Requirement 2 Scenario | 无 |
| 幻影迁移 | D4：PurgePhantomInitials 幂等 | Requirement 4/5 | 无 |
| 摘要规则 | D6：adjustment→期初余额，1月延续→上年结转 | Requirement 6/7 | 无 |
| 平衡校验 | D7：check 报错 / generate 告警 | Requirement 8 | 无 |
| -f 重建基线 | D8：删除当月及以后 + 跨年不复制 | Requirement 9 | 无 |

## 5. Implementation Signal

- [x] No unstaged files
- [x] All commits committed

**Commit range**: `<to-be-filled>`

---

## 实测验证记录（2026-08-26）

1. **幻影期初清理**：e2e 后 JSON 中银行存款 Balances 仅 `2025-10/11/12`（无 2025-01..09 幻影），FirstRecord.Amount=0，全部自动识别科目首次记录金额为 0 ✓
2. **add-manual 生效**：`add-manual 银行存款-工商银行 2025-10 100000` 后 -f 重建，期初表显示该科目期初 100000（借）✓
3. **-f 重建不污染**（D8 修复验证）：-f 重建 2025-10 后银行存款期初=0（修复前会被旧版当月期末污染为 -118119.59）✓
4. **期初行摘要**：银行存款-工商银行（建账）→ "期初余额"；2026-01 银行存款（跨年延续）→ "上年结转" ✓
5. **check 期初平衡**：干净 e2e 数据 check 报"期初借贷不平衡（2025-12 快照）差额 -15760.35 元"——经 python 手工核对，2025-12 快照 Initial 之和 = -1576035 分，**报错正确反映测试数据真实不平衡**（该数据存在借贷不平衡凭证，印证 H2 必要性）✓
6. `go test ./...` 全绿；`bash scripts/test-e2e.sh --skip-test` 全流程生成成功 ✓

---

## Overall Decision

- [x] ✅ PASS
- [ ] ⚠️ PASS WITH WARNINGS: `<note>`
- [ ] ❌ FAIL: `<reason>`
