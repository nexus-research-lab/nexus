// Package provider 封装 Provider 域的 HTTP handlers。
//
// L2 | 父级: internal/handler（L1 见 AGENTS.md）
//
// 成员清单：
//   - handlers.go：Handlers、Provider 配置路由，以及 CC Switch 同步与默认偏好联动。
//   - failure.go：Provider 强 ETag/If-Match 与基于稳定 service error 的 FailureCore 投影。
//   - subscription.go：公共订阅 Provider 的管理员路由与 FailureCore 权限投影。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package provider
