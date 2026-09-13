package voucher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// touchFiles 在 dir 下创建指定的（可含子目录的）文件。
func touchFiles(t *testing.T, dir string, names ...string) {
	t.Helper()
	for _, n := range names {
		p := filepath.Join(dir, filepath.FromSlash(n))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func mustCheck(t *testing.T, dir string) *SequenceReport {
	t.Helper()
	r, err := CheckSequence(dir)
	if err != nil {
		t.Fatalf("CheckSequence: %v", err)
	}
	return r
}

func TestCheckSequenceContinuousPasses(t *testing.T) {
	dir := t.TempDir()
	touchFiles(t, dir,
		"记字第0001号.md", "记字第0002号.md", "记字第0003号.md",
		"读我.txt", "print-config.json", // 非 .md 不参与
	)
	r := mustCheck(t, dir)
	if !r.OK() {
		t.Fatalf("应为通过，问题：%+v", r.Issues)
	}
	if r.Count != 3 || r.MaxNum != 3 {
		t.Errorf("Count=%d MaxNum=%d, want 3/3", r.Count, r.MaxNum)
	}
	if !strings.Contains(r.Summary(), "序列检查通过") {
		t.Errorf("摘要应说明通过：%s", r.Summary())
	}
}

func TestCheckSequenceEmptyDirPasses(t *testing.T) {
	dir := t.TempDir()
	r := mustCheck(t, dir)
	if !r.OK() {
		t.Fatalf("空目录应通过，问题：%+v", r.Issues)
	}
	if r.Count != 0 || r.MaxNum != 0 {
		t.Errorf("Count=%d MaxNum=%d, want 0/0", r.Count, r.MaxNum)
	}
}

func TestCheckSequenceDetectsMissing(t *testing.T) {
	dir := t.TempDir()
	touchFiles(t, dir, "记字第0001号.md", "记字第0002号.md", "记字第0004号.md", "记字第0005号.md")
	r := mustCheck(t, dir)
	if r.OK() {
		t.Fatal("应检出缺号")
	}
	if len(r.Issues) != 1 || r.Issues[0].Kind != IssueMissingNum {
		t.Fatalf("应只有一条缺号问题，得到 %+v", r.Issues)
	}
	if !strings.Contains(r.Issues[0].Message, "3") {
		t.Errorf("缺号信息应含 3：%s", r.Issues[0].Message)
	}
}

func TestCheckSequenceDetectsDuplicate(t *testing.T) {
	dir := t.TempDir()
	touchFiles(t, dir, "记字第0001号.md", "记字第0002号.md")
	// 同号不同名（历史遗留写法）——白名单会先判为非正式，故这里用子目录放同号文件
	touchFiles(t, dir, "sub/记字第0002号.md")
	r := mustCheck(t, dir)
	if r.OK() {
		t.Fatal("应检出问题")
	}
	kinds := map[SequenceIssueKind]int{}
	for _, is := range r.Issues {
		kinds[is.Kind]++
	}
	// 子目录里的同号文件同时是"非正式位置"，且根层 0002 只有一份 → 不算重号
	if kinds[IssueNonStandard] != 1 {
		t.Fatalf("子目录文件应判为非正式，问题：%+v", r.Issues)
	}

	// 真正的重号场景：两个文件都被白名单接受不可能同名，故用不同前导零写法（0002 与 02）
	dir2 := t.TempDir()
	touchFiles(t, dir2, "记字第0002号.md", "记字第02号.md")
	r2 := mustCheck(t, dir2)
	hasDup := false
	for _, is := range r2.Issues {
		if is.Kind == IssueDuplicate && is.VoucherNum == 2 {
			hasDup = true
		}
	}
	if !hasDup {
		t.Fatalf("应检出凭证号 2 重复，问题：%+v", r2.Issues)
	}
}

func TestCheckSequenceDetectsNonStandardNames(t *testing.T) {
	dir := t.TempDir()
	touchFiles(t, dir,
		"记字第0001号.md",
		"草稿.md",
		"模板.md",
		"2026-03-05.md",
		"记字第0002号 副本.md",
		"记字第0002号 更正.md",
	)
	r := mustCheck(t, dir)
	if r.OK() {
		t.Fatal("应检出非正式文件名")
	}
	var nonStd []string
	for _, is := range r.Issues {
		if is.Kind == IssueNonStandard {
			nonStd = append(nonStd, is.Files[0])
		}
	}
	for _, want := range []string{"2026-03-05.md", "草稿.md", "模板.md", "记字第0002号 副本.md", "记字第0002号 更正.md"} {
		found := false
		for _, got := range nonStd {
			if got == want {
				found = true
			}
		}
		if !found {
			t.Errorf("应列出非正式文件 %q，实际：%v", want, nonStd)
		}
	}
	// 正式文件仍应被计入
	if r.Count != 1 || r.MaxNum != 1 {
		t.Errorf("Count=%d MaxNum=%d, want 1/1", r.Count, r.MaxNum)
	}
	// 只有 0001 一张、无缺号
	for _, is := range r.Issues {
		if is.Kind == IssueMissingNum {
			t.Errorf("不应报缺号：%s", is.Message)
		}
	}
}

// TestCheckSequenceWalksSubdirs 子目录里的 .md 也会被 generate 递归吃进账本，必须被检出。
func TestCheckSequenceWalksSubdirs(t *testing.T) {
	dir := t.TempDir()
	touchFiles(t, dir, "记字第0001号.md", "_draft/记字第0009号.md", "nested/deep/草稿.md")
	r := mustCheck(t, dir)
	var got []string
	for _, is := range r.Issues {
		if is.Kind == IssueNonStandard {
			got = append(got, is.Files[0])
		}
	}
	if len(got) != 2 {
		t.Fatalf("应检出 2 个子目录文件，实际 %v", got)
	}
	if got[0] != "_draft/记字第0009号.md" || got[1] != "nested/deep/草稿.md" {
		t.Errorf("子目录文件列表 = %v（应升序且带相对路径）", got)
	}
}

func TestCheckSequenceMissingListCapped(t *testing.T) {
	dir := t.TempDir()
	// 只有 0001 与 0500 → 缺 2..499（498 个），明细应被截断
	touchFiles(t, dir, "记字第0001号.md", "记字第0500号.md")
	r := mustCheck(t, dir)
	for _, is := range r.Issues {
		if is.Kind == IssueMissingNum {
			if !strings.Contains(is.Message, "等共 498 个") {
				t.Errorf("缺号明细应被截断并给出总数：%s", is.Message)
			}
			return
		}
	}
	t.Fatal("应检出缺号")
}

func TestCheckSequenceDeterministicOrder(t *testing.T) {
	dir := t.TempDir()
	touchFiles(t, dir, "记字第0001号.md", "记字第0004号.md", "草稿.md", "记字第0004号 副本.md")
	first := mustCheck(t, dir)
	for i := 0; i < 5; i++ {
		next := mustCheck(t, dir)
		if len(next.Issues) != len(first.Issues) {
			t.Fatalf("问题条数不稳定")
		}
		for j := range next.Issues {
			if next.Issues[j].Kind != first.Issues[j].Kind || next.Issues[j].Message != first.Issues[j].Message {
				t.Fatalf("输出顺序不稳定：第 %d 条 %q vs %q", j, next.Issues[j].Message, first.Issues[j].Message)
			}
		}
	}
	// 非正式在前、缺号在后
	if first.Issues[0].Kind != IssueNonStandard {
		t.Errorf("首条应为非正式文件，实际 %v", first.Issues[0].Kind)
	}
	if last := first.Issues[len(first.Issues)-1]; last.Kind != IssueMissingNum {
		t.Errorf("末条应为缺号，实际 %v", last.Kind)
	}
}

func TestCheckSequenceErrors(t *testing.T) {
	if _, err := CheckSequence(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Error("目录不存在应报错")
	}
	f := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckSequence(f); err == nil {
		t.Error("传入文件应报错")
	}
}
