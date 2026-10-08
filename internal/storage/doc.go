// Package storage 负责数据库连接打开与 migration 目录/方言解析。
//
// L2 | 父级: internal（L1 见 AGENTS.md）
//
// 成员清单：
//   - database.go：OpenDB 打开带 immediate transaction、busy timeout 与外键保护的运行连接；OpenMigrationDB 提供 migration 专用连接。
//   - dialect.go：MigrationDirName / GooseDialect 解析驱动的迁移目录与 goose 方言。
//   - time_value.go：统一解析 SQLite/PostgreSQL 聚合 deadline 返回的可空时间值。
//   - room_lock.go：跨 repository 的 Room-first mutation 行锁协议。
//   - queueadmission/：保存不受 Agent workspace 控制、与精确队列 payload 和物理会话绑定的一次性直接用户配置 admission。
//   - configuration/：仅供宿主读取的配置 revision 持久密钥，原子初始化且缺失/损坏不重新生成。
//   - channelauthorization/：保存 owner-main 私有 DM 发起的 Channel 授权 flow、
//     加密短期展示材料与不含秘密的不可变 completion audit。
//   - connectors/：OAuth client 与人类批准的 durable Connector authorization flow，
//     包括加密临时凭据、轮询领取和事务终态。
//   - skills/：保存 owner Skill catalog、持久单调版本及跨进程 mutation transaction。
//   - teamrelay/：按 deployment 幂等保存一份 Relay Message 与 room_seq，并为各 owner 保存独立恢复游标。
//   - sandbox/：保存 owner/session/generation 绑定的桌面沙箱 effective-policy 回执、启动意图、一次性放行与原生集合回收事实，以及仅供持锁宿主使用的有界 pending key 扫描、资源完成阶段和终态后的资源/策略待办扫描。
//
// 暴露接口：OpenDB、OpenMigrationDB、MigrationDirName、GooseDialect、
// NullableTime、LockRoomForMutation，以及各领域 Repository 构造器（含 sandbox.NewRepository）。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package storage

// sandbox 资源恢复记录固定原目录并单调推进 prepared/quarantined/deleting/complete；pending 阻断新进程，策略只接受原进程与资源双重收口。
// sandbox 启动意图保留可选原 scratch parent/leaf 身份；只有宿主创建句柄可提供，任务可写 marker 不构成该事实。
// sandbox 策略回执另存不可变原 runtime launch key；warm 策略代次不等同于原进程代次，旧记录不补猜关联。
// sandbox 进程登记在同 owner/session/generation 内按探测到 runtime 的固定顺序保存独立启动，唯一活跃栅栏不变；旧登记用途为空时视为 runtime。
