# Runtime 人工交互规范

## 1. 文档目标

本文档定义 Nexus 中阻塞式人工交互、runtime client、WebSocket 连接三者之间的边界。协议因兼容 SDK 仍使用 `permission_request` / `permission_response`，产品语义不局限于权限。

目标很简单：

- runtime client 可复用
- 前端连接可重连
- 等待用户响应的请求不会因为连接切换而失效
- Room 公区始终是解除执行阻塞的主入口

## 2. 核心概念

### 2.1 runtime session

- 指某个 agent 私有运行时
- 由 `session_key` 标识
- 可以绑定 `sdk_session_id`

### 2.2 route session

- 指前端当前实际订阅和展示的会话
- DM 下通常等于 runtime session
- Room 下通常是共享 `room:*`

### 2.3 sender

- 某一次前端连接对应的发送器
- 是连接级对象，不是运行时级对象

### 2.4 controller

- 某个 route session 当前拥有控制权的 sender
- 只有 controller 可以：
  - 发送消息
  - 停止生成
  - 提交人工交互响应

### 2.5 pending human interaction

- 已发出且 runtime 正在等待用户响应的请求
- 属于运行时上下文
- 不属于某一次连接
- 当前包括工具批准/拒绝、结构化问答和计划确认；后续新增类型也必须进入同一投影

## 3. 当前架构

### 3.1 runtime 复用规则

- runtime client 按 `session_key` 复用
- runtime client 不直接持有 sender
- runtime client 只依赖权限策略接口

### 3.2 权限上下文规则

人工交互运行时上下文统一负责：

- `runtime session -> route session` 映射
- `route session -> senders` 绑定
- `route session -> controller` 归属
- pending request 生命周期

### 3.3 连接规则

- 一个 route session 可以有多个观察者
- 同时只有一个 controller
- controller 断开后，系统需要重新确定控制端

## 4. 人工交互流程

1. runtime 触发一个需要用户响应的请求
2. 权限上下文根据 runtime session 找到 route session
3. 请求只投递给当前 controller
4. controller 返回 `permission_response`
5. 后端唤醒对应等待中的 runtime 请求

`interaction_mode` 只决定公区使用批准控件还是结构化输入控件，不决定请求是否属于人工介入。未知工具和未知批准类请求必须回退到可批准/拒绝的通用控件。

## 5. 重连规则

### 5.1 断开

- sender 解绑
- pending request 不直接销毁
- runtime client 不因为 sender 断开而销毁

### 5.2 重连

重连后前端必须重新声明：

- 当前绑定哪个 session
- 是否请求控制权

系统据此恢复：

- sender 集合
- controller
- 待处理人工交互的投递目标

## 6. Room 特殊规则

Room 中必须分开两件事：

- 共享会话路由：`room:group:<conversation_id>`
- agent 私有运行时：`agent:<agent_id>:ws:group:<conversation_id>`

请求来源于私有 runtime，但展示和交互挂到共享 route session。

Room 公区是所有阻塞式人工交互的权威处理面：

- 权限模式不会改变投影位置；只要 runtime 实际在等待用户，公区就必须展示
- 工具审批、结构化问答、计划确认和未知新工具都适用
- 请求先于 Agent 消息到达、消息已完成或仍在活动 slot 中，都不能丢失公区入口
- Thread 可以同步完整交互，但只能作为详情镜像，不能成为唯一处理入口

## 7. 当前实现约束

- WebSocket 入口固定：`/nexus/v1/chat/ws`
- runtime 继续按 `session_key` 复用
- 人工交互请求只发给控制端，不广播给全部观察者
- `session_status` 负责同步运行态和控制端归属，不负责定义消息历史

## 8. 禁止项

- runtime client 直接持有 sender
- 用某次连接对象承载 pending human interaction 生命周期
- 观察者窗口提交人工交互响应
- Room 中把共享 session 当成某个 agent 的私有 runtime
- 根据工具名决定请求是否进入 Room 公区
- 让 Thread 成为任何阻塞式人工交互的唯一处理入口

## 9. 一句话总结

人工交互系统的关键不是“它属于哪种工具”，而是：

- runtime 可以长期存在
- 连接可以随时换
- 等待用户响应的请求总能重新找到当前控制端
- Room 用户无需进入 Thread 就能解除执行阻塞
