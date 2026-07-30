// INPUT: owner-scoped Room contexts、数据库会话索引与 workspace 持久化证据。
// OUTPUT: 空 conversation 的 dry-run 计划，或经二次判定后的正式删除报告。
// POS: 历史重复空白页维护；只编排判定，实际删除复用 DeleteConversation。
package room

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const (
	emptyConversationStateConfirmed = "confirmed_empty"
	emptyConversationStateOccupied  = "occupied"
	emptyConversationStateUnknown   = "unknown"

	emptyConversationActionKeep         = "keep"
	emptyConversationActionWouldDelete  = "would_delete"
	emptyConversationActionDeleted      = "deleted"
	emptyConversationActionPreserve     = "preserve"
	emptyConversationActionSkipUnknown  = "skip_unknown"
	emptyConversationActionDeleteFailed = "delete_failed"
)

// PruneEmptyConversationsOptions 描述历史空 conversation 清理范围。
type PruneEmptyConversationsOptions struct {
	RoomID string
	Apply  bool
}

// EmptyConversationPruneItem 描述单个 conversation 的判定与动作。
type EmptyConversationPruneItem struct {
	RoomID           string    `json:"room_id"`
	RoomName         string    `json:"room_name,omitempty"`
	ConversationID   string    `json:"conversation_id"`
	ConversationType string    `json:"conversation_type,omitempty"`
	Title            string    `json:"title,omitempty"`
	IsDraft          bool      `json:"is_draft"`
	CreatedAt        time.Time `json:"created_at,omitempty"`
	State            string    `json:"state"`
	Action           string    `json:"action"`
	Reasons          []string  `json:"reasons,omitempty"`
}

// EmptyConversationDraftRepair 描述清理后唯一 draft 指针的兼容修复。
type EmptyConversationDraftRepair struct {
	RoomID                       string   `json:"room_id"`
	KeeperConversationID         string   `json:"keeper_conversation_id,omitempty"`
	PreviousDraftConversationIDs []string `json:"previous_draft_conversation_ids,omitempty"`
	Action                       string   `json:"action"`
	Error                        string   `json:"error,omitempty"`
}

// EmptyConversationPruneReport 是 nexusctl 可直接序列化的维护报告。
type EmptyConversationPruneReport struct {
	Applied              bool                           `json:"applied"`
	ScopeRoomID          string                         `json:"scope_room_id,omitempty"`
	RoomsScanned         int                            `json:"rooms_scanned"`
	ConversationsScanned int                            `json:"conversations_scanned"`
	ConfirmedEmpty       int                            `json:"confirmed_empty"`
	Kept                 int                            `json:"kept"`
	WouldDelete          int                            `json:"would_delete"`
	Deleted              int                            `json:"deleted"`
	Occupied             int                            `json:"occupied"`
	Unknown              int                            `json:"unknown"`
	DeleteFailed         int                            `json:"delete_failed"`
	Items                []EmptyConversationPruneItem   `json:"items"`
	DraftRepairs         []EmptyConversationDraftRepair `json:"draft_repairs,omitempty"`
	DraftRepairFailed    int                            `json:"draft_repair_failed"`
	Warnings             []string                       `json:"warnings,omitempty"`
}

type emptyConversationInspection struct {
	state   string
	reasons []string
}

type emptyConversationRoomPlan struct {
	items      []EmptyConversationPruneItem
	candidates []protocol.ConversationContextAggregate
	keeper     *protocol.ConversationContextAggregate
}

// PruneEmptyConversations 规划或执行 owner scope 内的历史空 conversation 清理。
//
// 默认只返回计划。Apply 模式会在每次删除前重新加载 Room、重新计算“最新空白页”
// 并再次检查目标证据，然后复用 DeleteConversation 完成 runtime、文件和 Goal 清理。
func (s *Service) PruneEmptyConversations(
	ctx context.Context,
	options PruneEmptyConversationsOptions,
) (EmptyConversationPruneReport, error) {
	report := EmptyConversationPruneReport{
		Applied:     options.Apply,
		ScopeRoomID: strings.TrimSpace(options.RoomID),
		Items:       []EmptyConversationPruneItem{},
	}

	roomIDs, err := s.emptyConversationPruneRoomIDs(ctx, report.ScopeRoomID)
	if err != nil {
		return report, err
	}
	for _, roomID := range roomIDs {
		report.RoomsScanned++
		if options.Apply {
			s.applyEmptyConversationRoomPlan(ctx, roomID, &report)
			continue
		}
		contexts, loadErr := s.GetRoomContexts(ctx, roomID)
		if loadErr != nil {
			if report.ScopeRoomID != "" {
				return report, loadErr
			}
			report.Warnings = append(
				report.Warnings,
				fmt.Sprintf("room %s: load contexts: %v", roomID, loadErr),
			)
			continue
		}
		report.ConversationsScanned += len(contexts)
		plan := s.buildEmptyConversationRoomPlan(ctx, contexts)
		report.Items = append(report.Items, plan.items...)
		if repair, needed := planEmptyConversationDraftRepair(contexts, plan.keeperID(), false); needed {
			report.DraftRepairs = append(report.DraftRepairs, repair)
		}
	}
	report.recount()
	return report, nil
}

func (s *Service) emptyConversationPruneRoomIDs(ctx context.Context, roomID string) ([]string, error) {
	if roomID != "" {
		if _, err := s.GetRoom(ctx, roomID); err != nil {
			return nil, err
		}
		return []string{roomID}, nil
	}

	// 维护命令需要覆盖 owner scope 的全部 Room；不复用 CLI 列表的展示上限。
	rooms, err := s.ListRooms(ctx, int(^uint(0)>>1))
	if err != nil {
		return nil, err
	}
	roomIDs := make([]string, 0, len(rooms))
	for _, roomValue := range rooms {
		if normalized := strings.TrimSpace(roomValue.Room.ID); normalized != "" {
			roomIDs = append(roomIDs, normalized)
		}
	}
	return roomIDs, nil
}

func (s *Service) applyEmptyConversationRoomPlan(
	ctx context.Context,
	roomID string,
	report *EmptyConversationPruneReport,
) {
	initialContexts, err := s.GetRoomContexts(ctx, roomID)
	if err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("room %s: load contexts: %v", roomID, err))
		return
	}
	report.ConversationsScanned += len(initialContexts)

	deletedItems := make([]EmptyConversationPruneItem, 0)
	failedByConversationID := make(map[string]string)
	for {
		// 每轮都重新加载并重新选 keeper，避免旧计划删掉此刻最新的空白页。
		contexts, loadErr := s.GetRoomContexts(ctx, roomID)
		if loadErr != nil {
			report.Warnings = append(report.Warnings, fmt.Sprintf("room %s: reload contexts: %v", roomID, loadErr))
			break
		}
		plan := s.buildEmptyConversationRoomPlan(ctx, contexts)
		if len(plan.candidates) == 0 {
			break
		}

		// candidates 按新到旧排列；先删最旧项，让 keeper 在整个过程中保持稳定。
		target := plan.candidates[len(plan.candidates)-1]
		rechecked := s.inspectEmptyConversation(ctx, target)
		if rechecked.state != emptyConversationStateConfirmed {
			continue
		}

		item := emptyConversationPruneItem(target, rechecked)
		item.Action = emptyConversationActionDeleted
		_, deleteErr := s.DeleteConversation(ctx, roomID, target.Conversation.ID)
		if deleteErr == nil {
			deletedItems = append(deletedItems, item)
			continue
		}

		// DeleteConversation 先提交数据库事务，再清理 runtime/文件/Goal。
		// 报错后必须重查，不能把已删除项当成仍然存在，也不能盲目重试。
		remaining, reloadErr := s.GetRoomContexts(ctx, roomID)
		if reloadErr == nil && !hasConversationContext(remaining, target.Conversation.ID) {
			item.Reasons = append(item.Reasons, "post_delete_cleanup_warning: "+deleteErr.Error())
			deletedItems = append(deletedItems, item)
			report.Warnings = append(
				report.Warnings,
				fmt.Sprintf("conversation %s deleted with cleanup warning: %v", target.Conversation.ID, deleteErr),
			)
			continue
		}

		failedByConversationID[target.Conversation.ID] = deleteErr.Error()
		report.Warnings = append(
			report.Warnings,
			fmt.Sprintf("conversation %s delete failed: %v", target.Conversation.ID, deleteErr),
		)
		break
	}

	finalContexts, err := s.GetRoomContexts(ctx, roomID)
	if err != nil {
		report.Items = append(report.Items, deletedItems...)
		report.Warnings = append(report.Warnings, fmt.Sprintf("room %s: final verification: %v", roomID, err))
		return
	}
	finalPlan := s.buildEmptyConversationRoomPlan(ctx, finalContexts)
	desiredDraftConversationID := finalPlan.keeperID()
	if finalPlan.keeper != nil {
		rechecked := s.inspectEmptyConversation(ctx, *finalPlan.keeper)
		if rechecked.state != emptyConversationStateConfirmed {
			desiredDraftConversationID = ""
		}
	}
	if repair, needed := planEmptyConversationDraftRepair(
		finalContexts,
		desiredDraftConversationID,
		true,
	); needed {
		if repairErr := s.repository.SetRoomDraftConversation(
			ctx,
			authctx.OwnerUserID(ctx),
			roomID,
			desiredDraftConversationID,
		); repairErr != nil {
			repair.Action = "failed"
			repair.Error = repairErr.Error()
			report.DraftRepairFailed++
			report.Warnings = append(
				report.Warnings,
				fmt.Sprintf("room %s draft repair failed: %v", roomID, repairErr),
			)
		} else if desiredDraftConversationID == "" {
			repair.Action = "cleared"
		} else {
			repair.Action = "set"
		}
		report.DraftRepairs = append(report.DraftRepairs, repair)

		reloaded, reloadErr := s.GetRoomContexts(ctx, roomID)
		if reloadErr != nil {
			report.Warnings = append(report.Warnings, fmt.Sprintf("room %s: reload after draft repair: %v", roomID, reloadErr))
		} else {
			finalContexts = reloaded
			finalPlan = s.buildEmptyConversationRoomPlan(ctx, finalContexts)
		}
	}
	for index := range finalPlan.items {
		if failure := failedByConversationID[finalPlan.items[index].ConversationID]; failure != "" {
			finalPlan.items[index].Action = emptyConversationActionDeleteFailed
			finalPlan.items[index].Reasons = append(finalPlan.items[index].Reasons, "delete_failed: "+failure)
		}
	}
	report.Items = append(report.Items, deletedItems...)
	report.Items = append(report.Items, finalPlan.items...)
}

func (s *Service) buildEmptyConversationRoomPlan(
	ctx context.Context,
	contexts []protocol.ConversationContextAggregate,
) emptyConversationRoomPlan {
	ordered := slices.Clone(contexts)
	sort.SliceStable(ordered, func(left int, right int) bool {
		leftCreated := ordered[left].Conversation.CreatedAt
		rightCreated := ordered[right].Conversation.CreatedAt
		if !leftCreated.Equal(rightCreated) {
			return leftCreated.After(rightCreated)
		}
		return ordered[left].Conversation.ID > ordered[right].Conversation.ID
	})

	plan := emptyConversationRoomPlan{
		items:      make([]EmptyConversationPruneItem, 0, len(ordered)),
		candidates: make([]protocol.ConversationContextAggregate, 0),
	}
	emptySeen := false
	for _, contextValue := range ordered {
		inspection := s.inspectEmptyConversation(ctx, contextValue)
		item := emptyConversationPruneItem(contextValue, inspection)
		switch inspection.state {
		case emptyConversationStateConfirmed:
			if !emptySeen {
				item.Action = emptyConversationActionKeep
				emptySeen = true
				keeper := contextValue
				plan.keeper = &keeper
			} else {
				item.Action = emptyConversationActionWouldDelete
				plan.candidates = append(plan.candidates, contextValue)
			}
		case emptyConversationStateOccupied:
			item.Action = emptyConversationActionPreserve
		default:
			item.Action = emptyConversationActionSkipUnknown
		}
		plan.items = append(plan.items, item)
	}
	return plan
}

func (p emptyConversationRoomPlan) keeperID() string {
	if p.keeper == nil {
		return ""
	}
	return strings.TrimSpace(p.keeper.Conversation.ID)
}

func planEmptyConversationDraftRepair(
	contexts []protocol.ConversationContextAggregate,
	keeperConversationID string,
	applied bool,
) (EmptyConversationDraftRepair, bool) {
	if len(contexts) == 0 {
		return EmptyConversationDraftRepair{}, false
	}
	previous := make([]string, 0)
	for _, contextValue := range contexts {
		if contextValue.Conversation.IsDraft {
			previous = append(previous, contextValue.Conversation.ID)
		}
	}
	sort.Strings(previous)
	keeperConversationID = strings.TrimSpace(keeperConversationID)
	if (keeperConversationID == "" && len(previous) == 0) ||
		(keeperConversationID != "" && len(previous) == 1 && previous[0] == keeperConversationID) {
		return EmptyConversationDraftRepair{}, false
	}
	action := "would_set"
	if keeperConversationID == "" {
		action = "would_clear"
	}
	if applied {
		action = "pending"
	}
	return EmptyConversationDraftRepair{
		RoomID:                       contexts[0].Room.ID,
		KeeperConversationID:         keeperConversationID,
		PreviousDraftConversationIDs: previous,
		Action:                       action,
	}, true
}

func (s *Service) inspectEmptyConversation(
	ctx context.Context,
	contextValue protocol.ConversationContextAggregate,
) emptyConversationInspection {
	occupied := make([]string, 0)
	unknown := make([]string, 0)
	conversationID := strings.TrimSpace(contextValue.Conversation.ID)
	if conversationID == "" || strings.TrimSpace(contextValue.Room.ID) == "" {
		unknown = append(unknown, "invalid_room_conversation_identity")
	}
	if contextValue.Conversation.MessageCount < 0 {
		unknown = append(unknown, "invalid_database_message_count")
	} else if contextValue.Conversation.MessageCount > 0 {
		occupied = append(occupied, "database_messages_present")
	}

	memberByAgentID := make(map[string]protocol.Agent, len(contextValue.MemberAgents))
	for _, memberAgent := range contextValue.MemberAgents {
		agentID := strings.TrimSpace(memberAgent.AgentID)
		if agentID == "" {
			unknown = append(unknown, "member_agent_id_missing")
			continue
		}
		memberByAgentID[agentID] = memberAgent
	}
	sessionAgents := make(map[string]struct{}, len(contextValue.Sessions))
	for _, sessionValue := range contextValue.Sessions {
		agentID := strings.TrimSpace(sessionValue.AgentID)
		if agentID == "" {
			unknown = append(unknown, "session_agent_id_missing")
			continue
		}
		sessionAgents[agentID] = struct{}{}
		if strings.TrimSpace(sessionValue.SDKSessionID) != "" {
			occupied = append(occupied, "sdk_session_present:"+agentID)
		}
		if _, exists := memberByAgentID[agentID]; !exists {
			unknown = append(unknown, "session_member_missing:"+agentID)
		}
	}
	if len(contextValue.Sessions) == 0 {
		unknown = append(unknown, "database_sessions_missing")
	}
	for agentID := range memberByAgentID {
		if _, exists := sessionAgents[agentID]; !exists {
			unknown = append(unknown, "member_session_index_missing:"+agentID)
		}
	}

	hasUserInput, err := s.conversationHasCanonicalUserInput(ctx, contextValue, memberByAgentID)
	if err != nil {
		unknown = append(unknown, "canonical_history_probe_failed:"+err.Error())
	} else if hasUserInput {
		occupied = append(occupied, "canonical_user_input_present")
	}

	if conversationID != "" {
		exists, err := s.files.RoomConversationArtifactsExist(
			contextValue.Room.OwnerUserID,
			conversationID,
		)
		if err != nil {
			unknown = append(unknown, "room_artifact_probe_failed:"+err.Error())
		} else if exists {
			occupied = append(occupied, "room_conversation_artifacts_present")
		}
	}

	for agentID, memberAgent := range memberByAgentID {
		workspacePath := strings.TrimSpace(memberAgent.WorkspacePath)
		if workspacePath == "" {
			resolved, err := s.resolveAgentWorkspacePath(
				ctx,
				contextValue.Room.OwnerUserID,
				agentID,
			)
			if err != nil {
				unknown = append(unknown, "agent_workspace_resolve_failed:"+agentID+":"+err.Error())
				continue
			}
			workspacePath = resolved
		}
		if conversationID == "" {
			continue
		}
		sessionKey := protocol.BuildRoomAgentSessionKey(
			conversationID,
			agentID,
			contextValue.Room.RoomType,
		)
		exists, err := s.files.ForOwner(contextValue.Room.OwnerUserID).
			SessionArtifactsExist(workspacePath, sessionKey)
		if err != nil {
			unknown = append(unknown, "agent_session_artifact_probe_failed:"+agentID+":"+err.Error())
		} else if exists {
			occupied = append(occupied, "agent_session_artifacts_present:"+agentID)
		}
	}

	if conversationID != "" {
		hasReferences, err := s.repository.HasConversationReferences(
			ctx,
			authctx.OwnerUserID(ctx),
			contextValue.Room.ID,
			conversationID,
			canonicalRoomConversationSessionKeys(contextValue),
		)
		if err != nil {
			unknown = append(unknown, "persistent_reference_probe_failed:"+err.Error())
		} else if hasReferences {
			occupied = append(occupied, "persistent_conversation_reference_present")
		}
	}

	if s.goals != nil {
		if s.goalReader == nil {
			unknown = append(unknown, "goal_evidence_reader_unavailable")
		} else if hasGoal, err := s.goalReader.HasGoalForRoomConversation(ctx, conversationID); err != nil {
			unknown = append(unknown, "goal_evidence_probe_failed:"+err.Error())
		} else if hasGoal {
			occupied = append(occupied, "goal_present")
		}
	}

	occupied = uniqueSortedStrings(occupied)
	unknown = uniqueSortedStrings(unknown)
	if len(occupied) > 0 {
		return emptyConversationInspection{
			state:   emptyConversationStateOccupied,
			reasons: append(occupied, unknown...),
		}
	}
	if len(unknown) > 0 {
		return emptyConversationInspection{state: emptyConversationStateUnknown, reasons: unknown}
	}
	return emptyConversationInspection{
		state:   emptyConversationStateConfirmed,
		reasons: []string{"all_empty_evidence_confirmed"},
	}
}

func (s *Service) conversationHasCanonicalUserInput(
	ctx context.Context,
	contextValue protocol.ConversationContextAggregate,
	memberByAgentID map[string]protocol.Agent,
) (bool, error) {
	conversationID := strings.TrimSpace(contextValue.Conversation.ID)
	if conversationID == "" {
		return false, errors.New("conversation id is empty")
	}
	if contextValue.Room.RoomType != protocol.RoomTypeDM {
		if s.roomHistory == nil {
			return false, errors.New("room history reader is unavailable")
		}
		return s.roomHistory.HasCanonicalUserInput(
			contextValue.Room.OwnerUserID,
			conversationID,
		)
	}

	primarySession := primaryConversationSession(contextValue.Sessions)
	if primarySession == nil || strings.TrimSpace(primarySession.AgentID) == "" {
		return false, errors.New("primary dm session is missing")
	}
	agentID := strings.TrimSpace(primarySession.AgentID)
	memberAgent, exists := memberByAgentID[agentID]
	if !exists {
		return false, fmt.Errorf("dm member %s is missing", agentID)
	}
	workspacePath := strings.TrimSpace(memberAgent.WorkspacePath)
	if workspacePath == "" {
		resolved, err := s.resolveAgentWorkspacePath(
			ctx,
			contextValue.Room.OwnerUserID,
			agentID,
		)
		if err != nil {
			return false, err
		}
		workspacePath = resolved
	}
	sessionKey := protocol.BuildRoomAgentSessionKey(
		conversationID,
		agentID,
		protocol.RoomTypeDM,
	)
	if s.history == nil {
		return false, errors.New("agent history reader is unavailable")
	}
	return s.history.ForOwner(contextValue.Room.OwnerUserID).
		HasCanonicalUserInput(workspacePath, sessionKey)
}

func primaryConversationSession(sessions []protocol.SessionRecord) *protocol.SessionRecord {
	for index := range sessions {
		if sessions[index].IsPrimary {
			return &sessions[index]
		}
	}
	if len(sessions) == 0 {
		return nil
	}
	return &sessions[0]
}

func emptyConversationPruneItem(
	contextValue protocol.ConversationContextAggregate,
	inspection emptyConversationInspection,
) EmptyConversationPruneItem {
	return EmptyConversationPruneItem{
		RoomID:           contextValue.Room.ID,
		RoomName:         contextValue.Room.Name,
		ConversationID:   contextValue.Conversation.ID,
		ConversationType: contextValue.Conversation.ConversationType,
		Title:            contextValue.Conversation.Title,
		IsDraft:          contextValue.Conversation.IsDraft,
		CreatedAt:        contextValue.Conversation.CreatedAt,
		State:            inspection.state,
		Reasons:          slices.Clone(inspection.reasons),
	}
}

func canonicalRoomConversationSessionKeys(
	contextValue protocol.ConversationContextAggregate,
) []string {
	conversationID := strings.TrimSpace(contextValue.Conversation.ID)
	if conversationID == "" {
		return nil
	}
	keys := []string{protocol.BuildRoomSharedSessionKey(conversationID)}
	agentIDs := make(map[string]struct{}, len(contextValue.MemberAgents)+len(contextValue.Sessions))
	for _, memberAgent := range contextValue.MemberAgents {
		if agentID := strings.TrimSpace(memberAgent.AgentID); agentID != "" {
			agentIDs[agentID] = struct{}{}
		}
	}
	for _, sessionValue := range contextValue.Sessions {
		if agentID := strings.TrimSpace(sessionValue.AgentID); agentID != "" {
			agentIDs[agentID] = struct{}{}
		}
	}
	for agentID := range agentIDs {
		// Group and DM are both current Room member execution encodings. Probe
		// both so historical type changes cannot hide a typed reference.
		keys = append(
			keys,
			protocol.BuildRoomAgentSessionKey(conversationID, agentID, protocol.RoomTypeGroup),
			protocol.BuildRoomAgentSessionKey(conversationID, agentID, protocol.RoomTypeDM),
		)
	}
	return uniqueSortedStrings(keys)
}

func hasConversationContext(contexts []protocol.ConversationContextAggregate, conversationID string) bool {
	for _, contextValue := range contexts {
		if contextValue.Conversation.ID == conversationID {
			return true
		}
	}
	return false
}

func uniqueSortedStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	sort.Strings(values)
	result := values[:0]
	for _, value := range values {
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}
	return result
}

func (r *EmptyConversationPruneReport) recount() {
	r.ConfirmedEmpty = 0
	r.Kept = 0
	r.WouldDelete = 0
	r.Deleted = 0
	r.Occupied = 0
	r.Unknown = 0
	r.DeleteFailed = 0
	for _, item := range r.Items {
		switch item.State {
		case emptyConversationStateConfirmed:
			r.ConfirmedEmpty++
		case emptyConversationStateOccupied:
			r.Occupied++
		case emptyConversationStateUnknown:
			r.Unknown++
		}
		switch item.Action {
		case emptyConversationActionKeep:
			r.Kept++
		case emptyConversationActionWouldDelete:
			r.WouldDelete++
		case emptyConversationActionDeleted:
			r.Deleted++
		case emptyConversationActionDeleteFailed:
			r.DeleteFailed++
		}
	}
}
