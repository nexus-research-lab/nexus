# hooks/agent/

L3 | 父级: ../CLAUDE.md

## 职责边界

- `use-agent-conversation.ts`: 公共装配入口，只组合领域控制器并投影公开返回值
- `index.ts`: 对外唯一导出入口
- `message/`: Assistant 内容身份、消息集合和流式事件各自维护纯数据模型
- `actions/`: 用户命令、协议请求构造和发送 ACK 生命周期
- `session/`: 会话键迁移、历史窗口、后台消息缓存与易失快照
- `runtime/`: 后端运行态、轮次、权限与 Room slot 的唯一前端投影
- `transport/`: WebSocket 连接、信封校验、事件路由和流式缓冲

跨层依赖只能指向稳定的数据函数或小接口；组件只能从 `index.ts` 使用公开 Hook。

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
