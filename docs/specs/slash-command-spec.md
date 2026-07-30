# Slash 指令统一协议

## 目标

Nexus 同时承载 nxs 与 Claude Code（CC）两种 runtime。两者都把可由用户
输入派发的 Slash 指令声明在初始化快照中，并都通过普通用户文本执行指令。
Nexus 需要在不让浏览器理解 runtime 私有协议的前提下，把 runtime 指令和
Nexus 自有指令投影为一个稳定的 Composer 目录。

Claude Code 的 initialize control response 提供名称、说明和参数提示，是目录
真相源；随后 `system/init` 中的 `slash_commands` 只有名称，用于流内会话元数据，
不应反向覆盖已经缓存的完整描述。nxs 与其保持同一 control wire 形状。

## 权责边界

| 层 | 责任 | 不负责 |
| --- | --- | --- |
| nxs / Claude Code | 在 initialize 快照中声明 runtime 可派发指令；解析收到的 `/name args` 普通用户文本 | 识别或执行 Nexus host 指令 |
| `nexus-agent-sdk-bridge` | 统一两种 runtime 的初始化能力读取、普通文本发送和单轮隐藏上下文清理 | 合并 host 目录；发明 Slash RPC |
| Nexus `runtime.Manager` | 管理 session/runtime generation；runtime 启动成功后缓存一次能力快照 | 为补全请求启动子进程 |
| Nexus `service/slashcommand` | 注册、校验、按 DM/Room 作用域匹配和执行 host 指令 | 读取 runtime 私有 metadata |
| WebSocket handler | 在 bind 后启动 DM runtime（若尚未启动），合并 host/runtime 描述并广播完整快照 | 让前端查询目录或拼接隐藏上下文 |
| Web Composer | 只消费当前 session 的完整快照，选择后发送原始 Slash 文本 | 启动 runtime、查询目录、判断命令归属 |

Host 命令先进入合并结果，因此与 runtime 同名时 Nexus host 命令保留该名称，
runtime 同名项被丢弃。名称比较不区分大小写，展示名称统一为不带 `/` 的
canonical 名称。

## 生命周期

1. 浏览器发送 `bind_session`，只携带会话地址和已有的 Room/Agent 作用域信息。
2. WebSocket handler 立即发送当前缓存的 `command_catalog` 快照：
   - 没有 runtime 时为 `cold`；
   - runtime 已启动但目录同步尚未结束时为 `starting`；
   - 已有缓存时直接发送该 generation 的完整快照。
3. 对 DM Agent session，后端在绑定生命周期内调用 `EnsureRuntimeSession`。
   该调用只负责按真实会话配置连接 runtime，不创建 round、不发送假消息。
4. `runtime.Manager` 为新 client 分配单调递增的 generation，并在该 generation
   第一次连接成功后调用 bridge 的 `SupportedCommands`。读取结果缓存为
   `ready` 或 `unavailable`；后续 round、Composer 打开和目录事件不会再次访问
   runtime。
5. handler 把缓存 runtime 描述与对应作用域的 host 描述合并、脱敏、排序后，
   广播一条完整权威快照。runtime 被替换时 generation 变化，下一次启动流程
   重新同步并广播。
6. 浏览器按 session key、Agent 身份、generation 和状态顺序丢弃过期事件，只
   替换本地目录状态；浏览器不发送 `get_command_catalog` 或
   `ensure_runtime_session`。

绑定时的目录读取和 host 派发都经过当前 owner/session 校验；host registry 只在
命令匹配成功后调用鉴权器，未知 Slash 不会因为 host registry 的存在而改变
runtime 的错误或透传语义。

## 状态

`cold` 表示当前 session 尚未启动 runtime；`starting` 表示后端正在连接并同步；
`ready` 表示 runtime 目录已缓存（快照仍可包含 host 命令）；`unavailable` 表示
本 generation 没有可投影的 runtime 目录。Room 当前只公开 host 作用域，runtime
命令暂不投影，因此不应通过 Room 目录事件启动或绑定 Room runtime。

## 执行

Composer 选择任意 `host` 或 `runtime` 描述后，仍发送一条普通 `chat` 文本，例如
`/model sonnet`。WebSocket handler 先让 host registry 尝试匹配：

- 匹配成功：执行 host handler，返回其产生的产品事件；
- 未匹配：原样交给 DM runtime，由 nxs/Claude 自己解析；
- 带附件的已知 host 指令：在 handler 执行前拒绝；
- DM 的任何 Slash 输入都标记为 atomic，清除 bridge 尚未消费的隐藏上下文，
  不追加 Goal、recovery 或 emotion context。

未知 Slash 不在 Nexus 侧报错，以便 runtime 新增指令时旧版 Nexus 仍能透传。
Nexus 只有在 client 同时提供 set/clear 语义时才使用下一轮隐藏上下文 buffer；
产品适配层可用旧 bridge 的 `SetNextTurnContext(nil)` 实现逻辑 clear。其他缺少
清理能力的 client 会把上下文内联到当轮输入，不为后续 Slash 留下待消费残留。

## 后续扩展

- 若 runtime 支持运行中新增/删除 Slash，bridge 需要新增带 generation 的能力事件；
  在此之前只接受 initialize 快照。
- 若 Room 需要 runtime Slash，必须为每个 Agent slot 定义独立 runtime session
  generation，不能复用共享 Room host 目录。
- Host handler 若需要持久化消息或 round 状态，应返回带规范 session/round 身份的
  产品事件，由 handler 统一补齐缺省 session key。
