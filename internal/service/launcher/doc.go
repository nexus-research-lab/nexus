// Package launcher 提供 Launcher 首屏查询与推荐能力。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go / bootstrap.go：Service 与 Session metadata/有界最新消息页组成的首屏最小必要数据（Bootstrap），幂等保证主智能体默认聊天存在，并记录慢查询阶段耗时；单个历史读取失败不得阻断目录。
//   - model.go：Launcher 视图模型。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package launcher
