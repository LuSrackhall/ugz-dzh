# production-gates Spec

## ADDED Requirements

### Requirement: 生成失败原子性
generate 先保存 xlsx 再回写 JSON；xlsx 落盘失败时 JSON 不更新，无"配置已回写、账本未生成"的中间态。

#### Scenario: xlsx 保存失败无中间态
- **WHEN** wb.Save() 失败（磁盘满等）
- **THEN** JSON 未回写当月余额，generate 返回错误

### Requirement: 凭证号未解析阻断
`ValidateVoucherBalance` 存在 VoucherNum<=0 条目时返回 error（不再仅 warning 跳过），generate 拒绝生成。

#### Scenario: 未解析凭证号被拒
- **WHEN** 某分录凭证号未解析（正文与文件名均无法解析）
- **THEN** 校验返回错误"凭证号未解析"，generate 拒绝生成

### Requirement: 跳月检测
非首月生成时，上月账本 xlsx 不存在 → 告警（不阻断）。

#### Scenario: 缺上月账本告警
- **WHEN** 生成 2026-02 且 2026-01.xlsx 不存在
- **THEN** 输出警告"2026-01 账本缺失，疑似跳月/漏月，请确认"
