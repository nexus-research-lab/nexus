// Package config 承载 Go 服务运行时配置与主机级可持久化设置。
//
// L2 | 父级: internal（L1 见 AGENTS.md）
//
// 成员清单：
//   - config.go：Config 运行时配置与 Load。
//   - loadenv.go：LoadDotEnv 从 .env 注入进程环境变量。
//   - runtime_settings.go：RuntimeSettings 可由 UI 持久化的主机级运行配置。
//
// 暴露接口：Config、Load、LoadDotEnv、RuntimeSettings。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package config
