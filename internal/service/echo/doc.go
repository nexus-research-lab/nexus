// Package echo 负责主动跟进的用户级开关与判断。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go：服务装配、带 Preferences revision 的设置更新，以及 dm.EchoPolicy 的 ScheduleAfter/Decide。
//   - gate.go：轻量 Gate、开口提示与活跃时段换算。
//
// Echo 只在主智能体主窗口生效：事件写入、取消、到点唤醒与延迟提交都在 dm 的主窗口 inbox（main_inbox_echo.go）。
//
// 暴露接口：NewService、GetSettings、UpdateSettings、UpdateSettingsAtVersion、ScheduleAfter、Decide。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package echo
