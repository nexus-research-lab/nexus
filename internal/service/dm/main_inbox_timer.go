// INPUT: 主窗口 inbox 中已到点的 pending 事件与最近的未来 available_at。
// OUTPUT: 启动恢复（退回遗留 delivering）与到点唤醒；没有周期轮询，只在最近到点时间或写入未来事件时醒来。
// POS: 主窗口未来事件（跟进、Echo）的唯一定时器；起轮仍由 main_inbox.go 的 drain 负责。
package dm

import (
	"context"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/duework"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
)

func (s *Service) newMainInboxTimer() *duework.Loop {
	return duework.New(duework.Options{
		OnError: func(err error) {
			s.LoggerFor(context.Background()).Warn("主窗口 inbox 定时唤醒失败，等待重试", "err", err)
		},
	})
}

// StartMainInboxTimer 退回上一进程遗留的 delivering 后启动定时器；首轮 reconcile 即唤醒仍有积压的主窗口。
func (s *Service) StartMainInboxTimer(ctx context.Context) (func(), error) {
	if s.inbox == nil || s.inboxTimer == nil {
		return nil, nil
	}
	if err := s.inbox.ReleaseDelivering(ctx, "", ""); err != nil {
		return nil, err
	}
	loopCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = s.inboxTimer.Run(loopCtx, s.reconcileMainInboxes)
	}()
	return func() {
		cancel()
		<-done
	}, nil
}

// notifyMainInboxTimer 在写入未来事件后让定时器重算最近到点时间。
func (s *Service) notifyMainInboxTimer() {
	if s.inboxTimer != nil {
		s.inboxTimer.Notify()
	}
}

// reconcileMainInboxes 唤醒所有已有到点事件的主窗口；单个主窗口解析失败只记录，不拖住其他主窗口。
func (s *Service) reconcileMainInboxes(ctx context.Context, now time.Time) (duework.Result, error) {
	inboxes, err := s.inbox.ListPendingInboxes(ctx, now)
	if err != nil {
		return duework.Result{}, err
	}
	for _, item := range inboxes {
		ownerCtx := runtimehost.ContextWithExactOwner(ctx, item.OwnerUserID)
		target, ok, resolveErr := s.resolveMainInboxTarget(ownerCtx, item.InboxKey, "")
		if resolveErr != nil {
			s.LoggerFor(ctx).Warn("主窗口 inbox 无法解析，跳过唤醒", "inbox_key", item.InboxKey, "err", resolveErr)
			continue
		}
		if ok {
			s.wakeMainInbox(target)
		}
	}
	next, err := s.inbox.NextAvailableAt(ctx, now)
	if err != nil {
		return duework.Result{}, err
	}
	return duework.Result{NextDueAt: next}, nil
}
