// Package cli 装配 nexusctl 命令行应用。
//
// L2 | 父级: internal（L1 见 AGENTS.md）
//
// 成员清单：
//   - app.go：New 创建 CLI 应用。
//   - services.go：按命令域延迟创建服务，避免 help 等命令拉起全量后端依赖。
//   - command_*.go：各命令域（agent / auth / automation / channel / connector /
//     conversation / emotion / imagegen / launcher / room / session / skill / workspace）。
//   - output.go / flag_int_pointer.go：输出格式与 flag helper。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package cli
