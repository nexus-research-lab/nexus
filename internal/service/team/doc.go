// Package team 统一 Relay 权威结果与 Nexus 本地投影的同步流程。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：service.go 定义可信 Access、远端与投影端口、五类同步操作及 ErrProjection。
// 目录、建群、快照和增量必须完成投影才成功；消息远端已提交时本地失败仅记录，沿旧游标恢复。
// 不隐式重发消息，不另设后台消费者；HTTP/WSS 解析、身份交换和错误映射由 handler/team 持有。
//
// [PROTOCOL]: 变更时检查 handler/team、storage/teamrelay 与父级 AGENTS.md。
package team
