## Why

用户要求把挂起事项全部完成。本批（Change 11）为报表与账簿补全：

1. **资产负债表 / 收支结余表**（设计专家【必须】）：村集体经济组织法定报表，当前缺失。
2. **科目汇总表**：会计软件标配（本月各科目借/贷发生额试算汇总）。
3. **凭证序时簿**：当月凭证按日期/凭证号列表（审计追溯）。
4. **结账标记**：已结账月份默认拒绝再次生成（-f 除外），防误改。
5. **未知科目默认"未分类"**（设计专家建议）：防误归类（现默认按"借"处理）。
6. **add-manual 生效月去误导**：提示"调整额只作用于建账月，-m 仅记录"。

## What Changes

1. **资产负债表**：`资产负债表` sheet——资产类科目期末（借方侧）+ 负债/权益类（贷方侧），资产合计 = 负债+权益合计（结转后平衡），差额提示。
2. **收支结余表**：`收支结余表` sheet——收入类合计 - 费用类合计 = 本年收益（当期/累计）。
3. **科目汇总表**：`科目汇总表` sheet——本月各科目借方/贷方发生额，合计借贷平衡。
4. **凭证序时簿**：`凭证序时簿` sheet——日期/凭证字号/摘要/借方合计/贷方合计（按序时）。
5. **结账标记**：JSON `全局设置.结账月`（如 "2025-12"）；generate 对 `month <= 结账月` 且无 -f 时拒绝（提示 -f）。
6. **未知科目未分类**：`inferPropertyByType` 未知类别返回 "未分类"（原"借"）。
7. **add-manual 提示**：输出"期初调整锚定建账月 {StartMonth}，-m 仅记录不参与计算"。

## Capabilities

### New Capabilities
- `financial-statements`: 资产负债表/收支结余表（法定报表）
- `voucher-summary-register`: 科目汇总表/凭证序时簿
- `closing-lock`: 结账月标记（防误改）
- `unknown-account-unclassified`: 未知科目未分类

## Impact

- `generator/`（balance_sheet.go/income_stmt.go/summary_sheet.go/voucher_register.go 新、generate.go 调用）
- `balance/balance.go`（inferPropertyByType、结账月配置）、`cmd/generate.go`（结账校验）、`cmd/add_manual.go`（提示）
- 测试：报表平衡、科目汇总借贷平衡、结账标记拒绝、e2e 回归
