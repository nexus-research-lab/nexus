// Package workspace 提供 Agent workspace 的文件读写、上传与实时同步。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go / file.go / memory.go / mutation.go / upload.go / path.go：Service、基于 confined-fd 的文件/记忆/条目/上传访问与路径。
//   - agent.go / model.go / reveal.go：Agent workspace、模型、本机定位。
//   - initializer.go / initializer_*.go：workspace 初始化阶段、主 Agent 文件策略，
//     全局绑定/显式停用与 workspace 动态 Skill 的运行时投影，以及共享 nexusctl、
//     nexuscfg、Agent-facing nexus shim 的安全生成；nexus shim 在每次宿主启动时
//     刷新到同版本 host multicall，不在 Agent round 中从源码构建。
//   - platform_skills.go / host_skills*.go / host_skill_link_*.go / user_skills.go：平台、
//     桌面宿主与 owner 外部 Skill 源同步、宿主直接 Skill 及安全目录链接快照、
//     稳定根下的分 Skill 刷新与后台监听、
//     边界内原子目录替换及 Claude Code 兼容入口（nxs 与 Claude Code 共用）。
//   - live.go / live_*.go：实时文件树模型与同步阶段（行级 diff / watcher / write）。
//   - upload_dedupe.go：上传去重。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package workspace
