// INPUT: 宿主验证过的 Agent/DM/Room runtime 身份、AutomationRun 与随机 capability。
// OUTPUT: 只在唯一活跃 physical round 中可解析的 Automation command Actor。
// POS: Nexus CLI broker 的身份防伪边界；CLI 参数和环境变量都不能声明 Actor/job/run。
package automation

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"time"
)

const runtimeCommandCapabilityTTL = 24 * time.Hour

type runtimeCommandRoundResolver interface {
	GetRunningRoundIDs(string) []string
}

// RuntimeCommandActor 是宿主为一个 physical round 固定的 Automation 调用身份。
type RuntimeCommandActor struct {
	OwnerUserID        string
	AgentID            string
	AgentName          string
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
}

func (a RuntimeCommandActor) normalized() RuntimeCommandActor {
	result := a
	result.OwnerUserID = strings.TrimSpace(result.OwnerUserID)
	result.AgentID = strings.TrimSpace(result.AgentID)
	result.AgentName = strings.TrimSpace(result.AgentName)
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

func (a RuntimeCommandActor) valid() bool {
	value := a.normalized()
	return value.OwnerUserID != "" && value.AgentID != "" &&
		value.LeaseSessionKey != "" && value.LeaseRoundID != ""
}

// MutationAllowed 只允许可信交互来源修改任务；后台 run 和内部/外部回传只读。
func (a RuntimeCommandActor) MutationAllowed() bool {
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

// CrossAgentAllowed 只由主智能体自己的可信 Nexus WebSocket 私有 DM 签发。
func (a RuntimeCommandActor) CrossAgentAllowed() bool {
	return a.IsMainAgent && strings.TrimSpace(a.SourceContextType) == "agent" && a.MutationAllowed()
}

type runtimeCommandCapabilityActor struct {
	actor     RuntimeCommandActor
	expiresAt time.Time
}

type runtimeCommandCapabilityRecord struct {
	sessionKey string
	actors     map[string]runtimeCommandCapabilityActor
}

// SetRuntimeCommandRoundResolver 注入 runtime manager 的只读活跃 round 解析能力。
func (s *Service) SetRuntimeCommandRoundResolver(resolver runtimeCommandRoundResolver) {
	if s == nil {
		return
	}
	s.runtimeCommandMu.Lock()
	s.runtimeCommandRounds = resolver
	s.runtimeCommandMu.Unlock()
}

// IssueRuntimeCommandCapability 为稳定 runtime session 复用 token，并登记当前 round Actor。
func (s *Service) IssueRuntimeCommandCapability(actor RuntimeCommandActor) (string, error) {
	if s == nil {
		return "", errors.New("Automation runtime command capability 尚未装配")
	}
	actor = actor.normalized()
	if !actor.valid() {
		return "", errors.New("Automation runtime command capability 缺少可信 owner、Agent 或 round lease")
	}
	now := s.nowFn()
	sessionKey := strings.Join([]string{actor.OwnerUserID, actor.AgentID, actor.LeaseSessionKey}, "\x00")
	s.runtimeCommandMu.Lock()
	defer s.runtimeCommandMu.Unlock()
	s.purgeRuntimeCommandCapabilitiesLocked(now)
	token := s.runtimeCommandTokens[sessionKey]
	record := s.runtimeCommandRecords[token]
	if record == nil {
		var err error
		token, err = newRuntimeCommandCapabilityToken()
		if err != nil {
			return "", err
		}
		record = &runtimeCommandCapabilityRecord{
			sessionKey: sessionKey,
			actors:     make(map[string]runtimeCommandCapabilityActor),
		}
		s.runtimeCommandRecords[token] = record
		s.runtimeCommandTokens[sessionKey] = token
	}
	record.actors[actor.LeaseRoundID] = runtimeCommandCapabilityActor{
		actor: actor, expiresAt: now.Add(runtimeCommandCapabilityTTL),
	}
	return token, nil
}

// ResolveRuntimeCommandCapability 只返回当前仍在运行且唯一的 round Actor。
func (s *Service) ResolveRuntimeCommandCapability(token string) (RuntimeCommandActor, error) {
	if s == nil {
		return RuntimeCommandActor{}, errors.New("Automation runtime command capability 尚未装配")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return RuntimeCommandActor{}, errors.New("Automation runtime command capability 不能为空")
	}
	now := s.nowFn()
	s.runtimeCommandMu.Lock()
	s.purgeRuntimeCommandCapabilitiesLocked(now)
	record := s.runtimeCommandRecords[token]
	resolver := s.runtimeCommandRounds
	actors := make([]RuntimeCommandActor, 0)
	if record != nil {
		for _, candidate := range record.actors {
			actors = append(actors, candidate.actor)
		}
	}
	s.runtimeCommandMu.Unlock()
	if record == nil || resolver == nil {
		return RuntimeCommandActor{}, errors.New("Automation runtime command capability 无效或尚未装配")
	}
	active := make([]RuntimeCommandActor, 0, 1)
	for _, actor := range actors {
		for _, roundID := range resolver.GetRunningRoundIDs(actor.LeaseSessionKey) {
			if strings.TrimSpace(roundID) == actor.LeaseRoundID {
				active = append(active, actor)
				break
			}
		}
	}
	if len(active) == 0 {
		return RuntimeCommandActor{}, errors.New("Automation runtime round 已结束或尚未开始")
	}
	if len(active) != 1 {
		return RuntimeCommandActor{}, errors.New("Automation runtime session 存在并发 round，无法安全确定调用身份")
	}
	return active[0], nil
}

func (s *Service) purgeRuntimeCommandCapabilitiesLocked(now time.Time) {
	for token, record := range s.runtimeCommandRecords {
		if record == nil {
			delete(s.runtimeCommandRecords, token)
			continue
		}
		for roundID, candidate := range record.actors {
			if !now.Before(candidate.expiresAt) {
				delete(record.actors, roundID)
			}
		}
		if len(record.actors) == 0 {
			delete(s.runtimeCommandRecords, token)
			delete(s.runtimeCommandTokens, record.sessionKey)
		}
	}
}

func newRuntimeCommandCapabilityToken() (string, error) {
	payload := make([]byte, 32)
	if _, err := rand.Read(payload); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}
