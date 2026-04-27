package memory

import (
	"regexp"
	"sort"
	"strings"
)

var asciiTokenPattern = regexp.MustCompile(`[0-9a-zA-Z_]+`)

// SimilarityMatcher 负责发现相似条目。
type SimilarityMatcher struct{}

// FindRelated 返回相似条目。
func (SimilarityMatcher) FindRelated(target *Entry, candidates []*Entry, limit int) []*Entry {
	if limit <= 0 {
		limit = 5
	}
	type scored struct {
		score float64
		entry *Entry
	}
	scoredEntries := make([]scored, 0, len(candidates))
	for _, candidate := range candidates {
		score := scoreEntry(target, candidate)
		if score < 0.5 {
			continue
		}
		scoredEntries = append(scoredEntries, scored{score: score, entry: candidate})
	}
	sort.Slice(scoredEntries, func(i int, j int) bool {
		return scoredEntries[i].score > scoredEntries[j].score
	})
	items := make([]*Entry, 0, min(limit, len(scoredEntries)))
	for index, item := range scoredEntries {
		if index >= limit {
			break
		}
		items = append(items, item.entry)
	}
	return items
}

func scoreEntry(target *Entry, candidate *Entry) float64 {
	if target == nil || candidate == nil {
		return 0
	}
	if target.Kind != candidate.Kind {
		return 0
	}
	if target.Kind == "LRN" && target.Category != candidate.Category {
		return 0
	}

	// 这里同时看词项重叠和编辑距离相似度，
	// 既避免中文短句只靠空格分词失真，也避免英文长句完全靠字符比较。
	left := normalizeEntryText(target)
	right := normalizeEntryText(candidate)
	return maxFloat(tokenOverlap(left, right), levenshteinSimilarity(left, right))
}

func normalizeEntryText(entry *Entry) string {
	chunks := []string{entry.Title}
	for _, key := range []string{"详情", "行动", "错误", "上下文", "需求", "反思", "经验"} {
		value := strings.TrimSpace(entry.FieldValue(key))
		if value != "" {
			chunks = append(chunks, value)
		}
	}
	return strings.ToLower(strings.Join(chunks, " "))
}

func tokenOverlap(left string, right string) float64 {
	leftTokens := tokenizeText(left)
	rightTokens := tokenizeText(right)
	if len(leftTokens) == 0 || len(rightTokens) == 0 {
		return 0
	}
	common := 0
	for token := range leftTokens {
		if _, ok := rightTokens[token]; ok {
			common++
		}
	}
	denominator := len(leftTokens)
	if len(rightTokens) < denominator {
		denominator = len(rightTokens)
	}
	return float64(common) / float64(denominator)
}

func tokenizeText(raw string) map[string]struct{} {
	items := make(map[string]struct{})
	for _, token := range asciiTokenPattern.FindAllString(raw, -1) {
		items[token] = struct{}{}
	}
	for _, value := range raw {
		if value >= '\u4e00' && value <= '\u9fff' {
			items[string(value)] = struct{}{}
		}
	}
	return items
}

func levenshteinSimilarity(left string, right string) float64 {
	leftRunes := []rune(left)
	rightRunes := []rune(right)
	if len(leftRunes) == 0 && len(rightRunes) == 0 {
		return 1
	}
	previous := make([]int, len(rightRunes)+1)
	current := make([]int, len(rightRunes)+1)
	for index := range previous {
		previous[index] = index
	}
	for leftIndex, leftValue := range leftRunes {
		current[0] = leftIndex + 1
		for rightIndex, rightValue := range rightRunes {
			cost := 0
			if leftValue != rightValue {
				cost = 1
			}
			current[rightIndex+1] = minInt(
				current[rightIndex]+1,
				previous[rightIndex+1]+1,
				previous[rightIndex]+cost,
			)
		}
		copy(previous, current)
	}
	distance := previous[len(rightRunes)]
	maxLength := len(leftRunes)
	if len(rightRunes) > maxLength {
		maxLength = len(rightRunes)
	}
	return 1 - (float64(distance) / float64(maxLength))
}

func minInt(values ...int) int {
	best := values[0]
	for _, value := range values[1:] {
		if value < best {
			best = value
		}
	}
	return best
}

func maxFloat(left float64, right float64) float64 {
	if left > right {
		return left
	}
	return right
}
