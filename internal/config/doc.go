// Package config 承载 Go 服务运行时配置。
//
// L2 | 父级: internal（L1 见 AGENTS.md）
//
// 成员清单：
//   - config.go：Config 运行时配置与 Load，包括 Connector active/legacy host key 选择参数。
//   - loadenv.go：LoadDotEnv 从 .env 注入进程环境变量。
//   - workspace_path.go：部署环境 workspace 根规范化。
//
// 暴露接口：Config、Load、LoadDotEnv。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package config
