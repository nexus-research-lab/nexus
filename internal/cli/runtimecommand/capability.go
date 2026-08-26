// INPUT: 宿主验证过的 runtime round、随机 capability 与 runtime manager 活跃 round 事实。
// OUTPUT: 只在唯一活跃 physical round 中可解析的 Goal/Execution/Automation command Actor。
// POS: nexus runtime command broker 的身份防伪边界；CLI 参数和环境变量不能声明 Actor 或责任绑定。
package runtimecommand

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

const (
	capabilityTTL           = 24 * time.Hour
	capabilityPurgeInterval = time.Hour
)

type RoundResolver interface {
	GetRunningRoundIDs(string) []string
}

// RoundContext 是一个 physical round 的完整 command identity 与共享动态 authority。
type RoundContext struct {
	SessionKey         string
	RoundID            string
	SourceContextType  string
	SourceContextID    string
	SourceContextLabel string
	CommandContext     runtimectx.RuntimeCommandContext
	Receipts           *ReceiptState
	Resources          *RoundResources
	Attempts           *AttemptState
}

// Actor 是宿主为一个 physical round 固定的 command 调用身份。
type Actor struct {
	OwnerUserID        string
	AgentID            string
	AgentName          string
	WorkspacePath      string
	SessionKey         string
	SessionLabel       string
	RoundID            string
	LeaseSessionKey    string
	LeaseRoundID       string
	SourceContextType  string
	SourceContextID    string
	SourceContextLabel string
	DefaultTimezone    string
	IsMainAgent        bool
	CurrentJobID       string
	CurrentRunID       string
	Round              RoundContext

	// GoalMutationAuthority 可为持久 Goal owner 的私有 Goal-only snapshot；它
	// 不得反向成为 Execution authority。
	GoalMutationAuthority   *runtimectx.GoalAuthorityState
	GoalResponsibilityState *runtimectx.ResponsibilityAuthorityState
}

func (a Actor) normalized() Actor {
	result := a
	result.OwnerUserID = strings.TrimSpace(result.OwnerUserID)
	result.AgentID = strings.TrimSpace(result.AgentID)
	result.AgentName = strings.TrimSpace(result.AgentName)
	result.WorkspacePath = strings.TrimSpace(result.WorkspacePath)
	result.SessionKey = strings.TrimSpace(result.SessionKey)
	result.SessionLabel = strings.TrimSpace(result.SessionLabel)
	result.RoundID = strings.TrimSpace(result.RoundID)
	result.LeaseSessionKey = strings.TrimSpace(result.LeaseSessionKey)
	result.LeaseRoundID = strings.TrimSpace(result.LeaseRoundID)
	result.SourceContextType = strings.ToLower(strings.TrimSpace(result.SourceContextType))
	result.SourceContextID = strings.TrimSpace(result.SourceContextID)
	result.SourceContextLabel = strings.TrimSpace(result.SourceContextLabel)
	result.DefaultTimezone = strings.TrimSpace(result.DefaultTimezone)
	result.CurrentJobID = strings.TrimSpace(result.CurrentJobID)
	result.CurrentRunID = strings.TrimSpace(result.CurrentRunID)
	return result
}

func (a Actor) valid() bool {
	value := a.normalized()
	return value.OwnerUserID != "" && value.AgentID != "" &&
		value.SessionKey != "" && value.RoundID != "" &&
		value.LeaseSessionKey != "" && value.LeaseRoundID != ""
}

func (a Actor) Valid() bool { return a.valid() }

// MutationAllowed 只允许可信交互来源修改 Automation；Goal/Execution 另由各自
// exact authority 与 Plan Mode 门禁决定。
func (a Actor) MutationAllowed() bool {
	if strings.TrimSpace(a.CurrentJobID) != "" || strings.TrimSpace(a.CurrentRunID) != "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(a.SourceContextType)) {
	case "agent", "agent_paired", "room":
		return true
	default:
		return false
	}
}

func (a Actor) CrossAgentAllowed() bool {
	return a.IsMainAgent && strings.TrimSpace(a.SourceContextType) == "agent" && a.MutationAllowed()
}

func (a Actor) AutomationRun() *protocol.AutomationRunContext {
	return a.Round.CommandContext.AutomationRun
}

type capabilityActor struct {
	actor     Actor
	expiresAt time.Time
}

type capabilityRecord struct {
	sessionKey      string
	leaseSessionKey string
	actors          map[string]capabilityActor
}

// Registry 持有 runtime command capability；它不依赖任何业务 service。
type Registry struct {
	mu          sync.Mutex
	records     map[string]*capabilityRecord
	tokens      map[string]string
	rounds      RoundResolver
	now         func() time.Time
	nextPurgeAt time.Time
}

func NewRegistry(rounds RoundResolver) *Registry {
	return &Registry{
		records: make(map[string]*capabilityRecord),
		tokens:  make(map[string]string),
		rounds:  rounds,
		now:     time.Now,
	}
}

func (r *Registry) Issue(actor Actor) (string, error) {
	if r == nil {
		return "", errors.New("runtime command capability 尚未装配")
	}
	actor = actor.normalized()
	if !actor.valid() {
		return "", errors.New("runtime command capability 缺少可信 owner、Agent、Session 或 round lease")
	}
	if actor.Round.Receipts == nil {
		return "", errors.New("runtime command capability 缺少 typed receipt state")
	}
	now := r.now()
	sessionKey := strings.Join([]string{actor.OwnerUserID, actor.AgentID, actor.LeaseSessionKey}, "\x00")
	r.mu.Lock()
	defer r.mu.Unlock()
	r.purgeDueLocked(now)
	token := r.tokens[sessionKey]
	record := r.records[token]
	if record != nil {
		r.purgeRecordActorsLocked(record, now)
		if len(record.actors) == 0 {
			delete(r.records, token)
			delete(r.tokens, record.sessionKey)
			token = ""
			record = nil
		}
	}
	if record == nil {
		var err error
		token, err = newCapabilityToken()
		if err != nil {
			return "", err
		}
		record = &capabilityRecord{
			sessionKey: sessionKey, leaseSessionKey: actor.LeaseSessionKey,
			actors: make(map[string]capabilityActor),
		}
		r.records[token] = record
		r.tokens[sessionKey] = token
	}
	record.actors[actor.LeaseRoundID] = capabilityActor{actor: actor, expiresAt: now.Add(capabilityTTL)}
	return token, nil
}

func (r *Registry) Resolve(token string) (Actor, error) {
	if r == nil {
		return Actor{}, errors.New("runtime command capability 尚未装配")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return Actor{}, errors.New("runtime command capability 不能为空")
	}
	now := r.now()
	r.mu.Lock()
	record := r.records[token]
	resolver := r.rounds
	if record != nil {
		r.purgeRecordActorsLocked(record, now)
		if len(record.actors) == 0 {
			delete(r.records, token)
			delete(r.tokens, record.sessionKey)
			record = nil
		}
	}
	leaseSessionKey := ""
	if record != nil {
		leaseSessionKey = record.leaseSessionKey
	}
	r.mu.Unlock()
	if record == nil || resolver == nil {
		return Actor{}, errors.New("runtime command capability 无效或尚未装配")
	}
	runningRoundIDs := resolver.GetRunningRoundIDs(leaseSessionKey)
	r.mu.Lock()
	defer r.mu.Unlock()
	record = r.records[token]
	if record == nil {
		return Actor{}, errors.New("runtime command capability 无效或尚未装配")
	}
	r.purgeRecordActorsLocked(record, r.now())
	if len(record.actors) == 0 {
		delete(r.records, token)
		delete(r.tokens, record.sessionKey)
		return Actor{}, errors.New("runtime command round 已结束或尚未开始")
	}
	var active Actor
	activeRoundID := ""
	for _, roundID := range runningRoundIDs {
		roundID = strings.TrimSpace(roundID)
		if roundID == "" || roundID == activeRoundID {
			continue
		}
		if candidate, ok := record.actors[roundID]; ok {
			if activeRoundID != "" {
				return Actor{}, errors.New("runtime command session 存在并发 round，无法安全确定调用身份")
			}
			active = candidate.actor
			activeRoundID = roundID
		}
	}
	if activeRoundID == "" {
		return Actor{}, errors.New("runtime command round 已结束或尚未开始")
	}
	return active, nil
}

func (r *Registry) purgeDueLocked(now time.Time) {
	if !r.nextPurgeAt.IsZero() && now.Before(r.nextPurgeAt) {
		return
	}
	for token, record := range r.records {
		if record == nil {
			delete(r.records, token)
			continue
		}
		r.purgeRecordActorsLocked(record, now)
		if len(record.actors) == 0 {
			delete(r.records, token)
			delete(r.tokens, record.sessionKey)
		}
	}
	r.nextPurgeAt = now.Add(capabilityPurgeInterval)
}

func (r *Registry) purgeRecordActorsLocked(record *capabilityRecord, now time.Time) {
	for roundID, candidate := range record.actors {
		if !now.Before(candidate.expiresAt) {
			delete(record.actors, roundID)
		}
	}
}

func newCapabilityToken() (string, error) {
	payload := make([]byte, 32)
	if _, err := rand.Read(payload); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}
