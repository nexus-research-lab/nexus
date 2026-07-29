package realtime

import "time"

// accelerateRoomGoalUsageRetry 缩短白盒测试中的退避时钟，不改变生产默认值。
func accelerateRoomGoalUsageRetry(service *Service) {
	if service != nil {
		service.goalUsageRetryBaseDelay = time.Millisecond
	}
}
