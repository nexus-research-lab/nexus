// Package skill 封装技能域的 HTTP handlers。
//
// L2 | 父级: internal/handler（L1 见 AGENTS.md）
//
// 成员清单：
//   - handlers.go：Handlers 及全局技能目录、Agent 私有 workspace Skill、
//     Agent 使用矩阵、启停、私有来源管理和兼容导入路由。
//   - failure.go：Marketplace 写请求的 not_applied/committed/unknown FailureCore 投影。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package skill
