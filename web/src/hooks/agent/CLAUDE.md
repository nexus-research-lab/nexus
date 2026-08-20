# hooks/agent/

L3 | 父级: ../CLAUDE.md

## 职责边界

- `use-agent-conversation.ts`: 公共装配入口，只组合消息、动作、会话、运行态与传输控制器
- `agent-conversation-model.ts`: 使用默认身份哨兵归一化 options，并将内部控制器投影为公开 Hook 返回协议
- `message/`: Assistant 内容身份、消息集合和流式事件各自维护纯数据模型
- `actions/`: 用户命令、协议请求构造和发送 ACK 生命周期
- `session/`: 会话键迁移、历史窗口，以及 `controller/` 下的身份、后台消息和易失快照装配
- `runtime/`: 后端运行态、轮次、权限、Room slot 与精确 Agent stopping/terminal 竞态的唯一前端投影
- `transport/`: WebSocket 连接、信封校验、稳定分发，以及 `handlers/` 下按协议事件族拆分的路由处理器；`execution_invalidated` 只在 envelope session 匹配当前会话后触发 WorkGraph 重新读取；ACK/明确拒绝先按本地 `client_request_id` 所有权收口原请求，再按 envelope session 决定是否投影当前 Feed/Error
- `command_catalog` 是后端从当前 Nexus 版本内置的 nxs、Claude 清单、Nexus host/固定产品命令与 owner 命名 WorkGraph Workflow 合成的权威快照；`bind_session` 只按当前 Agent runtime 选择清单，不启动 session 或 runtime。Composer 只消费事件，不查询目录、不启动 runtime、不按浮层打开或 idle 轮次刷新；Workflow 目录显式刷新/删除后只通过一次相同 Session rebind 请求新快照。文本 `/goal` 与 UI `set_goal` 在后端合流为同一 host command；前端只为两者投影同一种 optimistic 控制记录，不把它们当普通模型输入。固定 `/workgraph` 与动态 `/<workflow>` 仍作为普通 Slash 文本提交，Composer 不解释 runtime 内部元数据。

跨层依赖只能指向稳定的数据函数或小接口；消费者直接导入公共装配入口，不增加单纯转发的目录出口。
公开类型由 `types/agent/agent-conversation.ts` 持有，装配入口不得重复转发类型。
消息集合去重、ACK 失败和事件路由生命周期必须留在所属子域，不得回流公共装配入口。

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
