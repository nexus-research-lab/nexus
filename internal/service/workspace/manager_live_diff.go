package workspace

import "strings"

func buildDiffStats(before *string, after *string) *DiffStats {
	if before == nil && after == nil {
		return nil
	}
	beforeLines := splitLiveLines(before)
	afterLines := splitLiveLines(after)
	if len(beforeLines) == 0 && len(afterLines) == 0 {
		return nil
	}

	commonPrefix := 0
	for commonPrefix < len(beforeLines) && commonPrefix < len(afterLines) && beforeLines[commonPrefix] == afterLines[commonPrefix] {
		commonPrefix++
	}

	commonSuffix := 0
	for commonSuffix < len(beforeLines)-commonPrefix && commonSuffix < len(afterLines)-commonPrefix &&
		beforeLines[len(beforeLines)-1-commonSuffix] == afterLines[len(afterLines)-1-commonSuffix] {
		commonSuffix++
	}

	deletions := len(beforeLines) - commonPrefix - commonSuffix
	additions := len(afterLines) - commonPrefix - commonSuffix
	if additions < 0 {
		additions = 0
	}
	if deletions < 0 {
		deletions = 0
	}
	if additions == 0 && deletions == 0 && before != nil && after != nil && *before != *after {
		additions = len(afterLines)
		deletions = len(beforeLines)
	}
	if additions == 0 && deletions == 0 {
		return nil
	}
	return &DiffStats{
		Additions:    additions,
		Deletions:    deletions,
		ChangedLines: additions + deletions,
	}
}

func splitLiveLines(content *string) []string {
	if content == nil || *content == "" {
		return nil
	}
	return strings.Split(*content, "\n")
}
