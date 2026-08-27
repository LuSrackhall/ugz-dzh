## Context

投产评估 5 条【必须】门槛，复核确认属实。本 change 一次性补齐（均小改动）。

## Goals / Non-Goals

**Goals:**
- 失败原子性（xlsx 先于 JSON）
- 凭证号未解析阻断
- 跳月告警
- README 备份/恢复章节 + year-close 期初锚定提示

**Non-Goals:**
- 重号/断号检测（复杂度与收益不匹配，靠"一文件一凭证"录入规范）
- 全量重建脚本、check xlsx 漂移比对（可选，投产后再补）

## Decisions

**D1 调序**：generate.go 将 `wb.Save()`（含 finalizeAllGLSheets 页末补齐）移到 `SaveConfig` 之前。Save 失败 → JSON 未写（无中间态）；SaveConfig 失败 → xlsx 已含本月合计，下次无 -f 被幂等拒绝，`-f` 重建自愈。

**D2 未解析阻断**：balance_check.go 的 unparsed warning 升级为 error。文件名回退已兜底（正文无凭证号时用文件名），真正未解析 = 正文无 + 文件名不可解析，极罕见；阻断强制用户修，符合 CLI 安全。e2e 数据凭证号齐全不受影响。

**D3 跳月告警**：GenerateWorkbook 开头（NewWorkbook 后）：`month > StartMonth` 且上月 xlsx 不存在 → 打印告警不阻断（余额链靠 JSON 回退仍连续，只是账页缺月提示）。跨年首月 prev=上年 12 月 xlsx 存在则不告警。

**D4 文档**：README 新增"备份与恢复"（git 提交 JSON + 定期归档 xlsx + 灾难恢复 `-f` 级联重建步骤）。

**D5 A2 提示**：year-close 输出一行：期初锚定建账月提示 + 新年度期初=上年末自动结转。

## Risks / Trade-offs

- **D2 行为变化**：未解析凭证从"跳过"变"拒绝"——更严格，符合 CLI 安全；如需容忍需显式改凭证号。
- **D1 顺序**：SaveConfig 失败场景靠 -f 重建自愈（文档说明）。
