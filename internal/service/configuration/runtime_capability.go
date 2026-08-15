// INPUT: 宿主验证过的 Agent/DM/Room runtime 身份与随机 capability token。
// OUTPUT: 只在对应 runtime round 存活时可解析的 configuration Actor。
// POS: nexuscfg broker 的身份防伪边界；CLI 参数和环境变量都不能声明 Actor。
package configuration

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"time"
)

const runtimeCapabilityTTL = 24 * time.Hour

type runtimeCapabilityActor struct {
	actor     Actor
	expiresAt time.Time
}

type runtimeCapabilityRecord struct {
	sessionKey string
	actors     map[string]runtimeCapabilityActor
}

// IssueRuntimeCapability 为稳定 runtime session 签发不可猜测凭据。
// 同一 session 跨 round 复用 token，避免仅因 capability 轮换重启 warm runtime。
func (s *Service) IssueRuntimeCapability(actor Actor) (string, error) {
	if s == nil || s.runtime == nil {
		return "", errors.New("nexuscfg runtime capability 尚未装配")
	}
	actor.OwnerUserID = strings.TrimSpace(actor.OwnerUserID)
	actor.AgentID = strings.TrimSpace(actor.AgentID)
	actor.LeaseSessionKey = strings.TrimSpace(actor.LeaseSessionKey)
	actor.LeaseRoundID = strings.TrimSpace(actor.LeaseRoundID)
	if !actor.RoundLeaseRequired || actor.OwnerUserID == "" || actor.AgentID == "" ||
		actor.LeaseSessionKey == "" || actor.LeaseRoundID == "" {
		return "", errors.New("nexuscfg runtime capability 缺少可信 owner、Agent 或 round lease")
	}

	now := s.runtimeCapabilityNow()
	sessionKey := strings.Join([]string{
		actor.OwnerUserID,
		actor.AgentID,
		actor.LeaseSessionKey,
	}, "\x00")
	s.runtimeCapabilityMu.Lock()
	defer s.runtimeCapabilityMu.Unlock()
	s.purgeRuntimeCapabilitiesLocked(now)
	token := s.runtimeCapabilityBySession[sessionKey]
	record := s.runtimeCapabilities[token]
	if record == nil {
		var err error
		token, err = newRuntimeCapabilityToken()
		if err != nil {
			return "", err
		}
		record = &runtimeCapabilityRecord{
			sessionKey: sessionKey,
			actors:     make(map[string]runtimeCapabilityActor),
		}
		s.runtimeCapabilities[token] = record
		s.runtimeCapabilityBySession[sessionKey] = token
	}
	record.actors[actor.LeaseRoundID] = runtimeCapabilityActor{
		actor:     actor,
		expiresAt: now.Add(runtimeCapabilityTTL),
	}
	return token, nil
}

// ResolveRuntimeCapability 只返回当前仍在运行且唯一的 round 身份。
func (s *Service) ResolveRuntimeCapability(token string) (Actor, error) {
	if s == nil || s.runtime == nil {
		return Actor{}, errors.New("nexuscfg runtime capability 尚未装配")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return Actor{}, errors.New("nexuscfg runtime capability 不能为空")
	}
	now := s.runtimeCapabilityNow()
	s.runtimeCapabilityMu.Lock()
	s.purgeRuntimeCapabilitiesLocked(now)
	record := s.runtimeCapabilities[token]
	actors := make([]Actor, 0)
	if record != nil {
		for _, candidate := range record.actors {
			actors = append(actors, candidate.actor)
		}
	}
	s.runtimeCapabilityMu.Unlock()
	if record == nil {
		return Actor{}, errors.New("nexuscfg runtime capability 无效或已过期")
	}

	active := make([]Actor, 0, 1)
	for _, actor := range actors {
		for _, roundID := range s.runtime.GetRunningRoundIDs(actor.LeaseSessionKey) {
			if roundID == actor.LeaseRoundID {
				active = append(active, actor)
				break
			}
		}
	}
	if len(active) == 0 {
		return Actor{}, errors.New("nexuscfg runtime round 已结束或尚未开始")
	}
	if len(active) != 1 {
		return Actor{}, errors.New("nexuscfg runtime session 存在并发 round，无法安全确定调用身份")
	}
	return active[0], nil
}

func (s *Service) purgeRuntimeCapabilitiesLocked(now time.Time) {
	for token, record := range s.runtimeCapabilities {
		if record == nil {
			delete(s.runtimeCapabilities, token)
			continue
		}
		for roundID, candidate := range record.actors {
			if !now.Before(candidate.expiresAt) {
				delete(record.actors, roundID)
			}
		}
		if len(record.actors) == 0 {
			delete(s.runtimeCapabilities, token)
			delete(s.runtimeCapabilityBySession, record.sessionKey)
		}
	}
}

func newRuntimeCapabilityToken() (string, error) {
	payload := make([]byte, 32)
	if _, err := rand.Read(payload); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}
