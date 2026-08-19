// Package workspaceisolation 把 owner 路径策略统一投影到 nxs/Claude Hook。
// Linux enforce 模式只把普通 Agent 的 bridge 进程入口切到 root-owned launcher；
// 主智能体保留宿主身份与 owner-scoped nexusctl，Hook 允许当前 owner 的完整
// 用户数据根并拒绝跨 owner/作用域覆盖；普通 Agent 在所有模式都不能调用原始
// nexusctl。
//
// L2 | 父级: internal/runtime（L1 见 AGENTS.md）
//
// 成员清单：
//   - config.go：feature mode、launcher 环境契约与会话输入。
//   - launcher.go：launcher policy 准备和 bridge options 装配。
//   - reaper.go：跨 UID 进程信号与 owner cgroup 回收命令装配。
//   - launcher_linux.go / launcher_other.go：平台级 launcher 权限校验。
//   - policy.go：路径 canonicalization、symlink 防护与读写授权。
//   - hook.go：nxs/Claude 共用的 mandatory PreToolUse Hook；按 shell 语法位置区分真实环境赋值与普通 NAME=value 文本，禁止覆盖宿主 capability 而不误报输出文本。
//
// 暴露接口：NormalizeMode、Apply。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package workspaceisolation
