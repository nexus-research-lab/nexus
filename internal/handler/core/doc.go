// Package core 封装核心 HTTP handlers（健康、偏好、默认配置等）；部署路径不由 HTTP 修改。
//
// L2 | 父级: internal/handler（L1 见 AGENTS.md）
//
// 成员清单：
//   - handlers.go：Handlers、核心路由及 Preferences 条件回滚的 WebSearch 热同步。
//   - preferences_version.go：Preferences ETag/If-Match CAS 和读写阶段 FailureCore 投影。
//   - imagegen_defaults.go：在 Preferences owner 锁内完成的图片生成默认偏好投影。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package core
