// INPUT: 应用装配提供的结构化 logger。
// OUTPUT: Goal durable resume 与立即 continuation 失败的进程级诊断。
// POS: Goal 业务服务可观测性边界；日志不参与状态机或重试判定。
package goal

import "log/slog"

// SetLogger 注入 Goal 后台恢复与异步 continuation 的结构化日志实例。
func (s *Service) SetLogger(logger *slog.Logger) {
	if s == nil {
		return
	}
	s.logger = logger
}
