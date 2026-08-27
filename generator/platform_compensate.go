// Package generator — 打印版尺寸的平台补偿。
//
// 背景：WPS Mac / Windows 渲染列宽、行高的实现不同，同一打印版 xlsx 在
// Windows 上渲染整体偏小（2026-08-28 实测：表格宽 -12.5%、高 -10.8%、
// 数据行行高 -6%）。
//
// ⚠️ v0.5.0 曾设 Windows 补偿 = 列宽 ×1.143、行高 ×1.04（目标"拉向 Mac 尺寸"），
// 实测导致**页面整体溢出**：Mac 设计表格 22.28×16.42cm 本身已占满页面可用空间
// （可用 22.27×16.41cm），任何放大都会撑爆页面。
//
// 结论：Windows 端不能按"= Mac 尺寸"补偿，必须**适配 Windows 页面**重新标定
// 布局参数（表格适配 Windows 可用区域、保持宽高比 1.357、装订边设计）。
// 在重新标定完成前，补偿系数置 1.0（无补偿 = 不溢出、可用），待标定后填入。
package generator

import "runtime"

// platformCompensate 返回打印版尺寸的平台补偿系数（列宽、行高）。
// 当前 Windows 系数为 1.0（回退），待 Windows 端重新标定后填入适配值。
func platformCompensate() (colScale, rowScale float64) {
	if runtime.GOOS == "windows" {
		// TODO: Windows 端重新适配标定（适配 Windows 页面，非 Mac 尺寸）
		return 1.0, 1.0
	}
	return 1.0, 1.0
}
