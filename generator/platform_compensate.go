// Package generator — 打印版尺寸的平台补偿。
//
// 背景：WPS Mac / Windows 渲染列宽、行高的实现不同，同一打印版 xlsx 在
// Windows 上渲染整体偏小（2026-08-28 实测：表格宽 -12.5%、高 -10.8%、
// 数据行行高 -6%）。
//
// 设计：generate 支持 --platform 指定打印版目标平台（auto=当前系统/mac/windows），
// 在 Mac 上也能生成 Windows 版打印版进行验证（平台无关的文件列宽值补偿）。
//
// ⚠️ 历史教训：v0.5.0 曾设 Windows 补偿 = 列宽 ×1.143、行高 ×1.04（目标"拉向 Mac
// 尺寸"），实测导致页面整体溢出：Mac 设计表格 22.28×16.42cm 本身已占满页面可用
// 空间（可用 22.27×16.41cm），任何放大都会撑爆页面。Windows 端应"适配 Windows
// 页面"重新标定，而非拉向 Mac 尺寸。当前系数保持 1.0（不溢出、可用），待标定。
package generator

import "runtime"

// PrintPlatform 打印版目标平台（auto=当前系统；mac/windows=指定平台）。
// 由 cmd 层在 TransformToPrint 前设置。
var PrintPlatform = "auto"

// platformCompensate 返回打印版尺寸的平台补偿系数（列宽、行高）。
// 当前 Windows 系数为 1.0（回退，不溢出），待 Windows 端重新标定后填入适配值。
func platformCompensate() (colScale, rowScale float64) {
	plat := PrintPlatform
	if plat == "" || plat == "auto" {
		plat = runtime.GOOS
	}
	if plat == "windows" {
		// Windows 端标定（2026-08-28 用户肉眼观察迭代）：列宽 ×1.1075、行高 ×0.992
		return 1.1075, 0.992
	}
	return 1.0, 1.0
}
