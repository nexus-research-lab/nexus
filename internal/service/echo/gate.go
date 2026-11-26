// INPUT: 主窗口最近的可见对话与当前用户的后台模型选择。
// OUTPUT: 轻量 Gate 的 follow_up/skip 决定、开口时交给原 Agent 的生成提示，以及活跃时段换算。
// POS: Echo 的判断内核；不持有状态，不起轮，不写库。
package echo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	echodomain "github.com/nexus-research-lab/nexus/internal/echo"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	"github.com/nexus-research-lab/nexus/internal/service/llm"
)

const (
	echoHistoryLimit = 12
	echoHistoryRunes = 24_000

	gateFollowUp = "follow_up"
	gateSkip     = "skip"
)

type gateDecision struct {
	Decision   string `json:"decision"`
	ReasonCode string `json:"reason_code"`
	Focus      string `json:"focus"`
}

func (s *Service) evaluateConversation(
	ctx context.Context,
	candidate dmsvc.EchoCandidate,
	history []protocol.Message,
) (gateDecision, error) {
	config, err := s.resolveGateConfig(ctx, candidate.OwnerUserID, candidate.AgentID)
	if err != nil {
		return gateDecision{}, err
	}
	raw, err := s.llmClient.GenerateText(ctx, llm.GenerateTextRequest{
		Config: config,
		System: "你是 Echo 的发送门。判断这段私聊是否存在自然、具体、对用户有价值且值得现在主动跟进的一件事。避免泛泛问候、催促、重复答案、营销语气和无新信息的提醒。涉及高风险或敏感建议、需要工具或外部信息时必须跳过。只返回严格 JSON。",
		Messages: []llm.Message{{
			Role: "user",
			Content: "最近对话：\n" + buildGateHistory(history) +
				"\n\n只返回三个字段：decision 为 follow_up 或 skip；follow_up 的 reason_code 只能是 awaiting_answer、promised_followup、unfinished_decision、requested_check_in，且 focus 必须是一句不超过 160 字的具体关注点；skip 的 reason_code 只能是 concluded、no_new_value、would_repeat、social_only、too_ambiguous、sensitive_context、needs_tool，focus 为空字符串。",
		}},
		// 输出额度包含思考内容，为无法关闭思考的模型预留 JSON 正文空间。
		MaxTokens:        2048,
		Temperature:      0,
		DisableReasoning: true,
	})
	if err != nil {
		return gateDecision{}, err
	}
	decision, err := parseGateDecision(raw)
	if err != nil {
		return gateDecision{Decision: gateSkip, ReasonCode: "invalid_output"}, nil
	}
	return decision, nil
}

func (s *Service) resolveGateConfig(
	ctx context.Context,
	ownerUserID string,
	agentID string,
) (*clientopts.RuntimeConfig, error) {
	prefs, err := s.prefs.Get(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	provider := strings.TrimSpace(prefs.DefaultBackgroundModelSelection.Provider)
	model := strings.TrimSpace(prefs.DefaultBackgroundModelSelection.Model)
	if provider == "" || model == "" {
		agent, agentErr := s.agents.GetAgent(ctx, agentID)
		if agentErr != nil {
			return nil, agentErr
		}
		provider = strings.TrimSpace(agent.Options.Provider)
		model = strings.TrimSpace(agent.Options.Model)
	}
	return s.providers.ResolveLLMConfig(ctx, provider, model)
}

func parseGateDecision(raw string) (gateDecision, error) {
	var decision gateDecision
	decoder := json.NewDecoder(strings.NewReader(trimGateJSONFence(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decision); err != nil {
		return gateDecision{}, fmt.Errorf("Echo Gate 返回格式无效: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return gateDecision{}, errors.New("Echo Gate 包含多余内容")
	}
	decision.Decision = strings.TrimSpace(decision.Decision)
	decision.ReasonCode = strings.TrimSpace(decision.ReasonCode)
	decision.Focus = strings.TrimSpace(decision.Focus)
	if !validGateReason(decision.Decision, decision.ReasonCode) {
		return gateDecision{}, errors.New("Echo Gate 决定或原因无效")
	}
	if decision.Decision == gateFollowUp {
		if decision.Focus == "" {
			return gateDecision{}, errors.New("Echo Gate 缺少具体关注点")
		}
		if len([]rune(decision.Focus)) > 160 {
			return gateDecision{}, errors.New("Echo Gate 关注点过长")
		}
	} else {
		decision.Focus = ""
	}
	return decision, nil
}

func trimGateJSONFence(raw string) string {
	raw = strings.TrimSpace(raw)
	for _, prefix := range []string{"```json\n", "```\n"} {
		if strings.HasPrefix(raw, prefix) && strings.HasSuffix(raw, "\n```") {
			return strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(raw, prefix), "\n```"))
		}
	}
	return raw
}

func validGateReason(decision string, reason string) bool {
	switch decision {
	case gateFollowUp:
		switch reason {
		case "awaiting_answer", "promised_followup", "unfinished_decision", "requested_check_in":
			return true
		}
	case gateSkip:
		switch reason {
		case "concluded", "no_new_value", "would_repeat", "social_only", "too_ambiguous", "sensitive_context", "needs_tool":
			return true
		}
	}
	return false
}

func buildGateHistory(messages []protocol.Message) string {
	lines := make([]string, 0, echoHistoryLimit)
	for index := len(messages) - 1; index >= 0 && len(lines) < echoHistoryLimit; index-- {
		message := messages[index]
		role := protocol.MessageRole(message)
		if role != "user" && role != "assistant" || message["hidden_from_user"] == true {
			continue
		}
		content := messageText(message)
		if content == "" {
			continue
		}
		label := "用户"
		if role == "assistant" {
			label = "Agent"
		}
		lines = append(lines, label+"："+content)
	}
	for left, right := 0, len(lines)-1; left < right; left, right = left+1, right-1 {
		lines[left], lines[right] = lines[right], lines[left]
	}
	value := strings.Join(lines, "\n")
	runes := []rune(value)
	if len(runes) > echoHistoryRunes {
		value = string(runes[len(runes)-echoHistoryRunes:])
	}
	return value
}

func messageText(message protocol.Message) string {
	if value, ok := message["content"].(string); ok {
		return strings.TrimSpace(value)
	}
	var parts []string
	switch content := message["content"].(type) {
	case []map[string]any:
		for _, block := range content {
			if text := messageString(block, "text"); text != "" {
				parts = append(parts, text)
			}
		}
	case []any:
		for _, raw := range content {
			if block, ok := raw.(map[string]any); ok {
				if text := messageString(block, "text"); text != "" {
					parts = append(parts, text)
				}
			}
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func buildFollowUpPrompt(focus string) string {
	return fmt.Sprintf(`你正在为当前私聊生成一次 Echo 主动跟进。会话已经安静下来；这不是对用户新消息的即时回复，而是你主动把一件仍值得继续的事自然接回来。

内部关注点（只作方向，不要照抄，也不要当成用户事实）：
%s

请像一个真正记得前文的人重新开口：
- 开头先自然承接上文。可以用一句贴合语境的轻量寒暄、回想或过渡，例如“对了……”“顺着刚才那件事……”“我又想了一下……”，但不要固定套用模板。
- 随后主动带来一个具体的新价值，例如更清楚的判断、可执行的小建议、能推动决定的具体问题，或对先前承诺的自然兑现。
- 保持该 Agent 的人物性格和会话语言；语气亲切、松弛，不像通知、催办或客服回访。
- 只发送一条简短消息，通常 2 到 5 句；没有必要时不要使用标题、清单或总结腔。
- 不要说“你还没回复”，不要责备或制造紧迫感，不要泛泛询问“还需要帮助吗”。不要提及 Echo、后台判断、等待时长或系统规则；不要执行任何工具或操作，也不要编造新事实。

如果不能在不依赖工具和新信息的前提下自然增加价值，或只能生硬地重新提问，只输出 %s`, strings.TrimSpace(focus), dmsvc.EchoNoReplyMarker)
}

func activeWindow(now time.Time, policy echodomain.Policy) (bool, time.Time, error) {
	location, err := time.LoadLocation(policy.Timezone)
	if err != nil {
		return false, time.Time{}, err
	}
	startMinute, err := echodomain.ParseClock(policy.ActiveStart)
	if err != nil {
		return false, time.Time{}, err
	}
	endMinute, err := echodomain.ParseClock(policy.ActiveEnd)
	if err != nil {
		return false, time.Time{}, err
	}
	local := now.In(location)
	startToday := time.Date(local.Year(), local.Month(), local.Day(), startMinute/60, startMinute%60, 0, 0, location)
	endToday := time.Date(local.Year(), local.Month(), local.Day(), endMinute/60, endMinute%60, 0, 0, location)
	if startMinute == endMinute {
		return true, now.UTC(), nil
	}
	if startMinute < endMinute {
		if !local.Before(startToday) && local.Before(endToday) {
			return true, now.UTC(), nil
		}
		if local.Before(startToday) {
			return false, startToday.UTC(), nil
		}
		return false, startToday.AddDate(0, 0, 1).UTC(), nil
	}
	if !local.Before(startToday) || local.Before(endToday) {
		return true, now.UTC(), nil
	}
	return false, startToday.UTC(), nil
}

func nextActiveTime(candidate time.Time, policy echodomain.Policy) (time.Time, error) {
	active, next, err := activeWindow(candidate, policy)
	if err != nil {
		return time.Time{}, err
	}
	if active {
		return candidate.UTC(), nil
	}
	return next.UTC(), nil
}

func localDayBounds(now time.Time, timezone string) (time.Time, time.Time, error) {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	local := now.In(location)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	return start.UTC(), start.AddDate(0, 0, 1).UTC(), nil
}

func messageString(message map[string]any, key string) string {
	value, _ := message[key].(string)
	return strings.TrimSpace(value)
}
