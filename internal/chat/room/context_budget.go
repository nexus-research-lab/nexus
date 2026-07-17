package room

import (
	"strings"
	"unicode"
)

const (
	defaultRoomContextWindowTokens = 32_768
	minRoomContextBudgetTokens     = 2_048
	maxRoomContextBudgetTokens     = 12_000
	roomContextEnvelopeTokens      = 192
	roomContextTruncatedSuffix     = "\n...(truncated by Nexus context budget)"
)

// RoomContextBudget 描述 Nexus 注入单轮 Room 可见上下文时可占用的 token 预算。
// 预算只覆盖产品附加的 Room 上下文；运行时历史、系统提示词和输出空间由内核自行管理。
type RoomContextBudget struct {
	ContextWindowTokens int
	TotalTokens         int
}

// NewRoomContextBudget 根据模型窗口生成保守的 Room 动态上下文预算。
func NewRoomContextBudget(contextWindowTokens int) RoomContextBudget {
	if contextWindowTokens <= 0 {
		contextWindowTokens = defaultRoomContextWindowTokens
	}
	total := contextWindowTokens / 12
	total = max(total, minRoomContextBudgetTokens)
	total = min(total, maxRoomContextBudgetTokens)
	return RoomContextBudget{
		ContextWindowTokens: contextWindowTokens,
		TotalTokens:         total,
	}
}

func (b RoomContextBudget) contentTokens() int {
	return max(0, b.TotalTokens-roomContextEnvelopeTokens)
}

func (b RoomContextBudget) currentMessageLimit() int {
	return max(512, b.contentTokens()*35/100)
}

func (b RoomContextBudget) publicDeltaLimit() int {
	return max(768, b.contentTokens()/2)
}

func (b RoomContextBudget) privateDeltaLimit() int {
	return max(512, b.contentTokens()*30/100)
}

func (b RoomContextBudget) publicAnchorLimit() int {
	return max(384, b.contentTokens()/5)
}

// estimateRoomTokens 使用保守、无模型依赖的规则估算文本 token。
// CJK 字符按一个 token 计，连续 ASCII 单词按四字符一个 token 计。
func estimateRoomTokens(value string) int {
	tokens := 0
	asciiRun := 0
	flushASCII := func() {
		if asciiRun == 0 {
			return
		}
		tokens += (asciiRun + 3) / 4
		asciiRun = 0
	}
	for _, character := range strings.TrimSpace(value) {
		switch {
		case character <= unicode.MaxASCII && (unicode.IsLetter(character) || unicode.IsDigit(character)):
			asciiRun++
		case unicode.IsSpace(character):
			flushASCII()
		default:
			flushASCII()
			tokens++
		}
	}
	flushASCII()
	return tokens
}

func fitRoomText(value string, maxTokens int) (string, int) {
	value = strings.TrimSpace(value)
	if value == "" || maxTokens <= 0 {
		return "", 0
	}
	if tokens := estimateRoomTokens(value); tokens <= maxTokens {
		return value, tokens
	}

	suffixTokens := estimateRoomTokens(roomContextTruncatedSuffix)
	if maxTokens <= suffixTokens {
		return fitRoomTextPrefix(value, maxTokens)
	}
	body, _ := fitRoomTextPrefix(value, maxTokens-suffixTokens)
	result := strings.TrimSpace(body) + roomContextTruncatedSuffix
	return result, estimateRoomTokens(result)
}

func fitRoomTextPrefix(value string, maxTokens int) (string, int) {
	runes := []rune(strings.TrimSpace(value))
	low, high := 0, len(runes)
	for low < high {
		middle := (low + high + 1) / 2
		if estimateRoomTokens(string(runes[:middle])) <= maxTokens {
			low = middle
			continue
		}
		high = middle - 1
	}
	result := strings.TrimSpace(string(runes[:low]))
	return result, estimateRoomTokens(result)
}
