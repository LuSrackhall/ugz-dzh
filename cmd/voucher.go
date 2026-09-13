package cmd

import "github.com/spf13/cobra"

func init() {
	rootCmd.AddCommand(voucherCmd)
}

// voucherCmd 凭证录入与检查子命令组。
//
// 定位：给系统补上"创建凭证"的能力——此前 CLI 只读凭证（凭证由人或 OCR 产生）。
// 方案甲：凭证 md 本身就是草稿态，改错 = 改/删 md 再重生成；本命令**不触发 generate**，
// 账本生成始终是用户的显式动作。
var voucherCmd = &cobra.Command{
	Use:   "voucher",
	Short: "凭证录入与检查（创建凭证 md / 列示 / 序列检查）",
	Long: `凭证录入与检查。

  voucher add    创建一张凭证 md（借贷平衡与科目定义闸门在内，不平不落盘）
  voucher list   列示某月凭证清单
  voucher check  检查凭证序列完整性与月目录卫生（缺号/重号/非正式文件名）

纪律（方案甲）：
  - 本命令不生成账本；生成需另行显式执行 ledger generate。
  - 写入目标目录由凭证日期决定（<输出根>/vouchers/YYYY_MM/）。
  - 月目录内只应存在正式凭证 记字第XXXX号.md——任何多余 .md 都会被递归解析计入账本。`,
}
