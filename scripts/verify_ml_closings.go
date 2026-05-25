package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: go run scripts/verify_ml_closings.go <year_dir>\n")
		os.Exit(1)
	}
	yearDir := os.Args[1]

	months := []string{"2026-01", "2026-02", "2026-03", "2026-04"}
	xlsxPaths := make(map[string]string)
	for _, m := range months {
		p := filepath.Join(yearDir, m+".xlsx")
		if _, err := os.Stat(p); err != nil {
			fmt.Fprintf(os.Stderr, "Missing: %s\n", p)
			os.Exit(1)
		}
		xlsxPaths[m] = p
	}

	failed := false

	wbs := make(map[string]*excelize.File)
	for _, m := range months {
		f, err := excelize.OpenFile(xlsxPaths[m])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open %s: %v\n", m, err)
			os.Exit(1)
		}
		defer f.Close()
		wbs[m] = f
	}

	allMLSheets := make(map[string]bool)
	for _, m := range months {
		for _, name := range wbs[m].GetSheetList() {
			if strings.HasPrefix(name, "多科目明细账-") {
				allMLSheets[name] = true
			}
		}
	}

	yuanToCents := func(s string) int64 {
		s = strings.TrimSpace(s)
		if s == "" || s == "-" {
			return 0
		}
		s = strings.ReplaceAll(s, ",", "")
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0
		}
		return int64(f*100 + 0.5)
	}

	type closingData struct {
		monthDebit   int64
		monthCredit  int64
		ytdDebit     int64
		ytdCredit    int64
		qtDebit      int64
		qtCredit     int64
		detailMonth  map[int]int64
		detailYTD    map[int]int64
		hasEntries   bool // true if this month has actual data entries
	}

	closingMap := make(map[string]map[string]*closingData)
	detailHeaders := make(map[string]map[int]string)

	for sheet := range allMLSheets {
		closingMap[sheet] = make(map[string]*closingData)

		for _, m := range months {
			f := wbs[m]
			if idx, _ := f.GetSheetIndex(sheet); idx == -1 {
				continue
			}
			rows, err := f.GetRows(sheet)
			if err != nil {
				continue
			}

			// Read detail headers from row 2
			if detailHeaders[sheet] == nil && len(rows) >= 2 {
				detailHeaders[sheet] = make(map[int]string)
				for col := 7; col < len(rows[1]); col++ {
					name := strings.TrimSpace(rows[1][col])
					if name != "" {
						detailHeaders[sheet][col] = name
					}
				}
			}

			// Check if any data row has date matching current month
			hasEntries := false
			for i := 2; i < len(rows); i++ {
				r := rows[i]
				if len(r) < 3 {
					continue
				}
				label := strings.TrimSpace(r[2])
				if label == "本月合计" || label == "本季合计" || label == "本年累计" || label == "期末余额" {
					break
				}
				if len(r) > 0 && strings.HasPrefix(strings.TrimSpace(r[0]), m) {
					hasEntries = true
					break
				}
			}

			if !hasEntries {
				continue
			}

			cd := &closingData{
				detailMonth: make(map[int]int64),
				detailYTD:   make(map[int]int64),
				hasEntries:  true,
			}
			for _, r := range rows {
				if len(r) < 3 {
					continue
				}
				label := strings.TrimSpace(r[2])
				switch label {
				case "本月合计":
					if len(r) > 3 {
						cd.monthDebit = yuanToCents(r[3])
					}
					if len(r) > 4 {
						cd.monthCredit = yuanToCents(r[4])
					}
					for col := range detailHeaders[sheet] {
						if col < len(r) {
							cd.detailMonth[col] = yuanToCents(r[col])
						}
					}
				case "本季合计":
					if len(r) > 3 {
						cd.qtDebit = yuanToCents(r[3])
					}
					if len(r) > 4 {
						cd.qtCredit = yuanToCents(r[4])
					}
				case "本年累计":
					if len(r) > 3 {
						cd.ytdDebit = yuanToCents(r[3])
					}
					if len(r) > 4 {
						cd.ytdCredit = yuanToCents(r[4])
					}
					for col := range detailHeaders[sheet] {
						if col < len(r) {
							cd.detailYTD[col] = yuanToCents(r[col])
						}
					}
				}
			}
			closingMap[sheet][m] = cd
		}
	}

	// V1: Month 01 YTD == Month Total
	fmt.Println("=== V1: Month 01 YTD == Month Total ===")
	for sheet, monthData := range closingMap {
		d01, ok := monthData["2026-01"]
		if !ok || !d01.hasEntries {
			continue
		}
		if d01.ytdDebit != d01.monthDebit || d01.ytdCredit != d01.monthCredit {
			fmt.Printf("FAIL %s 2026-01: YTD(D=%d E=%d) != Month(D=%d E=%d)\n",
				sheet, d01.ytdDebit, d01.ytdCredit, d01.monthDebit, d01.monthCredit)
			failed = true
		} else {
			fmt.Printf("PASS %s 2026-01: YTD=MonthTotal (D=%.2f E=%.2f)\n",
				sheet, float64(d01.ytdDebit)/100, float64(d01.ytdCredit)/100)
		}
	}

	// V2: YTD[n] = YTD[n-1] + Month[n] (only if BOTH months have entries)
	fmt.Println("\n=== V2: YTD Incremental ===")
	for sheet, monthData := range closingMap {
		for i := 1; i < len(months); i++ {
			prev := months[i-1]
			curr := months[i]
			dPrev, okPrev := monthData[prev]
			dCurr, okCurr := monthData[curr]
			if !okPrev || !okCurr || !dPrev.hasEntries || !dCurr.hasEntries {
				continue
			}
			expDebit := dPrev.ytdDebit + dCurr.monthDebit
			expCredit := dPrev.ytdCredit + dCurr.monthCredit
			if dCurr.ytdDebit != expDebit || dCurr.ytdCredit != expCredit {
				fmt.Printf("FAIL %s %s->%s: YTD(D=%d E=%d) != prevYTD(D=%d E=%d)+month(D=%d E=%d)=exp(D=%d E=%d)\n",
					sheet, prev, curr, dCurr.ytdDebit, dCurr.ytdCredit,
					dPrev.ytdDebit, dPrev.ytdCredit, dCurr.monthDebit, dCurr.monthCredit,
					expDebit, expCredit)
				failed = true
			} else {
				fmt.Printf("PASS %s %s->%s: YTD(D=%.2f E=%.2f) = prev+month\n",
					sheet, prev, curr,
					float64(dCurr.ytdDebit)/100, float64(dCurr.ytdCredit)/100)
			}
		}
	}

	// V3: Quarter-end 本季合计 (only for sheets active all 3 months)
	fmt.Println("\n=== V3: Q1 本季合计 (2026-03) ===")
	for sheet, monthData := range closingMap {
		d03, ok := monthData["2026-03"]
		if !ok || !d03.hasEntries {
			continue
		}
		var qtDebitSum, qtCreditSum int64
		allPresent := true
		for _, qm := range []string{"2026-01", "2026-02", "2026-03"} {
			d, ok := monthData[qm]
			if !ok || !d.hasEntries {
				allPresent = false
				break
			}
			qtDebitSum += d.monthDebit
			qtCreditSum += d.monthCredit
		}
		if !allPresent {
			continue
		}
		if d03.qtDebit != qtDebitSum || d03.qtCredit != qtCreditSum {
			fmt.Printf("FAIL %s 2026-Q1: QT(D=%d E=%d) != sum(D=%d E=%d)\n",
				sheet, d03.qtDebit, d03.qtCredit, qtDebitSum, qtCreditSum)
			failed = true
		} else {
			fmt.Printf("PASS %s 2026-Q1: QT(D=%.2f E=%.2f) = sum 01+02+03\n",
				sheet, float64(d03.qtDebit)/100, float64(d03.qtCredit)/100)
		}
	}

	// V4: Detail column YTD cumulative (only when current month has entries)
	fmt.Println("\n=== V4: Detail Column Cumulative ===")
	for sheet, monthData := range closingMap {
		headers := detailHeaders[sheet]
		if len(headers) == 0 {
			continue
		}

		cumDetail := make(map[int]int64)
		for _, m := range months {
			cd, ok := monthData[m]
			if !ok || !cd.hasEntries {
				continue
			}
			for col := range headers {
				cumDetail[col] += cd.detailMonth[col]
				expected := cumDetail[col]
				actual := cd.detailYTD[col]
				if actual != expected {
					fmt.Printf("FAIL %s %s detail[%s]: YTD=%.2f != expected %.2f\n",
						sheet, m, headers[col], float64(actual)/100, float64(expected)/100)
					failed = true
				} else {
					fmt.Printf("PASS %s %s detail[%s]: YTD=%.2f\n",
						sheet, m, headers[col], float64(actual)/100)
				}
			}
		}
	}

	// V5: Verify that inactive sheets have YTD unchanged from last active month
	fmt.Println("\n=== V5: Inactive Sheet YTD Preservation ===")
	for sheet, monthData := range closingMap {
		var lastActive *closingData
		var lastActiveMonth string
		for _, m := range months {
			cd, ok := monthData[m]
			if ok && cd.hasEntries {
				lastActive = cd
				lastActiveMonth = m
			}
		}
		if lastActive == nil {
			continue
		}
		// Check that later months without entries have the same YTD as last active
		for _, m := range months {
			if m <= lastActiveMonth {
				continue
			}
			// Read from xlsx directly for inactive months
			f := wbs[m]
			if idx, _ := f.GetSheetIndex(sheet); idx == -1 {
				continue
			}
			rows, err := f.GetRows(sheet)
			if err != nil {
				continue
			}
			// Check if any data row has current-month date
			hasEntries := false
			for i := 2; i < len(rows); i++ {
				r := rows[i]
				if len(r) < 3 {
					continue
				}
				if strings.TrimSpace(r[2]) == "本月合计" {
					break
				}
				if len(r) > 0 && strings.HasPrefix(strings.TrimSpace(r[0]), m) {
					hasEntries = true
					break
				}
			}
			if hasEntries {
				continue // skip, handled by V2/V4
			}
			// Find YTD row in this inactive sheet
			for _, r := range rows {
				if len(r) >= 3 && strings.TrimSpace(r[2]) == "本年累计" {
					ytdD := yuanToCents(r[3])
					ytdC := int64(0)
					if len(r) > 4 {
						ytdC = yuanToCents(r[4])
					}
					if ytdD != lastActive.ytdDebit || ytdC != lastActive.ytdCredit {
						fmt.Printf("FAIL %s %s (inactive): YTD(D=%d E=%d) != lastActive %s YTD(D=%d E=%d)\n",
							sheet, m, ytdD, ytdC, lastActiveMonth, lastActive.ytdDebit, lastActive.ytdCredit)
						failed = true
					} else {
						fmt.Printf("PASS %s %s (inactive): YTD preserved from %s (D=%.2f E=%.2f)\n",
							sheet, m, lastActiveMonth, float64(ytdD)/100, float64(ytdC)/100)
					}
					break
				}
			}
		}
	}

	if failed {
		fmt.Println("\n=== SOME VERIFICATIONS FAILED ===")
		os.Exit(1)
	}
	fmt.Println("\n=== ALL VERIFICATIONS PASSED ===")
}
