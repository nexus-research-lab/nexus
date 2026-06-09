package sessionresume

import (
	"strings"

	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

// Reason 描述 Nexus 产品层 session resume 决策原因。
type Reason string

const (
	ReasonEmptySessionID        Reason = "empty_session_id"
	ReasonNoTranscriptStore     Reason = "no_transcript_store"
	ReasonNonTranscriptSession  Reason = "non_transcript_session"
	ReasonTranscriptExists      Reason = "transcript_exists"
	ReasonTranscriptMissing     Reason = "transcript_missing"
	ReasonTranscriptCheckFailed Reason = "transcript_check_failed"
)

// TranscriptStore 提供 Nexus 产品层可见的 transcript 存在性检查。
type TranscriptStore interface {
	TranscriptSessionExists(workspacePath string, sessionID string) (bool, error)
}

// Decision 表示一次 session resume 策略判断结果。
type Decision struct {
	SessionID string
	Allowed   bool
	Reason    Reason
	Err       error
}

// Policy 负责 Nexus 产品层对持久化 resume 状态的判断。
//
// 这里不依赖 bridge client 或 SDK wire 类型：bridge 负责运行时连接和消息流，
// SDK 负责协议结构；Nexus 产品层负责决定哪些 session id 可以成为产品入口的 resume 状态。
type Policy struct {
	history TranscriptStore
}

// NewPolicy 创建 session resume 策略。
func NewPolicy(history TranscriptStore) Policy {
	return Policy{history: history}
}

// CanPersist 判断一个 runtime session id 是否可以写入 Nexus 的持久化 resume 状态。
func (p Policy) CanPersist(workspacePath string, sessionID string) Decision {
	normalizedSessionID := strings.TrimSpace(sessionID)
	if normalizedSessionID == "" {
		return Decision{SessionID: normalizedSessionID, Allowed: false, Reason: ReasonEmptySessionID}
	}
	if p.history == nil {
		return Decision{SessionID: normalizedSessionID, Allowed: true, Reason: ReasonNoTranscriptStore}
	}
	if !workspacestore.IsTranscriptSessionID(normalizedSessionID) {
		return Decision{SessionID: normalizedSessionID, Allowed: true, Reason: ReasonNonTranscriptSession}
	}
	return p.checkTranscript(workspacePath, normalizedSessionID)
}

// CanResume 判断一个已持久化的 resume id 是否仍可传给 runtime。
func (p Policy) CanResume(workspacePath string, sessionID string) Decision {
	normalizedSessionID := strings.TrimSpace(sessionID)
	if normalizedSessionID == "" {
		return Decision{SessionID: normalizedSessionID, Allowed: false, Reason: ReasonEmptySessionID}
	}
	if p.history == nil {
		return Decision{SessionID: normalizedSessionID, Allowed: true, Reason: ReasonNoTranscriptStore}
	}
	return p.checkTranscript(workspacePath, normalizedSessionID)
}

func (p Policy) checkTranscript(workspacePath string, sessionID string) Decision {
	exists, err := p.history.TranscriptSessionExists(workspacePath, sessionID)
	if err != nil {
		return Decision{
			SessionID: sessionID,
			Allowed:   false,
			Reason:    ReasonTranscriptCheckFailed,
			Err:       err,
		}
	}
	if exists {
		return Decision{SessionID: sessionID, Allowed: true, Reason: ReasonTranscriptExists}
	}
	return Decision{SessionID: sessionID, Allowed: false, Reason: ReasonTranscriptMissing}
}
