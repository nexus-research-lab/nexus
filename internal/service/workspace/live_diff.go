package workspace

import "strings"

func buildDiffStats(before *string, after *string) *DiffStats {
	if before == nil && after == nil {
		return nil
	}
	diff := liveLineDiff{
		before:      before,
		after:       after,
		beforeLines: splitLiveLines(before),
		afterLines:  splitLiveLines(after),
	}
	return diff.stats()
}

type liveLineDiff struct {
	before      *string
	after       *string
	beforeLines []string
	afterLines  []string
}

func (d liveLineDiff) stats() *DiffStats {
	if len(d.beforeLines) == 0 && len(d.afterLines) == 0 {
		return nil
	}
	prefix := commonLinePrefix(d.beforeLines, d.afterLines)
	suffix := commonLineSuffix(d.beforeLines, d.afterLines, prefix)
	additions := max(0, len(d.afterLines)-prefix-suffix)
	deletions := max(0, len(d.beforeLines)-prefix-suffix)
	additions, deletions = d.fullChangeFallback(additions, deletions)
	if additions == 0 && deletions == 0 {
		return nil
	}
	return &DiffStats{
		Additions:    additions,
		Deletions:    deletions,
		ChangedLines: additions + deletions,
	}
}

func commonLinePrefix(before []string, after []string) int {
	length := 0
	for length < len(before) && length < len(after) && before[length] == after[length] {
		length++
	}
	return length
}

func commonLineSuffix(before []string, after []string, prefix int) int {
	length := 0
	for length < len(before)-prefix && length < len(after)-prefix &&
		before[len(before)-1-length] == after[len(after)-1-length] {
		length++
	}
	return length
}

func (d liveLineDiff) fullChangeFallback(additions int, deletions int) (int, int) {
	if additions != 0 || deletions != 0 || d.before == nil || d.after == nil || *d.before == *d.after {
		return additions, deletions
	}
	return len(d.afterLines), len(d.beforeLines)
}

func splitLiveLines(content *string) []string {
	if content == nil || *content == "" {
		return nil
	}
	return strings.Split(*content, "\n")
}
