package workspace

import "testing"

func TestBuildDiffStatsCountsChangedMiddleLines(t *testing.T) {
	before := "first\nbefore\nlast"
	after := "first\nafter\nlast"
	stats := buildDiffStats(&before, &after)
	if stats == nil || stats.Additions != 1 || stats.Deletions != 1 || stats.ChangedLines != 2 {
		t.Fatalf("diff 统计错误: %+v", stats)
	}
}

func TestBuildDiffStatsIgnoresEqualContent(t *testing.T) {
	content := "same"
	if stats := buildDiffStats(&content, &content); stats != nil {
		t.Fatalf("相同内容不应产生 diff: %+v", stats)
	}
}
