# 权限运行时与连接生命周期规范

## 1. 文档目的

本文档定义 Nexus 中“Claude SDK 运行时”“WebSocket 连接”“交互式权限确认”三者之间的边界与生命周期规则。

这里要解决的不是某个 `WebSearch` 为什么被拒绝，而是更底层的几个问题：

- Claude SDK client 应该跟谁绑定
- WebSocket 断线重连后，权限请求应该如何继续
- 为什么 SDK client 不能直接持有某次连接的权限回调
- Room / DM / IM 通道应该如何共用同一套运行时模型

后续凡是涉及以下改动，都必须以本文档为准：

- `AgentRuntime`
- `PermissionStrategy`
- WebSocket 重连
- 权限请求重投递
- `session_key` 与前端路由绑定

## 2. 几类概念先分开

### 2.1 SDK client

- 指 `ClaudeSDKClient`
- 由 `session_key` 维度复用
- 它的生命周期长于单次 WebSocket 连接
- 它不应该直接感知浏览器连接对象

### 2.2 WebSocket 连接

- 指某一次浏览器到后端的临时连接
- 连接会断开、重建、切换
- 它只负责消息收发，不拥有 Claude 运行时本身

### 2.3 sender

- 指某个连接对应的消息发送器
- 它是连接级对象，不是会话级对象
- sender 失效不代表 SDK client 失效

### 2.4 PermissionStrategy

- 指 Claude SDK `can_use_tool` 依赖的权限策略
- 它是“权限决策接口”，不是“连接状态容器”
- 自动权限、交互式权限都应通过这个接口接入

### 2.5 PermissionRuntimeContext

- 指交互式权限运行时的全局上下文
- 它管理：
  - 待确认请求
  - 前端路由映射
  - session 绑定集合
  - 当前控制端 sender
  - 重连后的权限请求重投递
- 它是运行时级对象，不是连接级对象

### 2.6 route_session_key

- 指前端当前页面/面板真正订阅的会话键
- DM 下通常就是浏览器会话本身
- Room 下通常是共享 `room:*`
- 权限确认应该投递给它，而不是盲发给 SDK 私有运行时 key

### 2.7 runtime session_key

- 指 SDK client 自己运行时使用的 `session_key`
- DM 中通常和前端看到的是同一把 key
- Room 中通常是 Agent 私有运行时 key

### 2.8 pending permission request

- 指已经发起、但前端尚未确认的权限请求
- 它属于运行时上下文，不属于某个具体连接

## 3. 根问题定义

历史上最容易踩中的问题是：

1. 某个 WebSocket 连接创建了 `InteractivePermissionStrategy`
2. 某个 SDK client 在创建时把 `can_use_tool -> strategy.request_permission(...)` 闭包绑死
3. 浏览器连接断开并重连
4. SDK client 仍被 `session_key` 复用
5. 但它内部持有的仍然是旧连接创建出来的权限策略
6. 此时一旦再次触发工具权限，就会出现：

```text
Permission channel closed
```

结论：

- SDK client 生命周期和 WebSocket 生命周期天然不同步
- 所以 SDK client 绝不能直接持有某次连接的权限状态

## 4. 设计目标

统一规范必须长期满足以下目标：

1. SDK client 可以按 `session_key` 稳定复用
2. WebSocket 可以随时断开和重连
3. 权限确认请求在重连后可以继续，而不是直接报废
4. 交互式权限与自动权限可以共存
5. Room 的共享前端路由与 Agent 私有运行时必须显式分开
6. 前端必须能显式声明“当前连接服务哪个 session”

## 5. 目标架构

目标结构如下：

```text
Claude SDK client
  -> PermissionStrategy
     -> InteractivePermissionStrategy（薄适配层）
        -> PermissionRuntimeContext（全局生命周期上下文）
           -> route_session_key -> bound senders
           -> route_session_key -> controller sender
           -> runtime session_key -> route context
           -> pending permission requests
```

重点原则：

- SDK client 只依赖 `PermissionStrategy`
- 交互式权限策略只做适配，不自己持有生命周期状态
- 生命周期状态统一进 `PermissionRuntimeContext`

## 6. 职责边界

### 6.1 AgentRuntime

负责：

- 按 `session_key` 获取或创建 SDK client
- 配置 Claude SDK 运行参数
- 复用已有 client

不负责：

- 保存 WebSocket 连接状态
- 保存 pending permission request
- 判断当前控制端是谁

### 6.2 InteractivePermissionStrategy

负责：

- 实现 `PermissionStrategy` 协议
- 把权限请求委托给 `PermissionRuntimeContext`
- 提供 `bind_session_route` / `cancel_requests_for_session` 这类薄接口

不负责：

- 持有连接级 sender 生命周期
- 保存全局 pending request 状态

### 6.3 PermissionRuntimeContext

负责：

- 保存待确认权限请求
- 保存运行时 session 到前端 route session 的映射
- 保存 route session 的全部绑定连接
- 维护 route session 当前控制端
- 在重连后重新投递仍待确认的权限请求
- 在前端返回 `permission_response` 后唤醒等待中的请求

### 6.4 WebSocket 层

负责：

- 建立和关闭连接
- 在连接存活期间把当前 sender 绑定到 `route_session_key`
- 显式声明当前连接对应的 `client_id`
- 在需要交互时声明当前连接是否请求控制权
- 重连后重新声明当前连接绑定的 session

不负责：

- 决定权限请求是否允许
- 持有 Claude SDK client 生命周期

## 7. 生命周期规则

### 7.1 创建 SDK client

- `AgentRuntime` 按 `session_key` 获取或创建 SDK client
- `can_use_tool` 只调用传入的 `PermissionStrategy`
- 不允许在 `AgentRuntime` 中直接感知 WebSocket sender

### 7.2 发起权限请求

当 Claude SDK 请求工具权限时：

1. `PermissionStrategy.request_permission(...)` 被调用
2. 若是交互式策略，则转发给 `PermissionRuntimeContext`
3. `PermissionRuntimeContext` 根据 `runtime session_key` 找到 `route_session_key`
4. 再根据 `route_session_key` 找到当前控制端 sender
5. 无论 DM 还是 Room，`permission_request` 都只发给控制端
6. 观察端只接收普通消息流和运行态同步，不接收可提交的权限卡

### 7.3 WebSocket 断开

连接断开时：

- sender 失效
- route 与 sender 的绑定被注销
- 若断开的是控制端，必须从剩余绑定中晋升新的控制端
- 已存在的 pending permission request 不应直接丢弃
- SDK client 不应因为 sender 失效而立即销毁

### 7.4 WebSocket 重连

连接重连后：

1. 前端必须重新声明“当前连接服务哪个 session”
2. 后端更新 `route_session_key -> bound senders`
3. 当前可见窗口可以在 `bind_session` 中请求控制权
4. 后端更新控制端后，必须向该 session 全部绑定连接广播新的 `session_status`
3. `PermissionRuntimeContext` 应重新投递仍在等待中的权限请求

### 7.5 用户确认权限

- 前端提交 `permission_response`
- 后端根据 `request_id` 找到 pending request
- 只有当前控制端允许提交 `permission_response`
- 唤醒等待中的 Claude SDK 权限回调

### 7.6 问答交互不是普通权限条

- `AskUserQuestion` 仍复用 `permission_request -> permission_response` 这条链
- 但它的前端交互模式必须标记为 `question`
- `question` 模式的目的只有两件事：
  - 复用 `request_id`
  - 回传 `user_answers`
- 前端不能再为它额外渲染一条通用的 `允许 / 拒绝` 权限条
- Room 主时间线若需要暴露入口，只能提供 `去回答`
  - 作用是打开对应 Thread
  - 不能直接发送 `allow`
- 若同一会话内同一 Agent 重试了同一条 `AskUserQuestion`，新的请求应替换旧的挂起请求
- `AskUserQuestion` 的 `allow` 必须携带 `user_answers`，不能发送空确认
- `AskUserQuestion` 若超时或通道不可用，应直接中断当前运行，不再继续重试

### 7.7 用户中断

- 若某轮被中断
- 与该运行时 `session_key` 相关的 pending permission request 应一起取消
- 取消语义属于运行时级别，不属于连接级别
- 只有当前控制端允许发起 `interrupt`
- `permission_response.allow` 只表示权限已放行，不表示 round 已结束
- 权限通过后，前端仍必须保持执行态，直到后端推送 terminal `round_status`

## 8. 前后端协议要求

### 8.1 `permission_request`

后端发给前端的权限请求事件必须包含：

- `request_id`
- `session_key`
- `tool_name`
- `tool_input`
- `interaction_mode`
- `expires_at`
- 与当前运行 round 的关联元信息：
  - `caused_by`
  - `agent_id`
  - 若当前链路能稳定拿到具体消息锚点，再带 `message_id`
- Room 场景下还必须携带 `room_id / conversation_id`

补充规则：

- DM 与固定 Session 也必须携带 `caused_by=round_id`
- 不能只有 Room 链路才带 round 关联元信息
- 否则 `AskUserQuestion` 超时、拒绝或中断后，前端无法把残留权限状态与 terminal `round_status` 正确收口
- `AskUserQuestion` 若最终拒绝原因为超时或权限通道不可用
  - 后端必须在对应 `tool_result` 上补结构化 `error_code`
  - 前端只允许基于该 `error_code` 渲染 `提问已超时 / 提问未完成`
  - 禁止再用英文错误串做 UI 分支

前端绑定规则：

- 主绑定链必须先按 `permission.message_id` 缩到同一条 assistant message
- 若同一条 assistant message 内有多个 `tool_use`
  - 再按 `tool_name + tool_input` 精确定位唯一调用
- 禁止恢复旧的“跨 message 签名队列”匹配
  - 不能只靠工具名或命令文本跨消息猜测归属

### 8.2 `permission_response`

前端回给后端的权限确认必须包含：

- `request_id`
- `decision`
- `session_key`

可选：

- `message`
- `interrupt`
- `user_answers`
- `updated_permissions`

### 8.3 `bind_session`

WebSocket 重连后，前端必须主动发送：

```text
type = bind_session
session_key = 当前页面活跃 session_key
client_id = 当前浏览器标签页稳定 ID
```

用途：

- 把新连接重新绑定到当前会话
- 让挂起中的权限请求有机会重投到新连接
- 让后端按 `last_seen_session_seq` 回放断线期间的 DM 增量

补充字段：

```text
last_seen_session_seq = 当前前端已收到的最后一个 session envelope 序号
request_control = 当前窗口是否显式请求成为控制端
```

规则：

- 若后端缓冲区仍覆盖该序号，按序补发 `message / stream / round_status`
- 若缓冲区已不完整，后端返回 `session_resync_required`
- 前端收到 `session_resync_required` 后，执行当前会话全量重拉
- `bind_session` 结束后，后端仍需补推一次当前 `session_status`
  - `session_status` 必须包含当前权威 `running_round_ids`
  - `session_status` 必须包含当前 `controller_client_id`
  - `session_status` 必须包含当前 `observer_count`
  - 该事件只负责同步“此刻还有哪些 round 正在运行”
  - 不得替代 durable 的 `round_status`
- 当控制端切换时，也必须再次广播 `session_status`

注意：

- `bind_session` 只做绑定，不触发业务消息处理
- 同一 session 允许多个连接同时 `bind_session`
- 同一 session 同时只能存在一个控制端

## 9. Room 特殊规则

Room 下要显式区分两把 key：

### 9.1 共享路由 key

- 前端主面板通常绑定 `room:*`
- 权限确认应该投递给这把共享路由 key

### 9.2 Agent 私有运行时 key

- Claude SDK client 通常跑在 `agent:*:ws:group:*`
- 这是实际执行工具的运行时 key

因此 Room 权限链必须做一次路由映射：

```text
runtime session_key -> route_session_key
```

不允许省略这层映射，否则权限请求会发到错误的前端面板。

除此之外还必须满足：

- Room 普通消息、成员变更、协作广播继续走 `subscribe_room`
- Room 权限请求不再走 room 广播，只发给该 route session 的控制端
- `bind_session` 负责恢复 route session 绑定与控制权，不依赖 `subscribe_room` 侧带
- 只要控制端重新绑定成功，后续权限卡就必须继续可见
- Room Thread 在绑定权限请求时，不能再把 `message_id` 当成唯一归属条件
  - 因为部分权限事件的 `message_id` 实际是 slot `msg_id`
  - 归属应优先依赖：
    - `agent_id`
    - `caused_by`
  - 这个回退只允许发生在 Room Thread 过滤阶段
  - 不能回退成全局签名匹配

### 9.3 AskUserQuestion 多选规则

- `AskUserQuestion` 可能显式声明多选
- 前端必须同时兼容：
  - `multi_select`
  - `multiSelect`
- 一旦命中多选模式：
  - UI 必须允许同一题选择多个选项
  - `permission_response.allow` 必须回传完整 `user_answers`

## 10. DM 特殊规则

DM 下通常：

- `runtime session_key == route_session_key`

但即使两者相同，也不能省略生命周期上下文。

原因：

- 问题的本质不是 key 是否相同
- 而是“当前绑定集合与控制端”会随着 WebSocket 重连和多窗口切换而变化

## 10.1 多观察者、单控制者规则

同一 `route_session_key` 允许多个前端连接同时在线，但必须满足以下约束：

- 所有绑定连接都是观察者候选
- 其中恰好一个连接是控制端
- `message / stream / round_status / session_status` 必须 fan-out 给全部绑定连接
- `permission_request / permission_response / chat / interrupt` 只允许控制端处理
- 控制端断开或解绑后，后端必须在剩余绑定中自动晋升新的控制端
- 只有显式 `bind_session(request_control=true)` 时，才允许抢占控制权

推荐交互语义：

- 新绑定窗口默认作为观察视图加入
- 当前控制端保持稳定，直到断开、解绑或显式交出/被显式抢占
- 观察视图可以实时看消息，但不能发送消息、停止生成或确认权限

## 11. 自动权限通道规则

Telegram、Discord、内部自动执行等场景可能使用自动权限策略。

这些通道必须遵循：

- 自动权限继续通过 `PermissionStrategy` 接口实现
- 不强制依赖 `PermissionRuntimeContext`
- 不因为 WebSocket 权限重连逻辑而被误伤

结论：

- 生命周期上下文只服务于交互式权限
- 不应把所有通道硬绑进同一套交互式连接模型

## 12. 明确禁止的做法

以下做法明确禁止：

### 12.1 让 SDK client 闭包持有某次 WebSocket 连接对象

- 这是导致重连后 `Permission channel closed` 的根因

### 12.2 连接断开时直接清空所有 pending permission request

- 用户可能只是短暂断线
- 权限卡片应允许在重连后继续确认

### 12.3 重连后依赖 `ping` 或业务消息“顺便绑定 session”

- 绑定必须显式
- 不能依赖隐式副作用

### 12.4 因 sender 断开就直接销毁 SDK client

- 这会破坏运行时复用
- 也会让长任务无法跨短暂断线继续

### 12.5 把交互式权限上下文强行用于 Telegram / Discord 自动权限

- 不同通道的权限语义不同
- 必须保留策略可插拔

### 12.6 把同一 session 的权限卡广播给所有观察端

- 这会导致多窗口重复确认
- 会把控制权仲裁退化成前端竞态

### 12.7 让非控制端继续发送 `chat / interrupt / permission_response`

- 这会破坏单控制端约束
- 也会让多窗口行为不可预测

## 13. 当前目标态

当前推荐目标态如下：

- `AgentRuntime` 继续按 `session_key` 复用 SDK client
- `InteractivePermissionStrategy` 退成薄适配层
- `PermissionRuntimeContext` 统一管理权限请求生命周期
- 前端在 WebSocket 重连后显式发送 `bind_session`
- 同一 session 支持多观察者，但只允许一个控制端
- Room 继续使用 `runtime key -> route key` 的双层映射
- 自动权限通道继续通过各自策略工作

后续如果再扩展：

- 多端同时在线
- 一个 session 多个观察者
- 显式控制端切换
- 权限请求转移给其他控制端

也必须以这套生命周期模型继续扩展，而不是回退到“把权限状态绑在连接对象里”。
