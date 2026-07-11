# hooks/agent/

L3 | 父级: ../CLAUDE.md

## 职责边界

- `use-agent-conversation.ts`: 公共装配入口，只组合消息、动作、会话、运行态与传输控制器并投影公开返回值
- `message/`: Assistant 内容身份、消息集合和流式事件各自维护纯数据模型
- `actions/`: 用户命令、协议请求构造和发送 ACK 生命周期
- `session/`: 会话键迁移、历史窗口，以及 `controller/` 下的身份、后台消息和易失快照装配
- `runtime/`: 后端运行态、轮次、权限与 Room slot 的唯一前端投影
- `transport/`: WebSocket 连接、信封校验、稳定分发，以及 `handlers/` 下按协议事件族拆分的路由处理器

跨层依赖只能指向稳定的数据函数或小接口；消费者直接导入公共装配入口，不增加单纯转发的目录出口。
消息集合去重、ACK 失败和事件路由生命周期必须留在所属子域，不得回流公共装配入口。

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
