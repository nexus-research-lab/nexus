// Package team 统一 Relay 权威结果与 Nexus 本地投影的同步流程。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：service.go 定义可信 Access、远端与投影端口、五类同步操作及 ErrProjection；membership.go 转发 Room 真人治理命令。
// node.go / node_control.go 管理本机显式授权、固定 Control 公共入口和持久回执对账。
// node.go 的消息历史查询限制每批 100 个引用，身份与组织作用域只从远程 Cookie 派生。
// node.go 的 PrepareRoom 独立核验当前群成员与本人本机 Agent，复用确定性 Room 并自动登记执行；暂停成员只准备会话，加入群不直接启动 runtime。
// 节点失效恢复使用当前有效真人登录，未知写入重放原意图；设备范围变更等已有任务收尾后再执行撤销与登记，不中断其他群任务。
// node_executor.go / node_runtime.go 负责持久领取、Room 原生启动/审批/中断和完整输出 outbox，不另建 runtime。
// 消息附件凭精确投递租约下载并核验摘要，复用 Room.UploadConversationAttachment 与 ChatRequest.Attachments；Slash 原文进入同一原生展开入口。
// node_watch.go 使用独立 Node Principal 订阅 Relay WS；复用 duework 合并唤醒和退避，无固定领取轮询。
// 启动、连接初始提示、重连、授权变化和槽位释放触发持久待办对账；未知 running 不重跑，失败只重放原 claim/output。
// pending 返回的 next_due_at 交给 duework 一次性定时器，覆盖其他节点崩溃后的租约到期；空闲无任务不设定时扫描。
// 执行中的 5 秒计时只续租；只有原生输出事件或已知失败的 outbox 重试才 drain，不扫描正常空 outbox。
// 执行日志以 stage、node/job/delivery/source_message 身份关联；相同节点/Agent/阶段失败每分钟最多告警一次。禁止记录凭据、消息正文及远端错误正文。
// 目录、建群、快照和增量必须完成投影才成功；消息远端已提交时本地失败仅记录，沿旧游标恢复。
// 不隐式重发真人消息；机器消费者只重放固定 claim/output，不重跑未知工具。真人 HTTP/WSS 解析、身份交换和错误映射由 handler/team 持有。
//
// [PROTOCOL]: 变更时检查 handler/team、storage/teamrelay 与父级 AGENTS.md。
package team
