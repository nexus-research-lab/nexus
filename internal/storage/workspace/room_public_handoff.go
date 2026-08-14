// INPUT: Room Agent handoff 的检测、host-only Goal collaboration attribution、排队、启动、终态与 Goal handback 事件。
// OUTPUT: 可跨进程恢复、按 handoff_id 幂等且能 fence 精确 Goal revision 的 append-only ledger。
// POS: 公区 handoff、structured Execution dispatch 与 Goal-attributed directed wake 的持久化事实源；InputQueue 只负责 busy 目标的投递。
package workspace

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const (
	roomPublicHandoffActionDetected       = "detected"
	roomPublicHandoffActionSourceFinished = "source_finished"
	roomPublicHandoffActionQueued         = "queued"
	roomPublicHandoffActionClaimed        = "claimed"
	roomPublicHandoffActionStarted        = "started"
	roomPublicHandoffActionTerminal       = "terminal"
	roomPublicHandoffActionGoalHandback   = "goal_handback_settled"
	roomPublicHandoffActionCancelled      = "cancelled"
)

// RoomPublicHandoff 表示一条从 source slot 指向 target Agent 的持久协作边。
// 状态迁移通过同一 handoff_id 追加事件，重放后得到最后一个有效状态。
type RoomPublicHandoff struct {
	HandoffID                string                             `json:"handoff_id"`
	OwnerUserID              string                             `json:"owner_user_id,omitempty"`
	ConversationID           string                             `json:"conversation_id"`
	RoomID                   string                             `json:"room_id,omitempty"`
	RootRoundID              string                             `json:"root_round_id,omitempty"`
	SourceAgentRoundID       string                             `json:"source_agent_round_id,omitempty"`
	SourceMessageID          string                             `json:"source_message_id"`
	SourceAgentID            string                             `json:"source_agent_id"`
	TargetAgentID            string                             `json:"target_agent_id"`
	Content                  string                             `json:"content"`
	ReplyRoute               protocol.RoomReplyRoute            `json:"reply_route,omitempty"`
	QueueSource              protocol.InputQueueSource          `json:"queue_source,omitempty"`
	GoalCollaborationBinding *protocol.GoalCollaborationBinding `json:"goal_collaboration_binding,omitempty"`
	WorkBinding              *protocol.ExecutionWorkBinding     `json:"work_binding,omitempty"`
	ReviewBinding            *protocol.ExecutionReviewBinding   `json:"review_binding,omitempty"`
	HopIndex                 int                                `json:"hop_index,omitempty"`
	QueueItemID              string                             `json:"queue_item_id,omitempty"`
	TargetRoundID            string                             `json:"target_round_id,omitempty"`
	TargetAgentRoundID       string                             `json:"target_agent_round_id,omitempty"`
	GoalSubstantiveOutput    bool                               `json:"goal_substantive_output,omitempty"`
	GoalPublicEvidence       bool                               `json:"goal_public_evidence,omitempty"`
	GoalHandbackRequired     bool                               `json:"goal_handback_required,omitempty"`
	GoalHandbackSettled      bool                               `json:"goal_handback_settled,omitempty"`
	Status                   string                             `json:"status"`
	ClaimedAt                int64                              `json:"claimed_at,omitempty"`
	CreatedAt                int64                              `json:"created_at"`
	UpdatedAt                int64                              `json:"updated_at"`
}

// RoomPublicHandoffStore 负责 Room handoff ledger 的并发追加与重放。
type RoomPublicHandoffStore struct {
	paths *Store
	files *SessionFileStore
	mu    sync.Mutex
}

// NewRoomPublicHandoffStore 创建 Room handoff ledger。
func NewRoomPublicHandoffStore(root string) *RoomPublicHandoffStore {
	paths := New(root)
	return &RoomPublicHandoffStore{paths: paths, files: newSessionFileStore(paths)}
}

// Detect 记录一条新 handoff；相同 handoff_id 重复检测时保持幂等。
func (s *RoomPublicHandoffStore) Detect(
	ownerUserID string,
	handoff RoomPublicHandoff,
) (RoomPublicHandoff, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	handoff.OwnerUserID = strings.TrimSpace(ownerUserID)
	if err := validateRoomPublicHandoff(handoff); err != nil {
		return RoomPublicHandoff{}, false, err
	}
	all, err := s.replayLocked(ownerUserID, handoff.ConversationID)
	if err != nil {
		return RoomPublicHandoff{}, false, err
	}
	if existing, ok := all[handoff.HandoffID]; ok {
		return existing, false, nil
	}
	now := time.Now().UnixMilli()
	handoff.Status = roomPublicHandoffActionDetected
	if handoff.CreatedAt == 0 {
		handoff.CreatedAt = now
	}
	handoff.UpdatedAt = now
	if err := s.appendLocked(ownerUserID, handoff.ConversationID, roomPublicHandoffActionDetected, handoff); err != nil {
		return RoomPublicHandoff{}, false, err
	}
	return handoff, true, nil
}

// MarkSourceFinished 标记 source slot 已成功发布最终消息。
func (s *RoomPublicHandoffStore) MarkSourceFinished(ownerUserID string, conversationID string, handoffID string) error {
	return s.transition(ownerUserID, conversationID, handoffID, roomPublicHandoffActionSourceFinished, func(value *RoomPublicHandoff) {
		value.Status = roomPublicHandoffActionSourceFinished
		value.ClaimedAt = 0
	}, func(value RoomPublicHandoff) bool {
		return value.Status == roomPublicHandoffActionDetected ||
			value.Status == roomPublicHandoffActionQueued
	})
}

// ReopenStartedForRecovery moves a replay-safe started edge back to
// source_finished during process-start recovery. Normal delivery retries must
// not call this method or regress a live target round.
func (s *RoomPublicHandoffStore) ReopenStartedForRecovery(
	ownerUserID string,
	conversationID string,
	handoffID string,
) error {
	return s.transition(ownerUserID, conversationID, handoffID, roomPublicHandoffActionSourceFinished, func(value *RoomPublicHandoff) {
		value.Status = roomPublicHandoffActionSourceFinished
		value.ClaimedAt = 0
	}, func(value RoomPublicHandoff) bool {
		return value.Status == roomPublicHandoffActionStarted &&
			roomPublicHandoffCanReplayStarted(value)
	})
}

// MarkQueued 记录 handoff 已进入 busy 目标的 InputQueue。
func (s *RoomPublicHandoffStore) MarkQueued(
	ownerUserID string,
	conversationID string,
	handoffID string,
	queueItemID string,
) error {
	return s.transition(ownerUserID, conversationID, handoffID, roomPublicHandoffActionQueued, func(value *RoomPublicHandoff) {
		value.Status = roomPublicHandoffActionQueued
		value.QueueItemID = strings.TrimSpace(queueItemID)
		value.ClaimedAt = 0
	}, func(value RoomPublicHandoff) bool {
		return value.Status == roomPublicHandoffActionDetected ||
			value.Status == roomPublicHandoffActionSourceFinished ||
			value.Status == roomPublicHandoffActionQueued
	})
}

// Claim 为本进程准备启动 handoff，防止恢复器与实时路径重复创建 target round。
func (s *RoomPublicHandoffStore) Claim(
	ownerUserID string,
	conversationID string,
	handoffID string,
) (RoomPublicHandoff, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.replayLocked(ownerUserID, conversationID)
	if err != nil {
		return RoomPublicHandoff{}, false, err
	}
	value, ok := all[strings.TrimSpace(handoffID)]
	if !ok || !roomPublicHandoffCanStart(value) {
		return value, false, nil
	}
	now := time.Now().UnixMilli()
	value.Status = roomPublicHandoffActionClaimed
	value.ClaimedAt = now
	value.UpdatedAt = now
	if err := s.appendLocked(ownerUserID, conversationID, roomPublicHandoffActionClaimed, value); err != nil {
		return RoomPublicHandoff{}, false, err
	}
	return value, true, nil
}

// ReleaseClaim 将启动失败的 handoff 恢复为 source_finished，允许下一次重试。
func (s *RoomPublicHandoffStore) ReleaseClaim(ownerUserID string, conversationID string, handoffID string) error {
	return s.transition(ownerUserID, conversationID, handoffID, roomPublicHandoffActionSourceFinished, func(value *RoomPublicHandoff) {
		value.Status = roomPublicHandoffActionSourceFinished
		value.ClaimedAt = 0
	}, func(value RoomPublicHandoff) bool {
		return value.Status == roomPublicHandoffActionClaimed
	})
}

// MarkStarted 记录 target round 已创建。
func (s *RoomPublicHandoffStore) MarkStarted(
	ownerUserID string,
	conversationID string,
	handoffID string,
	targetRoundID string,
) error {
	return s.transition(ownerUserID, conversationID, handoffID, roomPublicHandoffActionStarted, func(value *RoomPublicHandoff) {
		value.Status = roomPublicHandoffActionStarted
		value.TargetRoundID = strings.TrimSpace(targetRoundID)
		value.ClaimedAt = 0
	}, func(value RoomPublicHandoff) bool {
		return value.Status == roomPublicHandoffActionClaimed
	})
}

// MarkTerminal 收口 target handoff，status 使用 finished、error 或 interrupted。
func (s *RoomPublicHandoffStore) MarkTerminal(
	ownerUserID string,
	conversationID string,
	handoffID string,
	status string,
) error {
	status = normalizeRoomPublicHandoffTerminalStatus(status)
	return s.transition(ownerUserID, conversationID, handoffID, roomPublicHandoffActionTerminal, func(value *RoomPublicHandoff) {
		value.Status = status
		value.ClaimedAt = 0
	}, func(value RoomPublicHandoff) bool {
		return roomPublicHandoffCanTerminal(value)
	})
}

// MarkTerminalWithGoalOutcome 在终态边上保留 Goal handback 恢复所需的最小结果。
// target round 本身始终不获得 Goal mutation authority。
func (s *RoomPublicHandoffStore) MarkTerminalWithGoalOutcome(
	ownerUserID string,
	conversationID string,
	handoffID string,
	status string,
	targetAgentRoundID string,
	substantiveOutput bool,
	publicEvidence bool,
) error {
	status = normalizeRoomPublicHandoffTerminalStatus(status)
	return s.transition(ownerUserID, conversationID, handoffID, roomPublicHandoffActionTerminal, func(value *RoomPublicHandoff) {
		value.Status = status
		value.ClaimedAt = 0
		value.TargetAgentRoundID = strings.TrimSpace(targetAgentRoundID)
		value.GoalSubstantiveOutput = substantiveOutput
		value.GoalPublicEvidence = publicEvidence && substantiveOutput && status == roomPublicHandoffActionFinished
		value.GoalHandbackRequired = protocol.NormalizeGoalCollaborationBinding(value.GoalCollaborationBinding) != nil
	}, func(value RoomPublicHandoff) bool {
		return roomPublicHandoffCanTerminal(value)
	})
}

// MarkGoalHandbackSettled 记录 host 已把一条终态协作边归还给精确 Goal revision。
// 该事实与 terminal 分开落盘，专门关闭 target terminal 与 Goal continuation
// 之间的进程崩溃窗口。
func (s *RoomPublicHandoffStore) MarkGoalHandbackSettled(
	ownerUserID string,
	conversationID string,
	handoffID string,
) error {
	return s.transition(ownerUserID, conversationID, handoffID, roomPublicHandoffActionGoalHandback, func(value *RoomPublicHandoff) {
		value.GoalHandbackSettled = true
	}, func(value RoomPublicHandoff) bool {
		return roomPublicHandoffIsTerminal(value.Status) &&
			protocol.NormalizeGoalCollaborationBinding(value.GoalCollaborationBinding) != nil &&
			!value.GoalHandbackSettled
	})
}

// CancelForSource 取消尚未启动的 source handoff，防止失败 source 继续唤醒目标。
func (s *RoomPublicHandoffStore) CancelForSource(
	ownerUserID string,
	conversationID string,
	sourceAgentRoundID string,
	status string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.replayLocked(ownerUserID, conversationID)
	if err != nil {
		return err
	}
	status = normalizeRoomPublicHandoffTerminalStatus(status)
	for _, value := range all {
		if strings.TrimSpace(value.SourceAgentRoundID) != strings.TrimSpace(sourceAgentRoundID) ||
			!roomPublicHandoffCanCancelSource(value) {
			continue
		}
		value.Status = status
		value.ClaimedAt = 0
		value.UpdatedAt = time.Now().UnixMilli()
		if err := s.appendLocked(ownerUserID, conversationID, roomPublicHandoffActionCancelled, value); err != nil {
			return err
		}
	}
	return nil
}

// ListRoot 返回同一 conversation/root 下已经记录过的全部 handoff 边。
// 调度层用它做 root 级去环、fanout 和取消判断；返回值是重放后的当前快照。
func (s *RoomPublicHandoffStore) ListRoot(
	ownerUserID string,
	conversationID string,
	rootRoundID string,
) ([]RoomPublicHandoff, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.replayLocked(ownerUserID, conversationID)
	if err != nil {
		return nil, err
	}
	rootRoundID = strings.TrimSpace(rootRoundID)
	result := make([]RoomPublicHandoff, 0)
	for _, value := range all {
		if strings.TrimSpace(value.RootRoundID) != rootRoundID {
			continue
		}
		result = append(result, value)
	}
	sort.SliceStable(result, func(i int, j int) bool {
		if result[i].CreatedAt != result[j].CreatedAt {
			return result[i].CreatedAt < result[j].CreatedAt
		}
		return result[i].HandoffID < result[j].HandoffID
	})
	return result, nil
}

// Get 返回指定 handoff 的当前快照，供队列恢复时重新取回逻辑 root。
func (s *RoomPublicHandoffStore) Get(
	ownerUserID string,
	conversationID string,
	handoffID string,
) (RoomPublicHandoff, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.replayLocked(ownerUserID, conversationID)
	if err != nil {
		return RoomPublicHandoff{}, false, err
	}
	value, ok := all[strings.TrimSpace(handoffID)]
	return value, ok, nil
}

// CancelForRoot 收口 root 下尚未完成的 handoff，供用户停止整轮时传播取消。
func (s *RoomPublicHandoffStore) CancelForRoot(
	ownerUserID string,
	conversationID string,
	rootRoundID string,
	status string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.replayLocked(ownerUserID, conversationID)
	if err != nil {
		return err
	}
	status = normalizeRoomPublicHandoffTerminalStatus(status)
	rootRoundID = strings.TrimSpace(rootRoundID)
	for _, value := range all {
		if strings.TrimSpace(value.RootRoundID) != rootRoundID || !roomPublicHandoffCanCancelRoot(value) {
			continue
		}
		value.Status = status
		value.ClaimedAt = 0
		value.UpdatedAt = time.Now().UnixMilli()
		if err := s.appendLocked(ownerUserID, conversationID, roomPublicHandoffActionCancelled, value); err != nil {
			return err
		}
	}
	return nil
}

// Pending 返回需要恢复或观察的 handoff。queued 仍然返回，调用方需要先确认
// 对应 InputQueue item 是否还在；若队列项已丢失，才能把它重新交给实时派发。
func (s *RoomPublicHandoffStore) Pending(ownerUserID string, conversationID string) ([]RoomPublicHandoff, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.replayLocked(ownerUserID, conversationID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()
	result := make([]RoomPublicHandoff, 0, len(all))
	for _, value := range all {
		if roomPublicHandoffIsPending(value, now) {
			result = append(result, value)
		}
	}
	sort.SliceStable(result, func(i int, j int) bool {
		if result[i].CreatedAt != result[j].CreatedAt {
			return result[i].CreatedAt < result[j].CreatedAt
		}
		return result[i].HandoffID < result[j].HandoffID
	})
	return result, nil
}

// PendingAll 逐用户扫描所有 Room handoff ledger，供进程启动恢复使用。
func (s *RoomPublicHandoffStore) PendingAll() ([]RoomPublicHandoff, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	owners, err := listRoomOwnerPathSegments(s.paths.StateRoot)
	if err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()
	result := make([]RoomPublicHandoff, 0)
	for _, ownerPathSegment := range owners {
		roomRootPath := s.paths.RoomConversationRoot(ownerPathSegment)
		root, openErr := s.files.openRoomRoot(ownerPathSegment, false)
		if errors.Is(openErr, os.ErrNotExist) {
			continue
		}
		if openErr != nil {
			return nil, openErr
		}
		entries, readDirErr := fs.ReadDir(root.FS(), ".")
		_ = root.Close()
		if readDirErr != nil {
			return nil, readDirErr
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			rows, readErr := s.files.readRoomJSONL(
				ownerPathSegment,
				filepath.Join(roomRootPath, entry.Name(), "public_handoffs.jsonl"),
			)
			if errors.Is(readErr, os.ErrNotExist) {
				continue
			}
			if readErr != nil {
				return nil, readErr
			}
			all := replayRoomPublicHandoffRows(rows)
			for _, value := range all {
				if filepath.Base(s.paths.RoomConversationDir(ownerPathSegment, value.ConversationID)) != entry.Name() {
					continue
				}
				recoveredOwnerUserID, ok := roomLedgerOwnerUserID(
					ownerPathSegment,
					value.OwnerUserID,
				)
				if !ok {
					continue
				}
				value.OwnerUserID = recoveredOwnerUserID
				if roomPublicHandoffIsStartupPending(value, now) {
					result = append(result, value)
				}
			}
		}
	}
	sort.SliceStable(result, func(i int, j int) bool {
		if result[i].CreatedAt != result[j].CreatedAt {
			return result[i].CreatedAt < result[j].CreatedAt
		}
		return result[i].HandoffID < result[j].HandoffID
	})
	return result, nil
}

// LegacyUnattributedTerminalRootsAll 返回旧版本留下的终态、未带 Goal
// collaboration attribution 的 Room handoff roots。调用方必须再用当前 Goal
// 审计事件做严格归因；storage 不从消息正文猜测 Goal 语义。
func (s *RoomPublicHandoffStore) LegacyUnattributedTerminalRootsAll() ([][]RoomPublicHandoff, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	owners, err := listRoomOwnerPathSegments(s.paths.StateRoot)
	if err != nil {
		return nil, err
	}
	result := make([][]RoomPublicHandoff, 0)
	for _, ownerPathSegment := range owners {
		roomRootPath := s.paths.RoomConversationRoot(ownerPathSegment)
		root, openErr := s.files.openRoomRoot(ownerPathSegment, false)
		if errors.Is(openErr, os.ErrNotExist) {
			continue
		}
		if openErr != nil {
			return nil, openErr
		}
		entries, readDirErr := fs.ReadDir(root.FS(), ".")
		_ = root.Close()
		if readDirErr != nil {
			return nil, readDirErr
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			rows, readErr := s.files.readRoomJSONL(
				ownerPathSegment,
				filepath.Join(roomRootPath, entry.Name(), "public_handoffs.jsonl"),
			)
			if errors.Is(readErr, os.ErrNotExist) {
				continue
			}
			if readErr != nil {
				return nil, readErr
			}
			grouped := make(map[string][]RoomPublicHandoff)
			for _, value := range replayRoomPublicHandoffRows(rows) {
				if filepath.Base(s.paths.RoomConversationDir(ownerPathSegment, value.ConversationID)) != entry.Name() ||
					protocol.NormalizeGoalCollaborationBinding(value.GoalCollaborationBinding) != nil ||
					strings.TrimSpace(value.RootRoundID) == "" {
					continue
				}
				recoveredOwnerUserID, ok := roomLedgerOwnerUserID(
					ownerPathSegment,
					value.OwnerUserID,
				)
				if !ok {
					continue
				}
				value.OwnerUserID = recoveredOwnerUserID
				key := value.OwnerUserID + "\x00" + value.ConversationID + "\x00" + value.RootRoundID
				grouped[key] = append(grouped[key], value)
			}
			for _, candidates := range grouped {
				if !roomPublicHandoffRootIsTerminal(candidates) {
					continue
				}
				sort.SliceStable(candidates, func(i int, j int) bool {
					if candidates[i].CreatedAt != candidates[j].CreatedAt {
						return candidates[i].CreatedAt < candidates[j].CreatedAt
					}
					return candidates[i].HandoffID < candidates[j].HandoffID
				})
				result = append(result, candidates)
			}
		}
	}
	sort.SliceStable(result, func(i int, j int) bool {
		left := result[i][0]
		right := result[j][0]
		if left.CreatedAt != right.CreatedAt {
			return left.CreatedAt < right.CreatedAt
		}
		if left.ConversationID != right.ConversationID {
			return left.ConversationID < right.ConversationID
		}
		return left.RootRoundID < right.RootRoundID
	})
	return result, nil
}

func roomPublicHandoffRootIsTerminal(items []RoomPublicHandoff) bool {
	if len(items) == 0 {
		return false
	}
	for _, item := range items {
		if !roomPublicHandoffIsTerminal(item.Status) {
			return false
		}
	}
	return true
}

// BindLegacyTerminalRootToGoal 将一个已被服务层证明的旧终态 root 绑定到
// 精确 Goal revision，并把每条边标为待 handback。重复调用保持幂等。
func (s *RoomPublicHandoffStore) BindLegacyTerminalRootToGoal(
	ownerUserID string,
	conversationID string,
	rootRoundID string,
	binding protocol.GoalCollaborationBinding,
	evidenceAgentID string,
	evidenceAgentRoundID string,
) error {
	bindingValue := protocol.NormalizeGoalCollaborationBinding(&binding)
	if bindingValue == nil {
		return errors.New("valid goal collaboration binding is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.replayLocked(ownerUserID, conversationID)
	if err != nil {
		return err
	}
	rootRoundID = strings.TrimSpace(rootRoundID)
	evidenceAgentID = strings.TrimSpace(evidenceAgentID)
	evidenceAgentRoundID = strings.TrimSpace(evidenceAgentRoundID)
	representativeID := ""
	var fallback RoomPublicHandoff
	for _, value := range all {
		if strings.TrimSpace(value.RootRoundID) != rootRoundID ||
			!roomPublicHandoffIsTerminal(value.Status) {
			continue
		}
		current := protocol.NormalizeGoalCollaborationBinding(value.GoalCollaborationBinding)
		if current != nil && *current != *bindingValue {
			return errors.New("legacy handoff root already belongs to a different Goal revision")
		}
		if representativeID == "" && value.Status == roomPublicHandoffActionFinished &&
			strings.TrimSpace(value.TargetAgentID) == evidenceAgentID &&
			evidenceAgentRoundID != "" {
			representativeID = value.HandoffID
		}
		if fallback.HandoffID == "" || value.UpdatedAt > fallback.UpdatedAt ||
			(value.UpdatedAt == fallback.UpdatedAt && value.HandoffID > fallback.HandoffID) {
			fallback = value
		}
	}
	if representativeID == "" {
		representativeID = fallback.HandoffID
	}
	if representativeID == "" {
		return nil
	}
	for _, value := range all {
		if strings.TrimSpace(value.RootRoundID) != rootRoundID ||
			!roomPublicHandoffIsTerminal(value.Status) {
			continue
		}
		current := protocol.NormalizeGoalCollaborationBinding(value.GoalCollaborationBinding)
		if current != nil {
			if *current != *bindingValue {
				return errors.New("legacy handoff root already belongs to a different Goal revision")
			}
		} else {
			value.GoalCollaborationBinding = cloneWorkspaceGoalCollaborationBinding(bindingValue)
		}
		value.GoalHandbackRequired = value.HandoffID == representativeID
		value.GoalHandbackSettled = false
		if value.HandoffID == representativeID && evidenceAgentRoundID != "" {
			value.TargetAgentRoundID = evidenceAgentRoundID
			value.GoalSubstantiveOutput = true
			value.GoalPublicEvidence = true
		}
		value.UpdatedAt = time.Now().UnixMilli()
		if err := s.appendLocked(ownerUserID, conversationID, "legacy_goal_attribution", value); err != nil {
			return err
		}
	}
	return nil
}

func cloneWorkspaceGoalCollaborationBinding(
	binding *protocol.GoalCollaborationBinding,
) *protocol.GoalCollaborationBinding {
	if binding == nil {
		return nil
	}
	value := *binding
	return &value
}

// GoalCollaborationInFlight reports whether this conversation still has a
// non-terminal host-attributed collaboration edge for the exact Goal revision.
// It is a scheduling fence only; callers must never turn it into model
// mutation authority.
func (s *RoomPublicHandoffStore) GoalCollaborationInFlight(
	ownerUserID string,
	conversationID string,
	binding protocol.GoalCollaborationBinding,
) (bool, error) {
	bindingValue := protocol.NormalizeGoalCollaborationBinding(&binding)
	if bindingValue == nil {
		return false, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.replayLocked(ownerUserID, conversationID)
	if err != nil {
		return false, err
	}
	for _, value := range all {
		candidate := protocol.NormalizeGoalCollaborationBinding(
			value.GoalCollaborationBinding,
		)
		if candidate == nil || *candidate != *bindingValue ||
			roomPublicHandoffCanTerminal(value) == false {
			continue
		}
		return true, nil
	}
	return false, nil
}

// GoalCollaborationInFlightAll reports whether any Room conversation owned by
// ownerUserID still carries a non-terminal edge or a terminal edge whose Goal
// handback receipt has not settled for the exact Goal revision.
// Communication MCP may deliberately deliver into another Room conversation,
// so the source Goal continuation cannot fence on its own ledger alone.
func (s *RoomPublicHandoffStore) GoalCollaborationInFlightAll(
	ownerUserID string,
	binding protocol.GoalCollaborationBinding,
) (bool, error) {
	bindingValue := protocol.NormalizeGoalCollaborationBinding(&binding)
	if bindingValue == nil {
		return false, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ownerPathSegment := appfs.UserPathSegment(strings.TrimSpace(ownerUserID))
	if ownerPathSegment == "" {
		return false, nil
	}
	roomRootPath := s.paths.RoomConversationRoot(ownerPathSegment)
	root, err := s.files.openRoomRoot(ownerPathSegment, false)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	entries, err := fs.ReadDir(root.FS(), ".")
	_ = root.Close()
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		rows, readErr := s.files.readRoomJSONL(
			ownerPathSegment,
			filepath.Join(roomRootPath, entry.Name(), "public_handoffs.jsonl"),
		)
		if errors.Is(readErr, os.ErrNotExist) {
			continue
		}
		if readErr != nil {
			return false, readErr
		}
		for _, value := range replayRoomPublicHandoffRows(rows) {
			candidate := protocol.NormalizeGoalCollaborationBinding(
				value.GoalCollaborationBinding,
			)
			if candidate != nil && *candidate == *bindingValue &&
				(roomPublicHandoffCanTerminal(value) ||
					(roomPublicHandoffIsTerminal(value.Status) &&
						roomPublicHandoffNeedsGoalHandback(value))) {
				return true, nil
			}
		}
	}
	return false, nil
}

func roomPublicHandoffIsPending(value RoomPublicHandoff, now int64) bool {
	switch value.Status {
	case roomPublicHandoffActionDetected,
		roomPublicHandoffActionSourceFinished,
		roomPublicHandoffActionQueued:
		return true
	case roomPublicHandoffActionClaimed:
		return now-value.ClaimedAt > roomPublicHandoffClaimTTL.Milliseconds()
	case roomPublicHandoffActionStarted:
		// Structured Execution dispatches and exact Goal collaboration edges
		// carry durable attribution and can be safely re-admitted after process
		// restart. Legacy public mentions keep their historical non-replay
		// behavior.
		return roomPublicHandoffCanReplayStarted(value)
	default:
		return false
	}
}

func roomPublicHandoffIsStartupPending(value RoomPublicHandoff, now int64) bool {
	if roomPublicHandoffIsTerminal(value.Status) &&
		protocol.NormalizeGoalCollaborationBinding(value.GoalCollaborationBinding) != nil &&
		roomPublicHandoffNeedsGoalHandback(value) {
		// target 已终态不等于 Goal 已继续。terminal 与 handback 是
		// 两个独立的 durable 阶段，启动恢复必须扫描中间态。
		return !value.GoalHandbackSettled
	}
	if value.Status == roomPublicHandoffActionClaimed {
		// Claims are process-local leases. No claim from the previous process is
		// live after startup, even when its wall-clock TTL has not elapsed.
		return true
	}
	return roomPublicHandoffIsPending(value, now)
}

func roomPublicHandoffNeedsGoalHandback(value RoomPublicHandoff) bool {
	if value.GoalHandbackSettled {
		return false
	}
	// TargetAgentRoundID and the outcome flags keep terminal records written by
	// the immediately preceding implementation recoverable. Generic rejection
	// and user-cancellation paths never set them and therefore cannot restart a
	// Goal the user intended to stop.
	return value.GoalHandbackRequired ||
		strings.TrimSpace(value.TargetAgentRoundID) != "" ||
		value.GoalSubstantiveOutput || value.GoalPublicEvidence
}

func roomPublicHandoffIsTerminal(status string) bool {
	switch strings.TrimSpace(status) {
	case roomPublicHandoffActionFinished,
		roomPublicHandoffActionError,
		roomPublicHandoffActionInterrupted:
		return true
	default:
		return false
	}
}

func roomPublicHandoffCanReplayStarted(value RoomPublicHandoff) bool {
	return value.WorkBinding != nil || value.ReviewBinding != nil ||
		protocol.NormalizeGoalCollaborationBinding(value.GoalCollaborationBinding) != nil
}

const roomPublicHandoffClaimTTL = 30 * time.Second

func (s *RoomPublicHandoffStore) transition(
	ownerUserID string,
	conversationID string,
	handoffID string,
	action string,
	mutate func(*RoomPublicHandoff),
	allowed func(RoomPublicHandoff) bool,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.replayLocked(ownerUserID, conversationID)
	if err != nil {
		return err
	}
	value, ok := all[strings.TrimSpace(handoffID)]
	if !ok {
		return nil
	}
	if !allowed(value) {
		return nil
	}
	mutate(&value)
	value.UpdatedAt = time.Now().UnixMilli()
	return s.appendLocked(ownerUserID, conversationID, action, value)
}

func (s *RoomPublicHandoffStore) appendLocked(
	ownerUserID string,
	conversationID string,
	action string,
	handoff RoomPublicHandoff,
) error {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return errors.New("conversation_id is required")
	}
	handoff.OwnerUserID = strings.TrimSpace(ownerUserID)
	handoff.ConversationID = conversationID
	return s.files.appendRoomJSONL(ownerUserID, s.paths.RoomPublicHandoffsPath(ownerUserID, conversationID), map[string]any{
		"action":    action,
		"handoff":   handoff,
		"timestamp": time.Now().UnixMilli(),
	})
}

func (s *RoomPublicHandoffStore) replayLocked(
	ownerUserID string,
	conversationID string,
) (map[string]RoomPublicHandoff, error) {
	rows, err := s.files.readRoomJSONL(
		ownerUserID,
		s.paths.RoomPublicHandoffsPath(ownerUserID, strings.TrimSpace(conversationID)),
	)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]RoomPublicHandoff{}, nil
	}
	if err != nil {
		return nil, err
	}
	result := replayRoomPublicHandoffRows(rows)
	for handoffID, value := range result {
		value.OwnerUserID = ownerUserID
		value.ConversationID = strings.TrimSpace(conversationID)
		result[handoffID] = value
	}
	return result, nil
}

func replayRoomPublicHandoffRows(rows []map[string]any) map[string]RoomPublicHandoff {
	result := make(map[string]RoomPublicHandoff)
	for _, row := range rows {
		payload, marshalErr := json.Marshal(row["handoff"])
		if marshalErr != nil {
			continue
		}
		var value RoomPublicHandoff
		if json.Unmarshal(payload, &value) != nil || strings.TrimSpace(value.HandoffID) == "" {
			continue
		}
		result[value.HandoffID] = value
	}
	return result
}

func validateRoomPublicHandoff(value RoomPublicHandoff) error {
	for name, field := range map[string]string{
		"handoff_id":        value.HandoffID,
		"conversation_id":   value.ConversationID,
		"source_message_id": value.SourceMessageID,
		"source_agent_id":   value.SourceAgentID,
		"target_agent_id":   value.TargetAgentID,
	} {
		if strings.TrimSpace(field) == "" {
			return errors.New(name + " is required")
		}
	}
	if value.WorkBinding != nil && value.ReviewBinding != nil {
		return errors.New("work_binding and review_binding are mutually exclusive")
	}
	if value.GoalCollaborationBinding != nil &&
		protocol.NormalizeGoalCollaborationBinding(value.GoalCollaborationBinding) == nil {
		return errors.New("goal_collaboration_binding requires goal_id and objective_revision")
	}
	if value.QueueSource != "" &&
		value.QueueSource != protocol.InputQueueSourceAgentPublicMention &&
		value.QueueSource != protocol.InputQueueSourceAgentRoomMessage {
		return errors.New("queue_source is invalid")
	}
	if value.GoalCollaborationBinding != nil &&
		value.QueueSource != protocol.InputQueueSourceAgentPublicMention &&
		value.QueueSource != protocol.InputQueueSourceAgentRoomMessage {
		return errors.New("goal_collaboration_binding requires a Room handoff queue_source")
	}
	if value.WorkBinding != nil || value.ReviewBinding != nil {
		if err := protocol.ValidateInputQueueCapabilityEnvelope(protocol.InputQueueItem{
			Scope:          protocol.InputQueueScopeRoom,
			AgentID:        value.TargetAgentID,
			TargetAgentIDs: []string{value.TargetAgentID},
			Source:         protocol.InputQueueSourceAgentRoomMessage,
			DeliveryPolicy: protocol.ChatDeliveryPolicyQueue,
			WorkBinding:    value.WorkBinding,
			ReviewBinding:  value.ReviewBinding,
		}); err != nil {
			return err
		}
	}
	return nil
}

func roomPublicHandoffCanStart(value RoomPublicHandoff) bool {
	switch value.Status {
	case roomPublicHandoffActionSourceFinished, roomPublicHandoffActionQueued:
		return true
	case roomPublicHandoffActionClaimed:
		return time.Now().UnixMilli()-value.ClaimedAt > roomPublicHandoffClaimTTL.Milliseconds()
	default:
		return false
	}
}

func roomPublicHandoffCanTerminal(value RoomPublicHandoff) bool {
	switch value.Status {
	case roomPublicHandoffActionFinished, roomPublicHandoffActionError, roomPublicHandoffActionInterrupted:
		return false
	default:
		return true
	}
}

func roomPublicHandoffCanCancelSource(value RoomPublicHandoff) bool {
	switch value.Status {
	case roomPublicHandoffActionDetected,
		roomPublicHandoffActionSourceFinished,
		roomPublicHandoffActionQueued,
		roomPublicHandoffActionClaimed:
		return true
	default:
		return false
	}
}

func roomPublicHandoffCanCancelRoot(value RoomPublicHandoff) bool {
	switch value.Status {
	case roomPublicHandoffActionDetected,
		roomPublicHandoffActionSourceFinished,
		roomPublicHandoffActionQueued,
		roomPublicHandoffActionClaimed,
		roomPublicHandoffActionStarted:
		return true
	default:
		return false
	}
}

func normalizeRoomPublicHandoffTerminalStatus(value string) string {
	switch strings.TrimSpace(value) {
	case "error":
		return roomPublicHandoffActionError
	case "interrupted", "cancelled":
		return roomPublicHandoffActionInterrupted
	default:
		return roomPublicHandoffActionFinished
	}
}

const (
	roomPublicHandoffActionFinished    = "finished"
	roomPublicHandoffActionError       = "error"
	roomPublicHandoffActionInterrupted = "interrupted"
)
