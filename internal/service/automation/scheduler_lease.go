package automation

// 本文件管理调度器数据库租约；任务定义与执行领取仍由各自仓储事务负责。

import (
	"context"
	"strings"
	"time"
)

const automationSchedulerLeaseName = "scheduled-tasks"
const defaultAutomationSchedulerLeaseTTL = 30 * time.Second

func (s *Service) schedulerLeaseTTL() time.Duration {
	seconds := s.config.AutomationSchedulerLeaseSeconds
	if seconds <= 0 {
		return defaultAutomationSchedulerLeaseTTL
	}
	return time.Duration(seconds) * time.Second
}

// refreshSchedulerLease 返回当前实例是否持有租约，以及本次是否刚成为 leader。
func (s *Service) refreshSchedulerLease(ctx context.Context, now time.Time) (bool, bool, error) {
	s.mu.Lock()
	held := s.schedulerLeaseHeld
	renewAt := s.schedulerLeaseRenewAt
	ownerID := strings.TrimSpace(s.schedulerOwnerID)
	s.mu.Unlock()

	if held && now.Before(renewAt) {
		return true, false, nil
	}
	if ownerID == "" {
		ownerID = s.idFactory("scheduler")
		s.mu.Lock()
		s.schedulerOwnerID = ownerID
		s.mu.Unlock()
	}

	ttl := s.schedulerLeaseTTL()
	acquired, err := s.repository.TryAcquireSchedulerLease(
		ctx,
		automationSchedulerLeaseName,
		ownerID,
		now,
		now.Add(ttl),
	)
	if err != nil {
		s.mu.Lock()
		s.schedulerLeaseHeld = false
		s.schedulerLeaseRenewAt = time.Time{}
		s.mu.Unlock()
		return false, false, err
	}

	s.mu.Lock()
	becameLeader := acquired && !s.schedulerLeaseHeld
	s.schedulerLeaseHeld = acquired
	if acquired {
		s.schedulerLeaseRenewAt = now.Add(ttl / 3)
	} else {
		s.schedulerLeaseRenewAt = time.Time{}
	}
	s.mu.Unlock()
	return acquired, becameLeader, nil
}

// releaseSchedulerLease 释放当前实例持有的租约。
func (s *Service) releaseSchedulerLease(ctx context.Context) error {
	s.mu.Lock()
	held := s.schedulerLeaseHeld
	ownerID := s.schedulerOwnerID
	s.schedulerLeaseHeld = false
	s.schedulerLeaseRenewAt = time.Time{}
	s.mu.Unlock()
	if !held || strings.TrimSpace(ownerID) == "" {
		return nil
	}
	return s.repository.ReleaseSchedulerLease(ctx, automationSchedulerLeaseName, ownerID)
}
