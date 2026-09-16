// Package team 统一 Relay 权威结果与 Nexus 本地投影的同步流程。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：service.go 定义可信 Access、远端与投影端口、五类同步操作及 ErrProjection；membership.go 转发 Room 真人治理命令。
// node.go / node_control.go 管理本机显式授权、固定 Control 公共入口和持久回执对账。
// node.go 的消息历史查询限制每批 100 个引用，身份与组织作用域只从远程 Cookie 派生。
// node.go 的 PrepareRoom 独立核验当前群成员与本人本机 Agent，首次执行前复用确定性 Room；不授权、不启动 runtime。
// node_executor.go / node_runtime.go 负责显式开启后的持久领取、Room 原生启动/审批/中断和完整输出 outbox，不另建 runtime。
// 目录、建群、快照和增量必须完成投影才成功；消息远端已提交时本地失败仅记录，沿旧游标恢复。
// 不隐式重发真人消息；机器消费者只重放固定 claim/output，不重跑未知工具。真人 HTTP/WSS 解析、身份交换和错误映射由 handler/team 持有。
//
// [PROTOCOL]: 变更时检查 handler/team、storage/teamrelay 与父级 AGENTS.md。
package team
