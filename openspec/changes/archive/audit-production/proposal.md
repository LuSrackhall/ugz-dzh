## Why

投产成熟度评估（agent-6ad11b15 / agent-612211a3，第四轮）结论"有条件投产"，复核确认 5 条【必须】门槛属实（小改动）：

1. **失败原子性**：generate 先 SaveConfig（JSON 回写）后 wb.Save()（xlsx）——xlsx 落盘失败时 JSON 已回写，中间态。
2. **凭证号防线**：VoucherNum<=0 仅 warning 且整组跳过平衡校验——未解析凭证可静默入账。
3. **跳月检测**：无上月 xlsx 存在性校验——跳月生成账页缺月无感知。
4. **备份/恢复文档**：README 无备份与灾难恢复章节。
5. **新年度期初路径（A2）**：year-close 后启动月仍旧年，新年度期初调整锚定旧建账月——运行时无提示。

复核不补（记录）：重号检测（复杂度/收益低）、全量重建脚本与 check xlsx 漂移比对（可选，非必须）。

## What Changes

1. **失败原子性**：generate.go 调换顺序——`wb.Save()`（xlsx 落盘）在前，`SaveConfig`（JSON 回写）在后；xlsx 失败则 JSON 未写，无中间态。
2. **凭证号未解析阻断**：`ValidateVoucherBalance` 未解析条目从 warning 升级为 **error**（强制修凭证号；文件名回退已兜底，真正未解析极罕见）。
3. **跳月检测**：`GenerateWorkbook` 非首月且上月 xlsx 不存在 → 告警"上月账本缺失，疑似跳月"。
4. **README 备份与恢复**：新增章节（git 备份 / xlsx 归档 / -f 级联重建步骤）。
5. **A2 提示**：`year-close` 输出期初锚定提示（新年度期初=上年末自动结转；如需调整请 -f 重建）。

## Capabilities

### New Capabilities
- `production-gates`: 投产门槛——失败原子性、凭证号未解析阻断、跳月检测、备份恢复文档、跨年期初提示

## Impact

- `generator/generate.go`（1、3）、`voucher/balance_check.go`（2）、`cmd/year_close.go`（5）、`README.md`（4）
- 测试：未解析阻断单测、跳月告警单测；e2e 回归
