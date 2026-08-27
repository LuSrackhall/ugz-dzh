// Package generator — 打印版尺寸的平台补偿。
//
// 背景：WPS 各平台/各机器渲染列宽、行高的实现不同（字体环境差异），同一打印版
// xlsx 在不同机器上渲染尺寸可能不一致。为满足"每端各自正确"，打印版变换层按
// 目标平台施加补偿系数（乘在列宽/行高值上），系数可在 print-config.json 配置。
//
// 目标平台：PrintPlatform（auto=当前系统；mac/windows=指定），由 cmd 层设置，
// 支持在 Mac 上生成 Windows 版打印版（配合 scripts/gen-win-test.sh 迭代标定）。
package generator

// PrintPlatform 打印版目标平台（auto=当前系统；mac/windows=指定平台）。
// 由 cmd 层在 TransformToPrint 前设置。
var PrintPlatform = "auto"

// platformCompensate 返回打印版尺寸的平台补偿系数（列宽、行高）。
// 系数来源：print-config.json（platforms.<当前平台>），缺省=默认标定值。
func platformCompensate() (colScale, rowScale float64) {
	cfg := platformConfig()
	return cfg.ColScale, cfg.RowScale
}
