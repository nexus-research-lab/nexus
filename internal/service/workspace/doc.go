// Package workspace 提供 Agent workspace 的文件读写、上传与实时同步。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go / file.go / memory.go / mutation.go / upload.go / path.go：Service、基于 confined-fd 的文件/记忆/条目/上传访问与路径。
//   - agent.go / model.go / reveal.go：Agent workspace、模型、本机定位。
//   - initializer.go / initializer_*.go：workspace 初始化阶段、主 Agent 文件策略，以及
//     全局绑定/显式停用与 workspace 动态 Skill 的运行时投影（复用 Agent 默认行为模板 / nexusctl / 模板集）。
//   - platform_skills.go / host_skills.go / user_skills.go：平台、桌面宿主与 owner 外部
//     Skill 源同步、边界内原子目录替换、Claude Code 兼容入口（nxs 与 Claude Code 共用）。
//   - live.go / live_*.go：实时文件树模型与同步阶段（行级 diff / watcher / write）。
//   - upload_dedupe.go：上传去重。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package workspace
