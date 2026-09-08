package generator

import (
	"fmt"
	"strings"

	"ledger/voucher"
)

// 分离明细账页（全局设置.分离明细账科目）。
//
// 账页样式与多科目明细账同族（ML 那张多栏式账页纸：四行表头、明细列区域、
// 左右双面装订），是 ML"默认合并、分离需配置"的分离形态——对仗"合并总账
// 科目"之于总分类账。配置按总账科目名（其下所有明细各自分离成页）或叶子
// 全路径（单明细分离）；单明细立页无下级明细，明细列自然为空。
// 分离页与合并页互不冲突共存（对仗 GL 叶子页与合并页共存）：合并 ML 的
// 明细列照常保留。与 GL 叶子页互不影响（加法路由，不提示不联动）；JSON
// 余额链/报表/回写零改动——拆的是页不是账。

// ensureMLStyleDetailSheet 确保独立明细账页存在（ML 样式）。
// 新建时写 Paper1 Front 占位页（页码 0，空明细列），与 ensureMLSheet 同构。
func (wb *Workbook) ensureMLStyleDetailSheet(account string) (string, error) {
	name := sheetNameDetail(account)
	if idx, err := wb.File.GetSheetIndex(name); err == nil && idx >= 0 {
		return name, nil
	}

	if _, err := wb.File.NewSheet(name); err != nil {
		return "", fmt.Errorf("创建 Sheet %s: %w", name, err)
	}

	// Paper1 Front 占位页（空占位表，页码=0，不写 Back 侧）——与 ensureMLSheet 一致
	if err := wb.writeMLPageHeader(name, 1, 0, 0, account, false, true); err != nil {
		return "", err
	}
	// 占位表为空白模板：不写明细科目名（14 列全空——单明细科目页无下级明细），
	// 也不写 Back 侧结构过次页占位；首数据页页头由 appendToMLSheetBody 写入。
	return name, nil
}

// appendToMLDetailSheet 追加分录/期初行到独立明细账页（ML 样式，明细列全空）。
func (wb *Workbook) appendToMLDetailSheet(account string, entries []voucher.Entry, initial int64) error {
	sheet, err := wb.ensureMLStyleDetailSheet(account)
	if err != nil {
		return err
	}
	// 单明细科目页：明细列全空（detailIdx 为空 map）
	return wb.appendToMLSheetBody(sheet, account, entries, map[string]int{}, initial)
}

// AppendDetailLedgerEntries 将命中 分离明细账科目 的分录追加到独立账页（生成步骤 7.1）。
// 加法路由：GL 叶子页与合并 ML 完全照常（明细列保留，互不冲突共存），独立页是额外视图。
// 仅期初非零且无当月分录的命中科目也建页写期初行（仅 1 月跨年延续或期初调整额
// 生效时写行）；期初=0 且无分录不建页。
func (wb *Workbook) AppendDetailLedgerEntries(entries []voucher.Entry, initials map[string]int64) error {
	if len(wb.Config.Settings.DetailSplit) == 0 {
		return nil
	}

	// 命中分离配置的分录按叶子全路径分组（条目=总账名或叶子全路径，见 detailSplit）
	groups := make(map[string][]voucher.Entry)
	hasEntries := make(map[string]bool)
	for _, e := range entries {
		path := e.GeneralAccount
		if e.DetailAccount != "" {
			path += "-" + e.DetailAccount
		}
		hasEntries[path] = true
		if wb.detailSplit(path) {
			groups[path] = append(groups[path], e)
		}
	}

	for path, es := range groups {
		if err := wb.appendToMLDetailSheet(path, es, initials[path]); err != nil {
			return fmt.Errorf("分离明细账 %s: %w", path, err)
		}
	}

	// 仅期初非零但无当月分录的命中科目：建页写期初行（1 月跨年延续 或 调整额生效）；
	// 无月结路径（changedSheets 无键）→ 手动补当前页结构过次页
	for path, initial := range initials {
		if hasEntries[path] || initial == 0 || !wb.detailSplit(path) {
			continue
		}
		if !strings.HasSuffix(wb.Month, "-01") && !wb.InitialAdjust[path] {
			continue
		}
		if err := wb.appendToMLDetailSheet(path, nil, initial); err != nil {
			return fmt.Errorf("分离明细账 %s: %w", path, err)
		}
		wb.padMLPage(sheetNameDetail(path), "")
	}
	return nil
}
