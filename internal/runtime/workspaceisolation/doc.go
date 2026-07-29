// Package workspaceisolation 把 owner 路径策略统一投影到 nxs/Claude Hook，
// 并在 Linux enforce 模式把普通 Agent 的 bridge 进程入口切到 root-owned
// launcher；主智能体保留宿主控制面身份，以使用 owner-scoped nexusctl。
//
// L2 | 父级: internal/runtime（L1 见 AGENTS.md）
//
// 成员清单：
//   - config.go：feature mode、launcher 环境契约与会话输入。
//   - launcher.go：launcher policy 准备和 bridge options 装配。
//   - reaper.go：owner cgroup 回收命令装配。
//   - launcher_linux.go / launcher_other.go：平台级 launcher 权限校验。
//   - policy.go：路径 canonicalization、symlink 防护与读写授权。
//   - hook.go：nxs/Claude 共用的 mandatory PreToolUse Hook。
//
// 暴露接口：NormalizeMode、Apply。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package workspaceisolation
