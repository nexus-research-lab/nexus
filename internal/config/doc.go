// Package config 承载 Go 服务运行时配置。
//
// L2 | 父级: internal（L1 见 AGENTS.md）
//
// 成员清单：
//   - config.go：Config 运行时配置与 Load，包括 Desktop 线上 Gateway、可选 Relay HTTP 端点与 Connector active/legacy host key 选择参数。
//   - loadenv.go：LoadDotEnv 从 .env 注入进程环境变量。
//   - workspace_path.go：部署环境 workspace 根规范化。
//
// 暴露接口：Config、Load、LoadDotEnv。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
// WindowsSandboxPreview 只由宿主 NEXUS_WINDOWS_SANDBOX_PREVIEW 显式启用旧机器 Host 监督，默认关闭；Windows 命令沙箱仍由 SDK MXC 合同默认负责，且不跳过能力握手。
// DesktopSandboxEnabled 标记当前为桌面模式、允许沙箱合同；实际启用还需用户偏好 desktop_sandbox_enabled（默认关闭），
// 不提供独立的进程环境开关。它不替代 Linux runtime isolation。
package config
