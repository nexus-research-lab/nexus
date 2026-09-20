// Package launcher 提供 Launcher 首屏查询与推荐能力。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go / bootstrap.go：Service 与 Session metadata/SQLite 短摘要组成的首屏最小必要数据（Bootstrap），幂等保证主智能体默认聊天存在，并记录慢查询阶段耗时；预览按 owner 一次查询独立摘要表，读取共享 500ms 预算；缺失或失效时留空，不扫描或重建历史。
//   - model.go：Launcher 视图模型。
//
// 慢查询与单会话预览失败必须走请求上下文 logger，确保耗时和 request_id 进入桌面导出日志。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package launcher
