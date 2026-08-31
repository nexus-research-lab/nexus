// INPUT: 带 owner-scoped creation_request_id 的 Agent 创建意图、持久 claim 与 workspace 初始化阶段。
// OUTPUT: 同 request/digest 的 exact Agent 回放、pending/failed/deleted 墓碑与可识别的提交事实。
// POS: Agent 创建可恢复主链；旧无 request ID 创建仍由 crud.go 原路径处理。
package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"
)

const agentCreationClaimLease = 2 * time.Minute

var agentCreationRequestIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

var (
	ErrAgentCreationRequestInvalid  = errors.New("agent creation request id invalid")
	ErrAgentCreationRequestConflict = errors.New("agent creation request conflict")
	ErrAgentCreationPending         = errors.New("agent creation pending")
	ErrAgentCreationFailed          = errors.New("agent creation failed")
	ErrAgentCreationResultDeleted   = errors.New("agent creation result deleted")
	ErrAgentCreationOutcomeUnknown  = errors.New("agent creation outcome unknown")
)

// CreationReconcileError 表示 Agent 记录已提交，但返回投影或 runtime 同步未完成。
type CreationReconcileError struct {
	cause error
}

func (e *CreationReconcileError) Error() string {
	return fmt.Sprintf("agent creation committed but projection failed: %v", e.cause)
}

func (e *CreationReconcileError) Unwrap() error { return e.cause }

// AgentCreationCommitted 只依赖显式提交错误类型，不解析文案。
func AgentCreationCommitted(err error) bool {
	var committed *CreationReconcileError
	return errors.As(err, &committed)
}

type agentCreationLock struct {
	mu   sync.Mutex
	refs int
}

func (s *Service) createAgentIdempotent(
	ctx context.Context,
	request protocol.CreateRequest,
) (*protocol.Agent, error) {
	request.CreationRequestID = strings.TrimSpace(request.CreationRequestID)
	if !agentCreationRequestIDPattern.MatchString(request.CreationRequestID) {
		return nil, ErrAgentCreationRequestInvalid
	}
	validation := validateName(request.Name)
	if !validation.IsValid || !validation.IsAvailable {
		return nil, fmtAgentNameInvalid(validation.Reason)
	}
	digest, err := agentCreationIntentDigest(request, validation.NormalizedName)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAgentCreationRequestInvalid, err)
	}
	ownerUserID := effectiveOwnerUserID(ctx)
	unlock := s.lockAgentCreation(ownerUserID + "\x00" + request.CreationRequestID)
	defer unlock()

	now := time.Now().UTC()
	candidateAgentID := NewAgentID()
	candidate := agentrepo.CreationRequestRecord{
		OwnerUserID:       ownerUserID,
		CreationRequestID: request.CreationRequestID,
		IntentDigest:      digest,
		AgentID:           candidateAgentID,
		WorkspacePath:     ResolveWorkspacePath(s.config, ownerUserID, candidateAgentID),
		Status:            agentrepo.CreationRequestPending,
		Stage:             agentrepo.CreationRequestReserved,
		ClaimToken:        NewAgentID() + NewAgentID(),
		LeaseExpiresAtMS:  now.Add(agentCreationClaimLease).UnixMilli(),
	}
	claim, claimed, err := s.repository.ClaimAgentCreation(ctx, candidate, now.UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAgentCreationOutcomeUnknown, err)
	}
	if claim.IntentDigest != digest {
		return nil, ErrAgentCreationRequestConflict
	}
	switch claim.Status {
	case agentrepo.CreationRequestCommitted:
		return s.projectCommittedAgentCreation(ctx, claim)
	case agentrepo.CreationRequestDeleted:
		return nil, ErrAgentCreationResultDeleted
	case agentrepo.CreationRequestFailed:
		return nil, ErrAgentCreationFailed
	case agentrepo.CreationRequestPending:
		if !claimed {
			return nil, ErrAgentCreationPending
		}
	default:
		return nil, ErrAgentCreationOutcomeUnknown
	}

	if err = s.EnsureReady(ctx); err != nil {
		return nil, s.failClaimedAgentCreation(ctx, *claim, "agent.ensure_ready_failed", err)
	}
	workspaceAgent := protocol.Agent{
		AgentID:       claim.AgentID,
		OwnerUserID:   claim.OwnerUserID,
		Name:          validation.NormalizedName,
		WorkspacePath: claim.WorkspacePath,
		Status:        "active",
		CreatedAt:     now,
	}
	if claim.Stage == agentrepo.CreationRequestReserved {
		if err = s.prepareReservedAgentWorkspace(ctx, request, workspaceAgent); err != nil {
			return nil, s.failClaimedAgentCreation(ctx, *claim, "agent.workspace_initialization_failed", err)
		}
		claim, err = s.confirmAgentCreationWorkspacePrepared(ctx, *claim)
		if err != nil {
			return nil, err
		}
	} else if claim.Stage != agentrepo.CreationRequestWorkspacePrepared {
		return nil, ErrAgentCreationOutcomeUnknown
	}
	record := BuildCreateRecord(
		s.config,
		request,
		claim.OwnerUserID,
		validation.NormalizedName,
		claim.AgentID,
		claim.WorkspacePath,
		"active",
		false,
	)
	if err = s.repository.CommitAgentCreation(ctx, *claim, record); err != nil {
		return s.reconcileAgentCreationAfterCommitError(ctx, *claim, err)
	}
	return s.projectCommittedAgentCreation(ctx, claim)
}

func (s *Service) confirmAgentCreationWorkspacePrepared(
	ctx context.Context,
	claim agentrepo.CreationRequestRecord,
) (*agentrepo.CreationRequestRecord, error) {
	marked, markErr := s.repository.MarkAgentCreationWorkspacePrepared(ctx, claim)
	if marked && markErr == nil {
		claim.Stage = agentrepo.CreationRequestWorkspacePrepared
		return &claim, nil
	}
	reconcileCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	current, readErr := s.repository.GetAgentCreationRequest(
		reconcileCtx,
		claim.OwnerUserID,
		claim.CreationRequestID,
	)
	if readErr != nil || current == nil {
		return nil, fmt.Errorf(
			"%w: %v",
			ErrAgentCreationOutcomeUnknown,
			errors.Join(markErr, readErr),
		)
	}
	if current.Status == agentrepo.CreationRequestPending &&
		current.Stage == agentrepo.CreationRequestWorkspacePrepared &&
		current.ClaimToken == claim.ClaimToken {
		return current, nil
	}
	switch current.Status {
	case agentrepo.CreationRequestCommitted:
		item, projectErr := s.projectCommittedAgentCreation(reconcileCtx, current)
		if projectErr != nil {
			return nil, projectErr
		}
		return nil, &CreationReconcileError{cause: fmt.Errorf("Agent %s already committed", item.AgentID)}
	case agentrepo.CreationRequestDeleted:
		return nil, ErrAgentCreationResultDeleted
	case agentrepo.CreationRequestFailed:
		return nil, ErrAgentCreationFailed
	default:
		return nil, fmt.Errorf("%w: %v", ErrAgentCreationPending, markErr)
	}
}

// GetAgentCreationRequestResult 仅按当前 owner 与 exact request ID 对账，不创建或重放 Agent。
func (s *Service) GetAgentCreationRequestResult(
	ctx context.Context,
	creationRequestID string,
) (*protocol.AgentCreationRequestResult, error) {
	creationRequestID = strings.TrimSpace(creationRequestID)
	if !agentCreationRequestIDPattern.MatchString(creationRequestID) {
		return nil, ErrAgentCreationRequestInvalid
	}
	ownerUserID := effectiveOwnerUserID(ctx)
	record, err := s.repository.GetAgentCreationRequest(ctx, ownerUserID, creationRequestID)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return &protocol.AgentCreationRequestResult{
			CreationRequestID: creationRequestID,
			Status:            protocol.AgentCreationRequestNotFound,
		}, nil
	}
	result := &protocol.AgentCreationRequestResult{
		CreationRequestID: creationRequestID,
		Status:            protocol.AgentCreationRequestStatus(record.Status),
	}
	if record.Status != agentrepo.CreationRequestCommitted {
		return result, nil
	}
	item, projectErr := s.projectCommittedAgentCreation(ctx, record)
	result.Agent = item
	return result, projectErr
}

func (s *Service) prepareReservedAgentWorkspace(
	ctx context.Context,
	request protocol.CreateRequest,
	agentValue protocol.Agent,
) error {
	if err := ensureDirectoryWithinRoot(
		WorkspaceBasePath(s.config),
		agentValue.WorkspacePath,
		agentWorkspaceDirectoryMode(),
	); err != nil {
		return err
	}
	root, err := s.openAgentWorkspace(agentValue, false)
	if err != nil {
		return err
	}
	if err = ensureRuntimeEmotionStateAt(root); err == nil {
		err = writeProfileTemplateAt(root, request.ProfileTemplate)
	}
	closeErr := root.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return s.initializeAgentWorkspace(ctx, agentValue)
}

func (s *Service) projectCommittedAgentCreation(
	ctx context.Context,
	claim *agentrepo.CreationRequestRecord,
) (*protocol.Agent, error) {
	item, err := s.repository.GetAgent(ctx, claim.AgentID, claim.OwnerUserID)
	if err != nil {
		return nil, &CreationReconcileError{cause: err}
	}
	if item == nil || item.Status != "active" {
		latest, readErr := s.repository.GetAgentCreationRequest(
			ctx,
			claim.OwnerUserID,
			claim.CreationRequestID,
		)
		if readErr == nil && latest != nil && latest.Status == agentrepo.CreationRequestDeleted {
			return nil, ErrAgentCreationResultDeleted
		}
		if readErr != nil {
			return nil, &CreationReconcileError{cause: readErr}
		}
		return nil, &CreationReconcileError{cause: ErrAgentNotFound}
	}
	normalizeManagedSemanticSkillBinding(item)
	normalizeAgentAvatar(item)
	if err = s.ensureAgentRuntimeState(*item); err == nil {
		err = s.enrichAgentWithSkillsCount(item)
	}
	if err != nil {
		return item, &CreationReconcileError{cause: err}
	}
	return item, nil
}

func (s *Service) reconcileAgentCreationAfterCommitError(
	ctx context.Context,
	claim agentrepo.CreationRequestRecord,
	commitErr error,
) (*protocol.Agent, error) {
	current, err := s.repository.GetAgentCreationRequest(
		ctx,
		claim.OwnerUserID,
		claim.CreationRequestID,
	)
	if err != nil || current == nil {
		return nil, fmt.Errorf("%w: %v", ErrAgentCreationOutcomeUnknown, errors.Join(commitErr, err))
	}
	switch current.Status {
	case agentrepo.CreationRequestCommitted:
		return s.projectCommittedAgentCreation(ctx, current)
	case agentrepo.CreationRequestDeleted:
		return nil, ErrAgentCreationResultDeleted
	case agentrepo.CreationRequestFailed:
		return nil, ErrAgentCreationFailed
	case agentrepo.CreationRequestPending:
		return nil, fmt.Errorf("%w: %v", ErrAgentCreationPending, commitErr)
	default:
		return nil, fmt.Errorf("%w: %v", ErrAgentCreationOutcomeUnknown, commitErr)
	}
}

func (s *Service) failClaimedAgentCreation(
	ctx context.Context,
	claim agentrepo.CreationRequestRecord,
	failureCode string,
	cause error,
) error {
	reconcileCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	marked, err := s.repository.FailAgentCreation(reconcileCtx, claim, failureCode)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrAgentCreationOutcomeUnknown, errors.Join(cause, err))
	}
	if !marked {
		current, readErr := s.repository.GetAgentCreationRequest(
			reconcileCtx,
			claim.OwnerUserID,
			claim.CreationRequestID,
		)
		if readErr != nil || current == nil {
			return fmt.Errorf("%w: %v", ErrAgentCreationOutcomeUnknown, errors.Join(cause, readErr))
		}
		switch current.Status {
		case agentrepo.CreationRequestCommitted:
			return &CreationReconcileError{cause: cause}
		case agentrepo.CreationRequestDeleted:
			return ErrAgentCreationResultDeleted
		case agentrepo.CreationRequestFailed:
			return fmt.Errorf("%w: %v", ErrAgentCreationFailed, cause)
		default:
			return fmt.Errorf("%w: %v", ErrAgentCreationPending, cause)
		}
	}
	// 只有失败墓碑已明确提交，后续请求不可能再认领该路径时才清理。
	_ = s.cleanupAgentWorkspace(reconcileCtx, protocol.Agent{
		AgentID:       claim.AgentID,
		OwnerUserID:   claim.OwnerUserID,
		WorkspacePath: claim.WorkspacePath,
	})
	return fmt.Errorf("%w: %v", ErrAgentCreationFailed, cause)
}

func agentCreationIntentDigest(request protocol.CreateRequest, normalizedName string) (string, error) {
	options := defaultAgentOptions(false)
	if request.Options != nil {
		options = mergeOptions(options, *request.Options)
	}
	options.SkillIDs, options.DisabledSkillIDs = BindManagedSemanticSkills(
		options.SkillIDs,
		options.DisabledSkillIDs,
	)
	payload := struct {
		Name            string           `json:"name"`
		Options         protocol.Options `json:"options"`
		Avatar          string           `json:"avatar"`
		Description     string           `json:"description"`
		ProfileTemplate string           `json:"profile_template"`
		VibeTags        []string         `json:"vibe_tags"`
	}{
		Name:            normalizedName,
		Options:         options,
		Avatar:          strings.TrimSpace(request.Avatar),
		Description:     request.Description,
		ProfileTemplate: request.ProfileTemplate,
		VibeTags:        request.VibeTags,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func (s *Service) lockAgentCreation(key string) func() {
	s.creationLocksMu.Lock()
	if s.creationLocks == nil {
		s.creationLocks = make(map[string]*agentCreationLock)
	}
	lock := s.creationLocks[key]
	if lock == nil {
		lock = &agentCreationLock{}
		s.creationLocks[key] = lock
	}
	lock.refs++
	s.creationLocksMu.Unlock()

	lock.mu.Lock()
	return func() {
		lock.mu.Unlock()
		s.creationLocksMu.Lock()
		lock.refs--
		if lock.refs == 0 {
			delete(s.creationLocks, key)
		}
		s.creationLocksMu.Unlock()
	}
}
