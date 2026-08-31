// Package preferences 读写带单调版本的用户级偏好 JSON 和独立 WebSearch 凭据。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go：按 owner 串行、经 confinedfs 完成 Get/Update、version CAS、跨文件双代发布与条件回滚。
//   - codec.go：Preferences/credential 的持久化格式、版本匹配、双代凭据构造与规范化解码。
//   - model.go：含持久化 version 的偏好模型、规范化和 WebSearch 校验。
//   - imagegen_tool.go：Web 与对话配置共用的默认图片工具投影。
//
// 暴露接口：NewService、Get、Update、SetEchoEnabled、SetEchoEnabledAtVersion、UpdateAtVersion、
// UpdatePrepared、UpdatePreparedAtVersion、RestoreIfVersion、DefaultPreferences、
// ReconcileImagegenDefaultTool。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package preferences
