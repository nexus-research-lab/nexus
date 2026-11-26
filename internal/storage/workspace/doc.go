// Package workspace 是 Agent 历史、transcript、输入队列与房间历史的持久化层。
//
// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//
// 成员清单：
//   - input_queue_recovery.go：持久化 dispatch pause/claim、带暂停 revision 栅栏的单项显式恢复准入与 owner-scoped canonical 队列发现；下游受理后才消费条目。
//   - agent_history.go / agent_history_overlay.go / room_history.go：完整回复落盘后更新独立 Room SQL 摘要，群聊只接受公区写入；失败记日志，不重试 canonical 消息。
//   - history_*.go / user_input_probe.go：分阶段历史投影与严格只读用户输入判定（compact / normalize / causal control ordering / pagination / last-write round_index /
//     rewrite_tail（区分未物化 round 与损坏边界）/ fork marker 前缀 / 按 Agent 执行轮配对的 result_summary / 同 message_id 合并 Goal 完成收据 / external_delivery / 不为完成态 host control 伪造中断 assistant 的 unfinished_round）。
//   - history_page_index.go / agent_history_page_index.go / room_history_page_index.go：DM/Room 共用的 round 分页编排、有界 exact-scope rebuild admission、source preflight/resolver selection 依赖与删除栅栏；每次全量重建记录耗时、分组数与落盘结果。
//   - history_read_model.go / history_read_model_memo.go / history_message_detail.go：宿主 SQLite/B-Tree 派生读模型，以原子 generation、source/payload 校验、有界 metadata window、淘汰、整库版本重建和大型 Tool/图片按需 detail 提供按页读取；
//     重建沿用上一代际并只重写签名变化的分组，进程内分组结构哈希备忘让未变化分组跳过投影与序列化；
//     round index 按固定 round 数分块保存，全量重建与尾部刷新都只写内容变化的块（进程内摘要凭 scope 行的 round_revision 确认库内仍是自己写入的那一版，否则回读各块）；canonical 历史不迁移。
//   - history_read_tail.go：DM 与 Room 增量刷新共用的尾部状态持久化：每个 scope 一份尾部原始行与状态 JSON，以及已冻结前缀的身份键集合，与分组在同一事务内发布。
//   - history_read_tail_test.go：身份键批量写入后按原值可查的回归，覆盖引号、NUL、多字节与非法 UTF-8。
//   - agent_history_incremental.go：DM 每轮刷新从读模型里的尾轮状态与 transcript 检查点续接，只重新规范化尾部分组，
//     只物化仍在尾部分组里与新增的 round marker；进程重启后同样成立，无法证明前缀不变时回退重建。
//   - history_long_session_bench_test.go：长会话逐轮追加（整页刷新与单独的 overlay 读取）、热读、冷读与重启后追加的基准（NEXUS_BENCH_ROUNDS 调整轮数）。
//   - room_history_checkpoint.go / room_transcript_stream.go：大 Room 按成员和公开消息 ID 回填可重建检查点，按文件区间读取主链与并行工具，源指纹变化后重新投影。
//   - room_history_checkpoint_test.go：覆盖多成员恢复、索引失效与工具详情，并提供桌面规模的历史读取基准。
//   - room_history_incremental.go：在共用后台单飞任务中按 ledger 偏移读取追加，保留原始尾轮，原子更新尾部分页、导航与检查点；依赖身份替换和跨轮修正回退重建。
//   - room_history_frozen_refs.go：为已离开尾轮的 transcript_ref 保存投影摘要与成员 transcript 检查点；成员只追加时从检查点续读并只核对新增内容，
//     无法证明检查点之前不变时完整读取并比对摘要，已发布消息投影变化则回退重建。
//   - room_history_frozen_refs_test.go：覆盖重启后续读、流式轮次、引用落在检查点之前或无换行末行，以及原地改写、分支切换、消息续写、marker 改写与重新归属的回退。
//   - room_long_session_bench_test.go：长 Room 逐轮追加、冷读与重启后追加的基准（NEXUS_BENCH_ROUNDS 调整轮数），夹具供冻结引用回归复用。
//   - agent_history*.go：Agent 历史门面、K3 工具面换代后的分段 transcript lineage 读取、overlay 与共享模型；
//     round marker 携带受理游标 input_seq 并原样物化到用户行，Room inline 用户行自带同一字段，合并同 ID 用户快照时不丢失。
//   - history_input_cursor_test.go：DM/Room 用户行的受理游标在实时行、分页读模型与完整读取之间一致，刷新与进程重启后不变。
//   - overlay_replay.go：session overlay 的进程内增量回放（overlay 历史行/marker 状态与 DM round index 共用）；按文件身份、尺寸、
//     同尺寸修改时间与首尾指纹确认纯追加后只解析新增完整行，文件被替换/截断/改写、末行未写完或新行覆盖已回放的 message_id 时整份重读；
//     回放状态按最近使用保留，受会话个数与源文件总字节上限约束。
//   - runtime_repair.go：enforce 模式下 owner runtime 权限修复与受限重试；transcript reader 与分页指纹共用修复入口。
//   - transcript_*.go：基于文件身份/大小/mtime/首尾指纹且由普通、分段、显式读取共用的 JSONL 解析缓存（纯追加只解析新增字节并沿用解析代系；完整解析大文件时按换行分块并行，行号与顺序解析一致）、
//     进程内 DM 投影前缀检查点（同代系主链与 marker 前缀不变时只重放最后一个用户轮次之后的条目）、
//     持久化的 DM 投影位置（transcript_checkpoint.go：轮次起点的行号、字节偏移与标量投影状态，续读只解析偏移之后的完整行，
//     文件身份、首尾指纹、起点条目、已冻结身份或 marker 前缀任一不符即回退重建）、
//     marker 对齐边界（transcript_marker_frontier.go：检查点只保存起点之前尚未定型的轮次与 marker，续读只重新对齐它们和新增部分，
//     结果与全局对齐一致，成本不随历史轮次数增长）、重复 UUID/自指链修复、reader、path、session、project、
//     可见性安全的 marker 对齐（含 reminder 提取后的空白 Goal 续跑）、guidance 与 root/source round 投影；
//     投影与 rewrite/fork 共用边界，派生读模型升级时清除旧轮次归属。
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
// 其余历史存取、分页索引和源解析共享未导出模型，避免为拆目录暴露内部状态。
// TranscriptProjectDirectoryName/Names 与 RebaseSessionLifecycleRecords 是迁移层复用的稳定路径边界；
// ReadTranscriptSessionMessages 用受控 session id 读取独立 Agent thread，
// ReadTranscriptLinkMessages 是 Claude Code runtime 输出链接唯一允许的双重 confined 读取入口。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
// room_history_runtime.go 复用游标索引读取 runtime 公区增量与最近终态；普通工具详情在同一事务中恢复，缺游标、图片详情或无可用索引时回退完整正文。
package workspace
