package generator

import (
	"github.com/xuri/excelize/v2"
)

// AlreadyGenerated 判断 xlsx 是否已生成过（含"本月合计"行）。
// 用于 generate 幂等保护（审计 M3）：不带 -f 且当月文件存在时，
// 含本月合计 → 已生成，要求 -f；不含（如 year-close 预生成空工作薄）→ 允许继续。
// 打开失败视为未生成（交由 NewWorkbook 在生成时报出）。
func AlreadyGenerated(path string) (bool, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	for _, sheet := range f.GetSheetList() {
		rows, err := f.GetRows(sheet)
		if err != nil {
			continue
		}
		for _, r := range rows {
			for _, c := range r {
				if c == "本月合计" {
					return true, nil
				}
			}
		}
	}
	return false, nil
}
