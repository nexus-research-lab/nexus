// Package team 负责本人 Agent 入群后的本机节点登记与在线投递执行。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：node.go / node_control.go 管理本人 Agent 入群后的本机自动登记、固定 Control 公共入口和持久回执对账。
// node.go 的消息历史查询限制每批 100 个引用，身份与组织作用域只从远程 Cookie 派生；一次请求内的身份与 Agent 目录由 nodeSession 共享，登记、撤销与入群授权经 provisionMu 串行。
// Desktop 经固定远程 Gateway 读取群成员时按 relay.CollectRoomMembers 的栅栏翻完全部成员页。
// node.go 的 PrepareRooms 独立核验当前群成员与本人本机 Agent，复用确定性 Room 并自动登记执行；暂停成员只准备会话，加入群不直接启动 runtime。
// 节点失效恢复使用当前有效真人登录，未知写入重放原意图；设备范围变更等已有任务收尾后再执行撤销与登记，不中断其他群任务。
// node_executor.go / node_runtime.go 负责持久领取、Room 原生启动/审批/中断和完整输出 outbox，不另建 runtime。
// 完整回复的结构化 @ 与候选正文一起持久化；切换候选先发布旧目标，无 @ 的新消息不能继承旧目标。
// 领取的 active Agent 范围与 Control 公开目录取交集，通过 Room PublicAgentDirectory 复用提示和 mention 解析；远端成员不物化为本机执行成员。
// node_files.go 仅消费当前轮次 deliverable 凭据，复用安全文件读取与 Relay 目录；冻结字节后重试，不扫描工作区。
// PrepareRooms 从常驻目录批量准备入群 Agent；RecoverJob 仅在精确 round 已停止且远端回执确认后结束未知任务。
// 消息附件凭精确投递租约下载并核验摘要，复用 Room.UploadConversationAttachment 与 ChatRequest.Attachments；Slash 原文进入同一原生展开入口。
// node_watch.go 使用独立 Node Principal 订阅 Relay WS；复用 duework 合并唤醒和退避，无固定领取轮询；稳定超过 1 分钟的连接断开从最小退避重连，授权停用时每 30 秒复查。
// 启动、连接初始提示、重连、授权变化和槽位释放触发持久待办对账；未知 running 不重跑，失败只重放原 claim/output。
// 领取为空时删除该 claiming 记录，不留下无来源的完成任务；Control 以 401 拒绝设备凭据时本地授权转 revoked，等本人下次入群重新登记。
// pending 返回的 next_due_at 交给 duework 一次性定时器，覆盖其他节点崩溃后的租约到期；空闲无任务不设定时扫描。
// 原生权限会话的变化信号唤醒租约维护，仅发送 running/waiting_input 白名单状态，不发送审批或工具正文。
// 执行中的 5 秒计时只续租；只有原生输出事件或已知失败的 outbox 重试才 drain，不扫描正常空 outbox。
// runtime 启动前按 上下文→准备 Room→续租→附件 顺序准备；确定性失败（含续租/附件被拒）ready→failed 并释放远端租约。
// 已结束执行的输出恢复中，续租或发布被明确拒绝均收口 failed 并释放槽位；网络故障保留 draining，不重跑 runtime。
// 执行日志以 stage、node/job/delivery/source_message 身份关联；相同节点/Agent/阶段失败每分钟最多告警一次。禁止记录凭据、消息正文及远端错误正文。
// 不隐式重发真人消息；机器消费者只重放固定 claim/output，不重跑未知工具。真人 HTTP/WSS 解析、身份交换和错误映射由 handler/team 持有。
//
// [PROTOCOL]: 变更时检查 storage/teamrelay 与父级 AGENTS.md。
package team
