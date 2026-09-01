// Package echo 负责主动跟进设置与 durable attempt 调度。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go：服务装配、带 Preferences revision 的设置更新、有界 detached 收口和生命周期入口。
//   - scheduler.go：attempt 资格判断、模型门禁、运行与可见消息提交闭环。
//
// 暴露接口：NewService、GetSettings、UpdateSettings、UpdateSettingsAtVersion、
// SettingsUpdateCommitted、OnUserActivity、OnTerminal、Start、Stop。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package echo
