// Package core 封装核心 HTTP handlers（健康、偏好、默认配置等）。
//
// L2 | 父级: internal/handler（L1 见 AGENTS.md）
//
// 成员清单：
//   - handlers.go：Handlers 及核心路由。
//   - imagegen_defaults.go：图片生成默认偏好 handler。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package core
