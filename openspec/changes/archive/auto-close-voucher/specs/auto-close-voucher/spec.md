# auto-close-voucher Spec

## ADDED Requirements

### Requirement: gen-close 生成结转凭证
`ledger gen-close -j {year}.json -o <output>`：对仍有余额的损益类（收入/费用）科目生成年末结转凭证，写入 `<output>/{year}/closing/记字第X号 年末损益结转.md`；已结转科目（closing/ 已有凭证覆盖的科目）跳过；编号自动递增不覆盖已有文件。

#### Scenario: 生成结转凭证
- **WHEN** 管理费用期末 127.15（借余）、其他收入期末 11350（借余）、补助收入期末 -65500（贷余），执行 gen-close
- **THEN** closing/ 生成一张凭证：借 本年收益 127.15/贷 管理费用-办公费 127.15；借 本年收益 11350/贷 其他收入 11350；借 补助收入 65500/贷 本年收益 65500；借贷平衡

### Requirement: generate 自动并入 closing/
generate 自动扫描 `<output>/{year}/closing/*.md` 并入凭证解析，输出"已并入 N 张自动结转凭证"。

#### Scenario: 结转凭证并入生成
- **WHEN** closing/ 有 1 张结转凭证，generate 12 月
- **THEN** 生成提示"已并入 1 张自动结转凭证"，12 月账本中收入/费用余额归零
