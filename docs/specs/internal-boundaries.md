# Internal 模块边界与治理验收

模块化单体的依赖与数据归属。状态：已实施并验收（2026-09-09）。

## 目标依赖

- HTTP、CLI、MCP 负责协议解析和入口鉴权，app 负责显式装配。
- 业务服务持有状态转换与跨域流程；仓储持有本领域 SQL 和事务操作。
- runtime 根包管理子进程、会话与轮次，不依赖 app 或业务服务。
- protocol 保持底层合同；领域内部事实放在领域包，不向 protocol 堆入私有实现。
- 外部 Relay 合同独立于客户端实现；存储和同步服务消费同一合同。

## 包职责

- `cmd/`：可执行入口——nexus-server（服务 + 自动迁移）、nexusctl 资源控制 CLI、nexuscfg 配置 CLI、Linux runtime launcher。macOS 桌面在迁移前持有 `app/sidecar.lock` 内核实例锁，直至服务关闭。
- `desktop/`：macOS AppKit/WKWebView、Windows WPF/WebView2 宿主与 browser-extension。
  - 负责窗口 chrome、bridge、状态根整体迁移与重启、本机 workspace 文件打开与 macOS 关联应用发现。
  - sidecar 生命周期按 boot-bound audit identity 精确终止；旧格式存活或未知记录保留，并拒绝并发启动。
  - Windows 用独立原生标题/菜单栏承载全部拖窗与系统命令；WebView 始终保持客户区，通过公开可见性生命周期随主窗口挂起或恢复；Theme/Dialog 将 Nexus token 投影到原生菜单与反馈窗。
  - Chromium 扩展的合同见 [Browser 能力规范](./browser-spec.md)。
- `protocol/`：跨 HTTP/WS/前端/运行时的协议真相源与 TS codegen 输入。包括会话、房间、Goal/Execution Graph 与命名工作图模型，NodeRun 历史、可恢复结构化产物、显式 partial/total、控制回连事实，Room creator/lead 身份，以及事件和枚举。
- `service/objectivealignment/`：Goal completion 与 Execution loop guard 共用的无状态目标对齐审计契约。
- `automation/`：定时任务调度域——任务级 capability grant、持久审批、主会话事件派发、run 阻塞与安全恢复。
- `service/memorymaintenance/`：Nexus 唤醒 nxs 后台记忆维护的宿主协调器；通过共享 runtime Manager 启动一次性 AutoDream，并统一监督、取消与回收。
- `app/`：HTTP 与 CLI 共用的显式服务装配和资源所有权。
  - 退出时先停止 runtime 准入并等待终态落盘，再关闭数据库。
  - `server` 只负责 HTTP/WS 与后台启停。
  - macOS 桌面入口将迁移前实例锁交给 App，以随包 helper 和 `app/processes` 完成两阶段恢复后才开放任务。
  - `goal` / `execution` / `workgraph` / `runtime` 承载宿主适配；`runtimecheck` 负责安装包内核配套检查。
  - Goal/Execution 跨域业务归 `service/goalexecution`，身份失效消费策略归 `service/auth`。
- `config/ storage/ infra/ migration/ version/`：装配、迁移与基础。
  - `infra/desktopinstance`：macOS sidecar 的状态根独占锁；不做 PID 推断，旧版未持锁宿主仍须单独核验。
  - `infra/duework`：后台 durable work 的合并唤醒、精确 deadline timer 与低频审计。
  - `infra/runtimeidentity`：Linux UID/GID、ACL、Landlock launcher。
  - `infra/confinedfs`：宿主目录 fd 边界。
  - `infra/runtimebootstrap`：校验 macOS 随包监督 helper 的固定 Bridge 构建身份与签名后摘要。
  - `infra/textutil`：只依赖标准库的字符串取值原语叶子包（`FirstNonEmpty`/`PointerValue`/`AnyString`）；各包不得私有复制同构 helper。

## MCP 能力域

`mcp/`、`connectors/`、`workspace/` 是能力域。

- `mcp` 根包持有 physical-round 共用可信上下文与 command receipt。
- `mcp/command` 持有 Goal/Execution/Automation/Subagent 的 `nexus.command` 工具协议和操作适配。
- 宿主自有、与 Nexus 系统功能相关的进程内工具统一挂在单一 `nexus` MCP server 下；各业务包只构建工具定义与固定上下文。
- 模型控制复用内置 Skill，业务输入直接进入宿主，不落临时 JSON。
- `mcp/communication` 以 `list_targets` 与上下文感知的 `send_message` 统一 DM、跨会话和当前 Room 通讯；不设独立 Room MCP 工具包。
  - IM 场景用宿主数据库保存投递来源，并把人类反馈交回原 Session。
  - 好友私聊保持独立语义。
- `mcp/browser` 通过单个 `browser` 工具提供完整浏览器操作。
- `mcp/visualize` 只暴露 `show_widget`；生成规范由 `skills/visualize` 承载。
- `mcp/artifact` 通过 `deliver_files` 登记 Skill/脚本等最终文件交付，由 workspace 服务校验后随产出 Agent 的精确轮次消息持久化。
- 第三方、用户自定义和 Connector 动态 MCP（包括独立的 `nexus_feishu_docx`）保持各自 server 身份、授权与生命周期；支持原生 MCP 的 Provider 直接挂载自身 server；不提供通用 REST 路由。
- owner 资源管理复用 nexus-manager / nexusctl；配置管理复用全 Agent 内置 nexus-configuration Skill 与 round-scoped nexuscfg；不挂载 manager 或 configuration MCP。

## DM 与 Room 共用宿主

- `service/runtimehost.Host` 持有 DM 与 Room realtime 共用的依赖（Provider、runtime admission、队列信任、用量、额度、执行上下文、子智能体准入、日志、MCP 工厂与 nexuscfg 环境、Slash 展开）、注入方法与共用阶段；成员见 [`runtimehost/doc.go`](../../internal/service/runtimehost/doc.go)。
- 两个 Service 都嵌入 Host，共用阶段只实现一次。
- runtimehost 不得依赖 dm 或 room/realtime；Room 的多成员 slot、公私域输出与宿主锁只在 realtime 中实现。

## 入口规范化

- 字符串在入口清洗一次：handler/WebSocket 解析、MCP parser、存储扫描与 `authctx` 身份读取负责 `strings.TrimSpace`，下游 service 信任已清洗值。
- `tools/trimcheck`（`make check-normalization`）用类型信息证明哪些 `TrimSpace` 冗余：
  - 常量；
  - 返回值全部已裁剪的本模块函数；
  - 所有赋值均已裁剪的局部变量；
  - 本模块声明且所有写入均已裁剪的字段（带 tag、被取地址、经接口参数反射写入或被类型转换覆盖的字段除外）。
- 分析分别在 linux/darwin/windows 下进行，只报告三者都成立的位置。

## 交付项与验收

| 项目 | 实现归属 | 验收 |
| --- | --- | --- |
| Team 同步 | `internal/relay` 合同，`service/relay` 客户端，`service/team` 同步流程 | handler 契约、服务失败路径、仓储连续游标与幂等测试 |
| Execution 命令观察 | orchestration 操作事实，runtimehook 回执转换 | 回执转换与 Runtime Graph 回归 |
| Session 删除 | deletion 协调，automation/orchestration 仓储事务内清理 | 中途失败回滚与 owner 隔离测试 |
| 历史投影 | message 负责结果归属与合并，storage/workspace 负责存取与查询 | 混合 Agent 结果、重放与历史分页回归 |
| 架构门禁 | Go 导入检查与现有 Go 检查流程 | 实际依赖图与禁止边界的负例 |

各项必须保持的行为：

- Team 同步
  - 远端消息已提交后，本地投影失败仍返回成功。
  - 目录、建群、快照和增量投影失败不能确认成功。
  - 身份与令牌只来自可信入口。
  - 不新增隐式重发或后台消费者；投影恢复继续使用既有 Snapshot/Difference 与原游标，不能把结果未知当作未提交重试。
- Execution 命令观察：请求身份、责任归属、拒绝状态、重放幂等与节点关联不变。
- Session 删除
  - 删除协调器是既有跨表事务的唯一提交者；领域仓储接受同一个 `sql.Tx`，不自行提交，也不把现有事务拆成多个服务调用。
  - Goal/Task 原有准备顺序不变，前置清理仍是原有可重试流程；不声称跨文件、runtime 与 SQL 已具有全局原子性。
  - 路由与执行数据同事务提交；owner 隔离，历史审计保留。
- 历史投影
  - 保留公共分页入口、物理 Agent round 配对、旧记录兼容与未匹配结果合成；不迁移历史格式或缓存模型。
  - 复用已有 `readHistoryPageWithIndex`，不增加另一套查询门面。
  - 文件安全、分页索引、源解析和删除栅栏同包维护，不为目录划分暴露内部模型。
- 架构门禁：检查生产导入，允许集成测试装配；无动态例外清单。

## 测试组织

- `service/room/realtime` 测试按 package 与行为聚合：内部状态、Goal、协作测试分别归组；外部交付、生命周期和共享夹具集中管理；queue、guidance、session、directed message 等大场景保持独立。
- `service/configuration` 测试按身份授权、输入与风险、脱敏、审批、审计及业务集成归组；共享装配与审批辅助集中在现有集成测试文件；重复成功路径复用完整场景；越权、CAS、凭据和删除恢复边界独立保留。

## 验证记录（2026-09-09）

- 目标包与调用方测试及其 `go vet` 通过：app、CLI、Team handler/service、Relay、deletion、orchestration/runtimehook、DM、Room、session、automation、相关仓储、message 与 nexus-server。
- `go test -race` 覆盖：Team 投影失败、Session 删除事务回滚和 owner 隔离、历史结果归属、运行回执转换及共享 Observer。
- `go test -run '^$' ./internal/... ./cmd/...` 验证全部内部包与命令入口可编译，不代表运行全部测试。
- 架构门禁及其允许/禁止依赖测试通过；门禁按当前 Go 构建环境检查生产导入。
- Go 格式、检查脚本语法和 `git diff --check` 通过。
