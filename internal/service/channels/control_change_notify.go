// INPUT: 已提交的 Channel 控制状态写入（配置、账号、登录、配对）。
// OUTPUT: 按 owner 发出的失效通知，供实时层令频道与配对目录重读。
// POS: Channel 控制状态的实时失效出口；只通知发生了变化，不携带快照。
package channels

import "context"

// ControlChangeNotifier 接收 Channel 控制状态提交后的失效通知。
type ControlChangeNotifier func(ctx context.Context, ownerUserID string)

// SetControlChangeNotifier 注入控制状态失效通知器。
func (s *ControlService) SetControlChangeNotifier(notifier ControlChangeNotifier) {
	s.changeNotifier = notifier
}

func (s *ControlService) notifyControlChanged(ctx context.Context, ownerUserID string) {
	if s.changeNotifier != nil {
		s.changeNotifier(ctx, ownerUserID)
	}
}
