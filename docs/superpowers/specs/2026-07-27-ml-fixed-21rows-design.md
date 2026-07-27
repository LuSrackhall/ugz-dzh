## Context

ML 多科目明细账需要每页固定21行结构：20行数据行 + 第21行红字"过次页"。
当前实现中，过次页只在翻页触发时才写入，不满页时不存在。

## Goals

- 每页第21行固定有红字"过次页"（结构模板）
- 不满页时只保留红字，不写翻页数据
- 次月数据以"期末余额"行或最后有实际数据的行为锚点续写
- 结构过次页不参与任何数据流判断

## Decisions

**数据定位**：`mlNextDataRow`/`mlNextDataRowAfterBreak` 改为查找有实际数据的最后行（非空的凭证号、金额、或"期末余额"标签），跳过结构过次页。

**预写**：`writeMLPageHeader` 后立即在 `pageStart+pageSize` 位置写红字"过次页"。翻页触发时 `writeMLPageBreakRow` 填入翻页数据。

**隔离**：`mlIsStructuralBreak` 判断过次页行是否有金额数据。无数据 → 结构模板。所有 break 查找函数均跳过结构过次页。
