package generator

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// 科目账页 Sheet 顺序归位（全局设置.科目顺序 接线）。
//
// 目标布局（与账簿装订分区一致，路径无关、结果确定）：
//
//	[当月期初表] [总分类账叶子账页…] [合并总账账页…] [多科目明细账账页…] [日记账/报表/期末表（原相对顺序）]
//
// 区块内按 科目顺序 排序：列出的科目排前（按列表先后），未列出的按 Sheet 名排序
// （同时消除 ML map 遍历创建顺序的随机性，使全量重建结果确定）。
// 匹配规则：科目顺序条目等于账页键（叶子全路径或总账科目名）或其总账科目名即命中；
// 条目指向不存在的账页时无影响。
//
// 实现：以「{月}期末」表为哨兵，按目标序列逐个 MoveSheet 到哨兵之前（前插法，
// 每次插入都紧贴哨兵，序列顺序即最终顺序）。期末表由 WriteFinalSheet 在本函数
// 之前生成，必然存在；异常缺失时回退用最后一张 Sheet 作哨兵。
//
// 注意：页码为每 Sheet 独立编号（"过次页"标签计数），重排不影响页码。
// 全新路径上 excelize 默认 Sheet1 由 Save() 延迟删除，此处不将其纳入目标序列。
// 月度增量生成（复制上月追加）中新账页先落在末尾，本函数在保存前统一归位：
// 调整 科目顺序 后重新 generate 当月即生效；历史月份文件不回改（铁律一）。
func (wb *Workbook) reorderSubjectSheets() {
	if wb.File == nil || wb.Config == nil {
		return
	}
	if debug := os.Getenv("LEDGER_DEBUG_SHEET_ORDER"); debug != "" {
		fmt.Printf("[debug] reorder 前: %v\n", wb.File.GetSheetList())
		defer func() { fmt.Printf("[debug] reorder 后: %v\n", wb.File.GetSheetList()) }()
	}

	order := wb.Config.Settings.Order
	rank := make(map[string]int, len(order))
	for i, o := range order {
		if _, ok := rank[o]; !ok { // 重复条目取首个出现位置
			rank[o] = i
		}
	}
	mergeSet := make(map[string]bool, len(wb.Config.Settings.MergeGLAccounts))
	for _, g := range wb.Config.Settings.MergeGLAccounts {
		mergeSet[g] = true
	}

	const (
		sectionGL    = 0 // 总分类账叶子账页
		sectionMerge = 1 // 合并总账账页（与叶子 GL 同名前缀，按 MergeGLAccounts 区分）
		sectionML    = 2 // 多科目明细账账页
	)
	rankOfKey := func(key string) int {
		if r, ok := rank[key]; ok {
			return r
		}
		gen := key
		if i := strings.IndexByte(key, '-'); i > 0 {
			gen = key[:i]
		}
		if r, ok := rank[gen]; ok {
			return r
		}
		return len(order) // 未列出 → 区块内按名排序
	}

	// 哨兵：{月}期末（WriteFinalSheet 先于本函数执行，必然存在；异常时回退最后一张）
	list := wb.File.GetSheetList()
	if len(list) == 0 {
		return
	}
	sentinel := wb.Month + "期末"
	if idx, err := wb.File.GetSheetIndex(sentinel); err != nil || idx < 0 {
		sentinel = list[len(list)-1]
	}

	type slot struct {
		name    string
		section int
		rankVal int
	}
	var slots []slot
	initials := []string{} // 当月期初表（WriteInitialSheet 已删往月，仅存当月一张）
	target := []string{}

	for _, name := range list {
		switch {
		case name == "Sheet1":
			// 全新路径的 excelize 默认页，Save() 延迟删除；不纳入目标序列
		case strings.HasPrefix(name, sheetPrefixGL):
			key := strings.TrimPrefix(name, sheetPrefixGL)
			sec := sectionGL
			if mergeSet[key] {
				sec = sectionMerge
			}
			slots = append(slots, slot{name: name, section: sec, rankVal: rankOfKey(key)})
		case strings.HasPrefix(name, sheetPrefixML):
			key := strings.TrimPrefix(name, sheetPrefixML)
			slots = append(slots, slot{name: name, section: sectionML, rankVal: rankOfKey(key)})
		case strings.HasSuffix(name, "期初"):
			initials = append(initials, name)
		default:
			// 固定尾部（日记账/报表/期末），保持原相对顺序
			target = append(target, name)
		}
	}

	sort.SliceStable(slots, func(i, j int) bool {
		if slots[i].section != slots[j].section {
			return slots[i].section < slots[j].section
		}
		if slots[i].rankVal != slots[j].rankVal {
			return slots[i].rankVal < slots[j].rankVal
		}
		return slots[i].name < slots[j].name
	})

	full := initials
	for _, s := range slots {
		full = append(full, s.name)
	}
	full = append(full, target...)

	// 前插法：按目标序列逐个移到哨兵之前，序列顺序即最终顺序
	for _, name := range full {
		if name != sentinel {
			_ = wb.File.MoveSheet(name, sentinel)
		}
	}
}
