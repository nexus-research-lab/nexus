// INPUT: 宿主资源配置、内存压力探针及 Manager 持有的会话和子智能体生命周期。
// OUTPUT: 启动前的共享额度预留、有界等待和空闲会话回收。
// POS: DM、Room 与后台 runtime 共用的软准入；不冒充 OS 进程内存硬限制。
package runtime

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	ErrRuntimeResourceBusy   = errors.New("运行资源已满，请等待当前任务结束后重试")
	ErrRuntimeMemoryPressure = errors.New("系统内存紧张，请等待内存释放后重试")
)

const (
	runtimeMaxPendingStartups  = 16
	runtimeResourceWaitTimeout = 30 * time.Second
)

// ResourceLimits 由 app 在 Manager 启用前装配；非正额度或 nil 探针关闭对应限制。
// MemoryAvailable 只读取宿主状态，允许并发调用；错误不能当作内存充足。
type ResourceLimits struct {
	MaxSessions     int
	MaxSubagents    int
	MemoryAvailable func() (bool, error)
}

func (m *Manager) SetResourceLimits(limits ResourceLimits) {
	m.mu.Lock()
	m.resourceLimits = limits
	m.mu.Unlock()
}

func (m *Manager) memoryAdmissionError() error {
	m.mu.RLock()
	probe := m.resourceLimits.MemoryAvailable
	m.mu.RUnlock()
	if probe == nil {
		return nil
	}
	available, err := probe()
	if err != nil {
		return fmt.Errorf("无法确认系统可用内存，暂缓启动: %w", err)
	}
	if !available {
		return ErrRuntimeMemoryPressure
	}
	return nil
}

// 等待只发生在启动事务内。子智能体回调直接拒绝超额启动，防止父任务
// 占着所有额度等待子任务，产生无法自行解除的等待环。
func (s *ClientStartup) admitResources(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, runtimeResourceWaitTimeout)
	defer cancel()
	m := s.manager
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		memoryErr := m.memoryAdmissionError()
		if memoryErr != nil && !errors.Is(memoryErr, ErrRuntimeMemoryPressure) {
			return memoryErr
		}
		m.mu.Lock()
		err := s.validateCloseEpochLocked()
		if err == nil {
			err = m.runtimeAgentAdmissionErrorLocked(s.sessionKey, s.ownerUserID(), runtimeSessionAgentID(s.sessionKey))
		}
		if state := m.sessions[s.sessionKey]; err == nil && state != nil && state.Closing {
			err = ErrRuntimeSessionClosing
		}
		admitted := err == nil && memoryErr == nil && s.reserveResourceSlotLocked()
		m.mu.Unlock()
		if err != nil || admitted {
			return err
		}
		// 回收路径与 TTL 清理共用关闭栅栏；失败会保留 Client 和占用额度。
		closed, err := m.closeIdleSessions(ctx, 0, 1)
		if err != nil {
			return fmt.Errorf("回收空闲运行资源失败: %w", err)
		}
		if closed > 0 {
			continue
		}
		blocked := ErrRuntimeResourceBusy
		if memoryErr != nil {
			blocked = memoryErr
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return errors.Join(blocked, ctx.Err())
		case <-timer.C:
		}
	}
}

func (s *ClientStartup) ownerUserID() string {
	if s.ownerLease == nil {
		return ""
	}
	return s.ownerLease.ownerUserID
}

func (s *ClientStartup) reserveResourceSlotLocked() bool {
	m := s.manager
	state := m.sessions[s.sessionKey]
	if !s.gate.resourceReserved && (state == nil || state.Client == nil) &&
		m.resourceLimits.MaxSessions > 0 && m.residentSessionCountLocked() >= m.resourceLimits.MaxSessions {
		return false
	}
	s.gate.resourceReserved = true
	return true
}

func (m *Manager) residentSessionCountLocked() int {
	count := 0
	for _, state := range m.sessions {
		if state != nil && state.Client != nil {
			count++
		}
	}
	for key, gate := range m.startupGates {
		state := m.sessions[key]
		if gate.resourceReserved && (state == nil || state.Client == nil) {
			count++
		}
	}
	return count
}

func (m *Manager) pendingStartupLimitReachedLocked() bool {
	limits := m.resourceLimits
	return (limits.MaxSessions > 0 || limits.MemoryAvailable != nil) && m.activeStartups >= runtimeMaxPendingStartups
}

func (m *Manager) subagentLimitReachedLocked() bool {
	if m.resourceLimits.MaxSubagents <= 0 {
		return false
	}
	count := 0
	for _, state := range m.sessions {
		if state != nil {
			count += activeSubagentBindings(state)
		}
	}
	return count >= m.resourceLimits.MaxSubagents
}

func activeSubagentBindings(state *sessionState) int {
	count := 0
	for _, binding := range state.SubagentHookBindings {
		if !binding.Terminal {
			count++
		}
	}
	return count
}
