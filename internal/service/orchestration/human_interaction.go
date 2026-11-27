// INPUT: Execution snapshot 中仍在运行的 root Attempt 与宿主注入的待响应人工交互快照。
// OUTPUT: 按 Attempt ID 归组、已排序的人工交互阻塞事实；读取器缺失时为空，不影响其他投影。
// POS: 让 coordinator 区分"负责人在等用户回答"与"runtime 失活"的唯一编排层入口。
package orchestration

import (
	"slices"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// HumanInteractionKind 区分阻塞 Attempt 的人工交互类型。
type HumanInteractionKind string

const (
	HumanInteractionQuestion   HumanInteractionKind = "question"
	HumanInteractionPermission HumanInteractionKind = "permission"
)

const askUserQuestionToolName = "AskUserQuestion"

// PendingHumanInteraction 是宿主报告的一个仍在等待人工响应的请求。
// 只携带身份与时间，不携带工具输入，避免把用户侧内容带进协调上下文。
type PendingHumanInteraction struct {
	RequestID         string
	RuntimeSessionKey string
	AgentID           string
	AgentRoundID      string
	ToolName          string
	CreatedAt         time.Time
}

// Kind 返回该交互在协调上下文中的类型。
func (p PendingHumanInteraction) Kind() HumanInteractionKind {
	if strings.TrimSpace(p.ToolName) == askUserQuestionToolName {
		return HumanInteractionQuestion
	}
	return HumanInteractionPermission
}

// PendingHumanInteractionReader 由宿主实现，按前端路由 session 读取待响应人工交互。
// Room 中全部成员 slot 共享 Execution.SessionKey 作为路由 session。
type PendingHumanInteractionReader interface {
	PendingHumanInteractions(dispatchSessionKey string) []PendingHumanInteraction
}

// SetPendingHumanInteractionReader 注入宿主的人工交互读取器；未注入时上下文不渲染等待状态。
func (s *Service) SetPendingHumanInteractionReader(reader PendingHumanInteractionReader) {
	s.humanInteractions = reader
}

// awaitingHumanByAttempt 把待响应人工交互归属到正在运行的 root Attempt。
// 归属条件：交互来自 Attempt 的执行 Agent，且 Attempt 已记录 runtime session 时必须一致。
// agent_round 不作为必要条件：同一 Attempt 的负责人可能在后续唤醒轮次中提问。
func (s *Service) awaitingHumanByAttempt(
	snapshot *protocol.ExecutionSnapshot,
) map[string][]PendingHumanInteraction {
	if s == nil || s.humanInteractions == nil || snapshot == nil {
		return nil
	}
	sessionKey := strings.TrimSpace(snapshot.Execution.SessionKey)
	if sessionKey == "" {
		return nil
	}
	interactions := s.humanInteractions.PendingHumanInteractions(sessionKey)
	if len(interactions) == 0 {
		return nil
	}
	return matchAwaitingHumanAttempts(snapshot.Attempts, interactions)
}

func matchAwaitingHumanAttempts(
	attempts []protocol.WorkAttempt,
	interactions []PendingHumanInteraction,
) map[string][]PendingHumanInteraction {
	result := make(map[string][]PendingHumanInteraction)
	for _, attempt := range attempts {
		if attempt.Status != protocol.WorkAttemptStatusPending &&
			attempt.Status != protocol.WorkAttemptStatusRunning {
			continue
		}
		if strings.TrimSpace(attempt.ParentAttemptID) != "" {
			continue
		}
		agentID := strings.TrimSpace(attempt.ExecutorAgentID)
		if agentID == "" {
			continue
		}
		runtimeSessionKey := strings.TrimSpace(attempt.RuntimeSessionKey)
		for _, interaction := range interactions {
			if strings.TrimSpace(interaction.AgentID) != agentID {
				continue
			}
			if runtimeSessionKey != "" &&
				strings.TrimSpace(interaction.RuntimeSessionKey) != "" &&
				strings.TrimSpace(interaction.RuntimeSessionKey) != runtimeSessionKey {
				continue
			}
			result[attempt.ID] = append(result[attempt.ID], interaction)
		}
	}
	for attemptID := range result {
		slices.SortFunc(result[attemptID], comparePendingHumanInteractions)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func comparePendingHumanInteractions(left PendingHumanInteraction, right PendingHumanInteraction) int {
	if order := left.CreatedAt.Compare(right.CreatedAt); order != 0 {
		return order
	}
	return strings.Compare(left.RequestID, right.RequestID)
}

// awaitingHumanKind 在同一 Attempt 存在多个交互时优先报告 question。
func awaitingHumanKind(interactions []PendingHumanInteraction) HumanInteractionKind {
	for _, interaction := range interactions {
		if interaction.Kind() == HumanInteractionQuestion {
			return HumanInteractionQuestion
		}
	}
	return HumanInteractionPermission
}
