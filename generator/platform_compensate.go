// Package generator — 打印版尺寸的平台补偿。
//
// 背景：WPS Mac / Windows 渲染列宽、行高的实现不同，同一打印版 xlsx 在
// Windows 上渲染整体偏小（2026-08-28 实测：表格宽 -12.5%、高 -10.8%、
// 数据行行高 -6%）。为满足"每端各自正确"，打印版变换层按运行平台
// 施加补偿系数（乘在列宽/行高值上），Mac 端系数 1 保持现状不变。
//
// 初值按 Windows 导出 PDF 实测标定（列宽 ×1.143、行高 ×1.04），
// 需在 Windows 上迭代验证收敛。
package generator

import "runtime"

// platformCompensate 返回打印版尺寸的平台补偿系数（列宽、行高）。
func platformCompensate() (colScale, rowScale float64) {
	if runtime.GOOS == "windows" {
		// Windows WPS 渲染偏小，放大补偿；初值待迭代收敛
		return 1.143, 1.04
	}
	return 1.0, 1.0
}
