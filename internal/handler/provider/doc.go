// Package provider 封装 Provider 域的 HTTP handlers。
//
// L2 | 父级: internal/handler（L1 见 AGENTS.md）
//
// 成员清单：
//   - handlers.go：Handlers、Provider 配置路由，以及 CC Switch 同步与默认偏好联动。
//   - subscription.go：订阅额度相关 handler。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package provider
