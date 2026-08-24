// Package runtimeselection 提供用户级 runtime 默认值读取能力。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go：runtime 主/后台/视觉模型默认值解析，以及持久化 WebSearch 偏好到 runtime 配置的投影。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package runtimeselection
