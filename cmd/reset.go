package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xuri/excelize/v2"
)

func init() {
	rootCmd.AddCommand(resetCmd)
	resetCmd.Flags().StringP("month", "m", "", "月份 YYYY-MM（必填）")
	resetCmd.Flags().StringP("output", "o", ".", "输出根目录")
	resetCmd.MarkFlagRequired("month")
}

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "重置打印标记",
	Long:  "清除指定月份 xlsx 中所有账页的\"需打印\"标记。\nxlsx 路径 = {output}/{year}/{month}.xlsx",
	RunE: func(cmd *cobra.Command, args []string) error {
		month, _ := cmd.Flags().GetString("month")
		output, _ := cmd.Flags().GetString("output")

		year := strings.Split(month, "-")[0]
		path := filepath.Join(output, year, month+".xlsx")
		f, err := excelize.OpenFile(path)
		if err != nil {
			return fmt.Errorf("打开 %s: %w", path, err)
		}
		defer f.Close()

		cleared := 0
		for _, name := range f.GetSheetList() {
			rows, err := f.GetRows(name)
			if err != nil {
				continue
			}
			for i, row := range rows {
				if len(row) > 0 && row[0] == "需打印" {
					cell, _ := excelize.CoordinatesToCellName(1, i+1)
					f.SetCellValue(name, cell, "")
					cleared++
				}
			}
		}

		if err := f.SaveAs(path); err != nil {
			return fmt.Errorf("保存 %s: %w", path, err)
		}

		fmt.Printf("已清除 %s 中 %d 处打印标记\n", month, cleared)
		return nil
	},
}
