// Package workspace 是 Agent 历史、transcript、输入队列与房间历史的持久化层。
//
// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//
// 成员清单：
//   - input_queue_recovery.go：持久化 dispatch pause/claim、带暂停 revision 栅栏的单项显式恢复准入与 owner-scoped canonical 队列发现；下游受理后才消费条目。
//   - agent_history.go / agent_history_overlay.go / room_history.go：完整回复落盘后更新独立 Room SQL 摘要，群聊只接受公区写入；失败记日志，不重试 canonical 消息。
//   - history_*.go / user_input_probe.go：分阶段历史投影与严格只读用户输入判定（compact / normalize / causal control ordering / pagination / last-write round_index /
//     rewrite_tail（区分未物化 round 与损坏边界）/ fork marker 前缀 / 按 Agent 执行轮配对的 result_summary / 同 message_id 合并 Goal 完成收据 / external_delivery / 不为完成态 host control 伪造中断 assistant 的 unfinished_round）。
//   - conversation_history.go / conversation_history_read.go：历史读模型（独立的 conversation-history.v1.sqlite）。写入时按 message_id 合并落表、
//     大块 Tool 输出与图片拆成内容寻址的 gzip detail、按轮重算导航条目；读取只查表并按页规范化（只有引导去重需要一次跨轮查询）。
//     会话未就绪、失效或来源身份变化时由读取方同步整会话回填（单飞 + 失败退避）；进程内第一次读取发现最近轮次未收口时回填一次，
//     补上进程在 runtime 写完 transcript、表还没提交时退出留下的缺口。首次打开时删除旧版派生读模型文件。
//   - conversation_history_room.go：Room 公区 ledger 追加与落表同锁、记录 ledger 偏移；读取时比对 ledger 身份与大小，被替换或截断即整会话回填，落后即补齐。
//   - conversation_history_dm.go：DM（agent:<id>:<channel>:dm:<ref>）的 round marker、overlay 与实时 transcript 原生回复落表，改写裁剪与 fork 同步；
//     Room 成员私有会话与 fork 未完成的会话分页时直接按 canonical 读取。
//   - history_page_index.go / history_message_detail.go：按根轮次分组与分页选择（读模型与 canonical 分页共用）、来源文件身份快照与 detail 拆分规则。
//   - conversation_history_test.go / history_pagination_test.go：读模型与 canonical oracle 的逐页等价、实时落表、detail、删除、改写、换 transcript 与重启后补齐，以及 DM/Room 受理游标与长会话夹具回归；
//     room_history_test.go 验证「按轮规范化等于整段规范化」「实时消息等于解析 transcript_ref」两个前提。
//   - agent_history*.go：Agent 历史门面、K3 工具面换代后的分段 transcript lineage 读取、overlay 与共享模型；
//     round marker 携带受理游标 input_seq 并原样物化到用户行，Room inline 用户行自带同一字段，合并同 ID 用户快照时不丢失。
//   - overlay_replay.go：session overlay 的进程内增量回放（overlay 历史行/marker 状态与 DM round index 共用）；按文件身份、尺寸、
//     同尺寸修改时间与首尾指纹确认纯追加后只解析新增完整行，文件被替换/截断/改写、末行未写完或新行覆盖已回放的 message_id 时整份重读；
//     回放状态按最近使用保留，受会话个数与源文件总字节上限约束。
//   - runtime_repair.go：enforce 模式下 owner runtime 权限修复与受限重试；transcript reader 与回填读取共用修复入口。
//   - transcript_*.go：基于文件身份/大小/mtime/首尾指纹且由普通、分段、显式读取共用的 JSONL 解析缓存（纯追加只解析新增字节并沿用解析代系；完整解析大文件时按换行分块并行，行号与顺序解析一致）、
//     进程内 DM 投影前缀检查点（同代系主链与 marker 前缀不变时只重放最后一个用户轮次之后的条目）、
//     marker 对齐结果（transcript_marker_alignment.go）、重复 UUID/自指链修复、reader、path、session、project、
//     可见性安全的 marker 对齐（含 reminder 提取后的空白 Goal 续跑）、guidance 与 root/source round 投影；
//     投影与 rewrite/fork 共用边界。
//   - 有界定向消息队列用 client_message_id 读取完整入队账本，已消费输入在恢复时不会重复派发。
//     派发失败通过 RestoreDispatched 按日志中的原位置恢复，保留跨成员排序的 QueueOrder。
//   - input_queue.go / input_queue_codec.go / input_queue_replay.go：输入队列存取、携带非授权 Goal collaboration attribution、完整 Execution WorkBinding 或独立 ReviewBinding 的跨派发持久幂等入队、责任项禁止 guide/合并的 capability envelope fence、可返回规范化提交的原子批量登记、预检版本一致的整批 conversation guidance 认领、按执行 scope 隔离的编解码与事件重放；exact recovery target 随回执持久化，后续用户停止撤销旧请求准入。
//   - room_history.go / room_directed_message.go / room_directed_message_wake.go / session_file.go / session_lifecycle.go / artifact_probe.go / jsonl.go：
//     Session runtime 回写可按 configuration version 强制拒绝过期的后台 Connector 预备；
//     保留 Agent 执行身份与 public handoff reply 因果的房间历史、可反向修复两阶段写入且按稳定 message_id 幂等的 Goal-attributed 定向消息/Room handoff（含 terminal/Goal handback 独立阶段和审计约束的 legacy attribution repair）、immediate/delayed schedule-complete 唤醒、带最后一次上下文占用快照的会话文件、可随状态根重定位的删除恢复记录、只读目录证据探测与 JSONL。
//   - session_meta_cache.go：ListSessions 的进程内 meta 读缓存，lstat 确认文件身份/大小/修改时间未变才复用上次安全读取的原始字节（每次重新解码），否则走 no-symlink 读取；Windows 无 lstat 文件身份时不缓存。
//   - permissions.go / confined_path.go / transcript_confined.go：enforce 新建权限、
//     owner/workspace 根绑定和 transcript confined-fd 访问。
//   - paths.go / transcript_path.go / transcript_project_hash.go / value_coerce.go：
//     路径、transcript 项目目录名、工程 hash、值转换。
//
// 结果与 assistant 的物理 Agent round 配对由 message.MergeHistoryResultSummaries 统一负责。
// 其余历史存取、读模型和源解析共享未导出模型，避免为拆目录暴露内部状态。
// TranscriptProjectDirectoryName/Names 与 RebaseSessionLifecycleRecords 是迁移层复用的稳定路径边界；
// ReadTranscriptSessionMessages 用受控 session id 读取独立 Agent thread，
// ReadTranscriptLinkMessages 是 Claude Code runtime 输出链接唯一允许的双重 confined 读取入口。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
// room_history_runtime.go 从读模型读取成员唤醒时的公区增量与最近终态并还原工具输出原文；游标定位不到或含图片 detail 时回退完整正文。
package workspace
