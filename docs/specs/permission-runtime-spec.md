# 权限运行时规范

## 1. 文档目标

本文档定义 Nexus 中权限请求、runtime client、WebSocket 连接三者之间的边界。

目标很简单：

- runtime client 可复用
- 前端连接可重连
- 权限请求不会因为连接切换而失效

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
  - 提交权限确认

### 2.5 pending permission request

- 已发出但尚未确认的权限请求
- 属于运行时上下文
- 不属于某一次连接

## 3. 当前架构

### 3.1 runtime 复用规则

- runtime client 按 `session_key` 复用
- runtime client 不直接持有 sender
- runtime client 只依赖权限策略接口

### 3.2 权限上下文规则

权限运行时上下文统一负责：

- `runtime session -> route session` 映射
- `route session -> senders` 绑定
- `route session -> controller` 归属
- pending request 生命周期

### 3.3 连接规则

- 一个 route session 可以有多个观察者
- 同时只有一个 controller
- controller 断开后，系统需要重新确定控制端

## 4. 权限请求流程

1. runtime 触发工具权限请求
2. 权限上下文根据 runtime session 找到 route session
3. 请求只投递给当前 controller
4. controller 返回 `permission_response`
5. 后端唤醒对应等待中的 runtime 请求

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
- 待确认权限卡投递目标

## 6. Room 特殊规则

Room 中必须分开两件事：

- 共享会话路由：`room:group:<conversation_id>`
- agent 私有运行时：`agent:<agent_id>:ws:group:<conversation_id>`

权限请求来源于私有 runtime，但展示和交互通常挂到共享 route session。

## 7. 当前实现约束

- WebSocket 入口固定：`/nexus/v1/chat/ws`
- runtime 继续按 `session_key` 复用
- 权限请求只发给控制端，不广播给全部观察者
- `session_status` 负责同步运行态和控制端归属，不负责定义消息历史

## 8. 禁止项

- runtime client 直接持有 sender
- 用某次连接对象承载 pending permission 生命周期
- 观察者窗口提交权限确认
- Room 中把共享 session 当成某个 agent 的私有 runtime

## 9. 一句话总结

权限系统的关键不是“谁发了请求”，而是：

- runtime 可以长期存在
- 连接可以随时换
- 权限请求总能重新找到当前控制端
