// 测试专用入口：生产路径已不再调用，仅供本包测试复用。
package automation

import "context"

func (s *Service) runDueOnce() {
	now := s.nowFn()
	s.runDueOnceAt(now)
	deliveryAt, err := s.loadDeliveryRetryDeadline(
		context.Background(),
		now,
	)
	if err == nil && deadlineReached(deliveryAt, now) {
		s.startDeliveryRetryBatch(now)
	}
}
