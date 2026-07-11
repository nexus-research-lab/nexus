# API 客户端

- 根目录只保留跨入口的 Launcher 客户端和领域目录，不提供聚合出口。
- `core/` 按请求准备、响应投影、错误类型、鉴权事件和协议时间转换拆分 HTTP 传输阶段。
- `agent/` 负责 Agent、workspace、私域和记忆快照接口。
- `account/` 负责登录、个人账户和订阅运营接口。
- `capability/` 负责技能、连接器、频道、Loop、定时任务和能力摘要接口。
- `conversation/` 负责 Session、Room、Goal 和子智能体任务接口。
- `settings/` 负责偏好、Provider、运行时和系统版本接口。

消费者直接导入职责文件；内部重组不保留旧路径转发层。
