# Runtime 人工交互规范

定义阻塞式人工交互、runtime client、WebSocket 连接三者的边界，以及活跃 Session 的 MCP 工具面。协议为兼容 SDK 仍使用 `permission_request` / `permission_response`，产品语义不限于权限。

设计目标：runtime client 可复用；前端连接可重连；等待用户响应的请求不因连接切换而失效；Room 用户无需进入 Thread，在公区即可解除执行阻塞。

## 1. 核心概念

| 概念 | 定义 |
| --- | --- |
| runtime session | 某个 agent 的私有运行时，由 `session_key` 标识，可绑定 `sdk_session_id` |
| route session | 前端实际订阅和展示的会话；DM 下通常等于 runtime session，Room 下通常是共享 `room:*` |
| sender | 某一次前端连接对应的发送器，是连接级对象，不是运行时级对象 |
| controller | 某个 route session 当前拥有控制权的 sender；只有它可以发送消息、停止生成、提交人工交互响应 |
| pending human interaction | 已发出、runtime 正在等待用户响应的请求；属于运行时上下文，不属于某次连接。包括工具批准/拒绝、结构化问答和计划确认，新增类型也必须进入同一投影 |

## 2. 架构规则

- runtime client 按 `session_key` 复用，不直接持有 sender，只依赖权限策略接口。
- 人工交互运行时上下文统一负责：`runtime session -> route session` 映射、`route session -> senders` 绑定、`route session -> controller` 归属、pending request 生命周期。
- 一个 route session 可以有多个观察者，同时只有一个 controller；controller 断开后需重新确定控制端。
- `session_status` 同步运行态和控制端归属，不定义消息历史。
- WebSocket 入口固定为 `/nexus/v1/chat/ws`。

## 3. 人工交互流程

1. runtime 触发需要用户响应的请求。
2. 权限上下文根据 runtime session 找到 route session。
3. 请求只投递给当前 controller，不广播给观察者。
4. controller 返回 `permission_response`。
5. 后端唤醒对应等待中的 runtime 请求。

### 3.1 控件与错误分类

- `interaction_mode` 只决定共享 Composer 使用批准控件还是结构化输入控件，不决定请求是否属于人工介入。
- 未知工具和未知批准类请求必须回退到可批准/拒绝的通用控件。
- 拒绝结果需要稳定分类时，权限决策必须在源头携带 `ErrorCode`。bridge 对原生 `nxs` 使用 `control_response.errorCode`，最终统一投影为 `tool_result.error_code`；不原生回传扩展字段的 runtime 由 bridge 按 `tool_use_id` 补齐。
- 消息层和前端不得通过匹配 `Message` 展示文案推断错误类型。

### 3.2 持久权限范围

- 持久权限范围只由 runtime 的 `suggestions` 决定。
- 存在建议时，Composer 把真实规则动作收进“允许本次”旁的下拉：动作行说明新增或修改什么权限规则；次级说明展示匹配内容与 runtime 指定的 Agent/项目/用户/会话范围。
- 选择后在允许响应中原样回传对应 `updated_permissions`。
- `suggestions` 为空时只能允许本次；前端和宿主不得推断或合成永久规则。

### 3.3 沙箱越界（`sandbox_escape`）

- 受限模式下，以下动作必须先进入人工审批：Bash/PowerShell 的显式沙箱外执行；Write/Edit 写入普通工作区外文件。
- 允许后只对当前一次精确动作临时授权：不扩大为目录范围，不写入 Agent 权限设置，也不能经 `updated_permissions` 或修改后的工具输入扩大。
- 隐藏路径、符号链接祖先、只读资源和宿主配置声明的受保护路径继续拒绝。
- Read、Glob、Grep、ViewImage 等读取工具按各自读取审核规则处理，不属于写入白名单。

## 4. 重连规则

- 断开：sender 解绑；pending request 不销毁；runtime client 不销毁。
- 重连：前端必须重新声明当前绑定哪个 session、是否请求控制权。系统据此恢复 sender 集合、controller 和待处理人工交互的投递目标。

## 5. Room 规则

Room 中必须区分：

- 共享会话路由：`room:group:<conversation_id>`
- agent 私有运行时：`agent:<agent_id>:ws:group:<conversation_id>`

请求来源于私有 runtime，但展示和交互挂到共享 route session。Room 底部 Composer 是所有阻塞式人工交互的权威处理面：

- 只要 runtime 实际在等待用户，Composer 就必须原位替换输入内容；权限模式不改变投影位置。
- 适用于工具审批、结构化问答、计划确认和未知新工具。
- 请求先于 Agent 消息到达、消息已完成或仍在活动 slot 中，都不能丢失 Composer 入口。
- 多 Agent 请求按首次到达顺序在同一 Composer 逐项接棒，并按 `request_id` 路由回原 Agent。
- 公区消息与 Thread 只保留不可操作的等待证据，不能出现第二组批准、拒绝或回答控件。

## 6. MCP 工具面

- 活跃 Nexus Session 的 MCP 工具面只由稳定会话拓扑和用户显式 MCP/Connector 选择决定。
- 内部唤醒、私域回传、Room 角色、WorkBinding/ReviewBinding、Goal authority 与通讯开关只改变逐轮执行权限，不得通过卸载 schema 鉴权。
- 无权轮次必须保留工具定义，并在 service 真相源 fail closed。
- `ToolSearch` 只是默认关闭的 schema 传递优化，不参与挂载或鉴权。

## 7. 禁止项

- runtime client 直接持有 sender。
- 用某次连接对象承载 pending human interaction 生命周期。
- 观察者窗口提交人工交互响应。
- Room 中把共享 session 当成某个 agent 的私有 runtime。
- 根据工具名决定请求是否进入 Room 公区。
- 让 Thread 成为任何阻塞式人工交互的唯一处理入口。
