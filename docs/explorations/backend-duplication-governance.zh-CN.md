# Go 后端重复与防御性代码治理审计

> 状态：non-normative，2026-10-08 一次性审计记录。当前约束以 AGENTS.md、`docs/specs/internal-boundaries.md` 与 `scripts/check-architecture` 为准；本文记录度量方法、已落地治理与剩余热点，不定义产品行为。

## 1. 审计范围与方法

- 范围：`internal/` 与 `cmd/` 的 Go 生产代码（基线 310,659 行，测试 186,482 行）；`web/src` 只做复制粘贴度量。
- 死代码：`deadcode -test ./...` 与 `deadcode ./...` 分别在 `GOOS=linux/darwin/windows` 下运行，只处理三平台交集，避免误删平台专用实现。
- 重复：`dupl -t 80` 找结构克隆；自写 AST 工具按“签名 + 函数体”哈希找逐字副本，再按函数形状找改名副本。
- 防御性代码：统计接收者/依赖 nil 守卫、`strings.TrimSpace` 密度与同函数内重复守卫，并逐条人工确认。

## 2. 发现

| 类别 | 基线 | 说明 |
| --- | --- | --- |
| 二进制与测试都不可达的函数 | 103 | 多为被新实现取代的旧路径、只剩一个调用者被删后的孤儿 |
| 仅测试调用的生产函数 | 99 | `X`/`XWithY`/`XAt` 薄包装、被替代的参考实现（如旧分页作为差分测试 oracle） |
| 逐字重复的私有 helper | 75 组 / 1,083 行 | `firstNonEmpty` 一个形状就有 28 份副本，另有指针取值、any 转字符串各 6–8 份 |
| 枚举判定散落各层 | 22 份 | `ExecutionStatus` 终态/当前态、`WorkItemKind`/`GoalActivation*` 合法性在 service、storage、MCP 各写一遍 |
| DM ↔ Room realtime 平行实现 | 14 个同名文件 | 附件解析、上下文块、Goal 哨兵错误判断等成对复制 |
| `strings.TrimSpace` | 9,166 次 | 同一值在 ingress、service、storage 各层反复裁剪 |
| 接收者/依赖 nil 守卫 | 407 处 | 集中在 room/realtime（76）、dm（51）、runtime（40）、handler/websocket（30） |

复制已经造成实际漂移：DM 附件解析在最终 owner 为空时拒绝，Room 副本缺少该检查。前端 `web/src` 在 15 行阈值下重复率仅 0.08%，问题集中在 Go 后端。

## 3. 已落地治理

1. **删除死代码**：删除三平台交集上不可达的 102 个函数及其遗留的 9 个类型/常量。
2. **测试入口移出生产包**：63 个仅同包测试使用的函数移入 `_test.go`，遵循“测试便利入口优先留在 `_test.go`”；跨包测试共享的装配入口（`handlertest`、`app.New*Service`）保留。
3. **共享字符串原语**：新增只依赖标准库的叶子包 `internal/infra/textutil`（`FirstNonEmpty`、`PointerValue`、`AnyString`），替换 50 份私有副本；架构门禁允许 runtime 根包导入该叶子包，并禁止它反向依赖任何 internal 包。`protocol` 不能导入 internal，保留自身一份。
4. **枚举知识归位 protocol**：`ExecutionStatus.Current/Terminal`、`GoalActivationOrigin/Reason.Valid`、`GoalActivationReason.PromotionOrigin`、`WorkItemKind.Valid`、绑定 `Clone`、`GoalUsage.IsZero` 取代 22 份分层副本。
5. **DM/Room 去重**：
   - `conversation.OpenAgentWorkspaceAttachment` 统一 Agent workspace 附件授权，Room 补齐空 owner 拒绝。
   - `runtime.GoalContextualInputs/ExecutionContextualInputs` 统一隐藏上下文块构造。
   - `goal.IsAbsent/IsInactive` 取代手写的哨兵错误集合。
   - `Lease.Resources()` 本身 nil 安全，删除三份 `sandboxResourcesFromLease` 包装。
   - `sdktool.ErrorResult/JSONResult/StructuredJSONResult` 取代六个 MCP 包的结果构造副本。
6. **冗余防御**：删除包在已裁剪 helper 外层的 19 处 `strings.TrimSpace`。
7. **防回归门禁**：`scripts/check-architecture` 按函数形状拒绝 textutil 原语的私有副本（改名也会命中），已接入 `make check-go` 增量门禁；它在接入当次又找出 8 份改名副本并完成替换。

结果：生产 Go 代码净减约 2,400 行；逐字重复 helper 从 1,083 行降到约 660 行；`deadcode -test` 剩余 7 项，均为单平台专用实现。

## 3.1 测试瘦身

- **方法**：3,081 个 Linux 可运行的顶层 Go 测试逐个单独运行，记录跨包语句覆盖（`-coverpkg=internal/...`）；对 49,116 个被覆盖块做贪心集合覆盖，选出保持全部覆盖所需的最小测试集。前端 406 个标准 Vitest 文件按文件单独采集 V8 语句与分支覆盖，做同样的选择。
- **始终保留**：被 `scripts/`、`makefile`、非 evidence 文档点名的必测用例；名字表明并发/竞态的用例（覆盖率无法表达交错）；Linux 上跳过或基线失败的用例；darwin/windows 专用测试；被父测试以 `-test.run` 子进程重入的入口（`TestRevisionKeyChildProcess`、`TestSandboxCrashHelper`）；校验仓库数据不变量的 `TestMigrationVersionsAreUniqueAcrossDialects`。
- **结果**：删除 1,025 个无独有覆盖的 Go 测试及其孤立辅助代码和 32 个空测试文件，测试代码从 187,751 行降到约 144,400 行；删除 41 个前端测试文件（145 个用例，约 2,400 行）。全量运行中仅少量依赖断连/取消时序的错误分支在单次运行里未命中，原覆盖它们的测试均保留。
- **局限**：覆盖率只能证明“执行过”，不能证明“断言过”。同一路径上断言不同错误码或边界值的测试可能被一并删除；后续补测试应优先针对未覆盖分支，而不是重复已覆盖路径。
- `internal/runtime/permission` 的 `TestMemberSessionPermissionDoesNotResolveOtherRoomMember` 在删除前的基线上即约 30% 概率失败：`BindSession` 异步重放已挂起请求，晚于首个请求执行时同一请求会被实时下发与重放各送达一次，测试误把重复事件当成第二个成员的请求。重放本身是至少一次语义，已改为按未见过的 `request_id` 取事件修复。

## 3.2 第二轮架构治理（AGENTS 瘦身、依赖装配、入口规范化、DM/Room 共用宿主）

- **AGENTS.md 瘦身**：60 段产品合同原文迁入对应 `docs/specs/`，新增 `online-team-spec.md`；AGENTS.md 从 50 KB 降到约 9 KB，只保留构建门禁、文档分层、依赖方向、工程原则和合同索引。
- **依赖装配 fail fast**：DM 与 Room realtime 方法内 134 处接收者与构造器自有存储的 nil 守卫删除；`RequireWiring` 让 app 装配缺依赖时在启动前失败；测试夹具通过 `withConstructorDefaults` 补齐构造器不变量。`rooms`/`agents` 守卫保留，测试用它表达“宿主没有 Room 仓库”。
- **入口规范化**：`tools/trimcheck` 用类型信息证明 494 处 `strings.TrimSpace` 冗余并删除（另有 48 处自赋值），三平台取交集保证可靠；`make check-normalization` 防回潮。剩余约 8,500 处多数作用于参数或带 tag/反射写入的字段，需要按领域在入口显式清洗后才能继续删除。
- **DM/Room 共用宿主**：`service/runtimehost.Host` 承载共用依赖与注入方法；两个 Service 嵌入它。
- **DM 是 Room 的一种（第三轮）**：每轮 Goal 状态统一为 `runtimehost.GoalRoundState`（一把 `Mu`），DM `roundRunner` 与 Room slot 都嵌入它。以下阶段各只实现一次：
  - Goal 续跑准备、派发与启动前失败回写；`/goal` 命令建 Goal（Room 额外校验唯一 lead）。
  - 用量增量、用量上限标记、from-now scope 绑定、子任务 round 认领，以及九处手写退避循环收敛成的 `RetryGoalUsage`。
  - 子任务待落库观察的合并/清除、工具推进标记、命令回执游标、result/assistant token 用量去重、完成收据构造。
  - SDK 消息映射适配、流关闭/空闲诊断字段（`exec.RoundStreamFailureLogFields`）、SDK session 可持久化判定、owner 后台任务。
- **语义对齐**（按 DM 规则统一）：
  - Room terminal 用量在只有结算边界时也会 flush，并把边界写入快照。
  - Room 子任务观察缺少 `ObservedAt` 时补当前时间。
  - DM 诊断日志补 `stream_read_error`，Room 补 `stream_last_summary`。
  - Room 不再在缺 runtime manager 时同步执行后台任务（runtime 是必需装配）。
- **结果**：相似度 ≥0.7 的 DM/Room 孪生函数从 30 对降到 21 对；生产 Go 代码在这一轮净减约 400 行。
- **剩余差异**：多为宿主身份与落点不同的薄适配（`RequireWiring`、`writeUsage`、`NewService`、请求校验、附件解析中 Room 会话资产分支、上下文占用持久化），属于 Room 的 slot/公私域策略，不再合并。

## 3.3 文档瘦身

- 修复失效链接：`docs/guides/workgrpah/`（拼写错误目录）下的 WorkGraph 指南移回 `docs/guides/`，删除同内容的 zip（含 `__MACOSX`）；指向不存在文件的链接改指或删除。
- `docs/README.md` 重写为单一索引，此前有若干规范、指南与测试记录无入口。
- 规范与指南：AGENTS.md 迁入的“宿主实现约束”块按主题并入正文或删除重复；超过 400 字节的段落和表格单元拆成列表；历史叙述改为当前规则加证据链接；跨文档重复改为链接到唯一归属文档。`docs/specs`、`docs/guides` 与架构蓝图合计约 760 KB → 655 KB。
- 按代码更正：`execution-graph-spec` 的 WorkGraph 保存不再调度隐藏模型轮次（`ConfirmSave` 直接保存）。
- 瘦身中发现、需产品确认的规范内部矛盾见本文附录 A；没有擅自改语义。

## 4. 未处理热点与建议（按收益排序）

1. **ingress 一次清洗，下游信任**。`protocol.ExecutionWorkBinding.Normalized` 的注释已经写明“清洗只发生在 ingress，下游一律信任已清洗的值”，但大部分领域没有执行这一原则。建议每个领域只在 handler/MCP parser/仓储扫描处裁剪，service 内部删除重复 `TrimSpace`；room/realtime（845 次）与 automation（839 次）收益最大。需逐领域推进并补齐 ingress 测试，不适合机械批量替换。
2. **必需依赖在构造期校验**。DM/Room 大量 `if s == nil || s.goals == nil` 把“未装配”伪装成“功能关闭”。建议构造函数对必需依赖 fail fast，真正可选的依赖改为显式 no-op 实现，再删除方法内守卫。需先确认生产装配与测试夹具，属于跨包改动。
3. **DM 与 Room realtime 的执行骨架**：已在 3.2 落地。剩余可做的是把 DM 改为单成员 Room slot 直接复用 Room 的 round 注册与派发锁，这会改变 DM 的会话键与历史落点，需要迁移方案。
4. **SQL 空值 helper**。`nullString`/`nullTimePointer` 等在各仓储包重复约 20 份。`internal/storage/time_value.go` 明确选择“普通 typed row scanner 保留在领域内”，本次尊重该决定；如需统一，应先修改该约定，再收敛到 `internal/storage`。
5. **大文件**。`orchestration/context.go`（1,888 行）、`room/realtime/goal_runtime.go`（1,809 行）、`room/realtime/chat.go`（1,607 行）可在第 3 项落地后按业务阶段拆分。

## 5. 复现命令

```sh
go install golang.org/x/tools/cmd/deadcode@latest github.com/mibk/dupl@latest
GOOS=linux deadcode -test ./...   # 另以 darwin/windows 运行并取交集
dupl -t 80 $(find internal cmd -name '*.go' ! -name '*_test.go')
make check-architecture           # 依赖方向 + textutil 私有副本
```

已知基线问题：`internal/runtime/clientopts` 中 5 个用例（桌面沙箱不支持 Linux、视觉模型校验）与 `internal/storage/sandbox` 中 3 个进程迁移用例在本次改动前的基线提交上即于 Linux 失败，与本次治理无关；其余 `go vet ./...` 与 `go test ./...` 全部通过。

## 附录 A：规范矛盾的处理（按实现修正）

瘦身中发现的规范内部矛盾已逐条对照代码修正，只保留与实现一致的表述：

| 文档 | 结论（依据） |
| --- | --- |
| desktop-sandbox-spec | macOS App 默认装配 supervisor、写 launch record、启动前执行进程与生命周期恢复，并对受监督启动的策略回执对账到 `reconciled`；非 darwin 构建均不装配（`internal/app/desktop_sandbox_darwin.go`）。scratch 标记清理仍只有显式 HTTP sweep。 |
| message-processing-spec | Room 主 Feed 活动行不读 ToolUseSummary：运行中工具为中性 shimmer，否则为静态主色文案加 spinner；Room 历史索引对已校验的 ledger 追加做尾部增量刷新，DM 全量重建；DM 与 Thread 工具组首次都折叠。 |
| workspace-isolation-spec | Unix 上 `/tmp` 是 runtime 读写根（`policy_linux.go`）；身份推导按 `authctx.Principal` 与 `appfs` 根目录，不存在 `UserScope` 类型。 |
| automation-permission-pipeline-spec | `delivery_session_key` 只由 `nexus.command` create/update 接受，HTTP 用结构化 `delivery`。 |
| echo-spec | 状态图补 `committing` 与 `evaluating→scheduled`（`internal/echo/model.go`）。 |
| skill-spec / 对话配置规范 | `credentials_encrypted` 存的是明文 Bearer token。 |
| execution-orchestration-spec | Room 观察者视图只有 Goal、目标、完成标准、图摘要与节点状态，不含 Assignment/Review/Submission 证据。 |
| room-spec / room-collaboration-spec | 浏览器群聊无目标输入：显式目标 → @ → 开启自动回复的群主 → 不启动 Agent（`chat.go`）；Goal 证据只在归属 revision 等于当前 revision 时写入，首条非 Lead 记录保留。 |
| online-team-spec / 蓝图 | 身份失效事件规则迁入 online-team-spec；蓝图失效代码路径已更正。 |
| frontend / capability 规范 | Room 导航偏好规则移出测试章节；能力页为 技能、工作图、连接器、定时任务、频道、配对。 |
| execution-graph-spec | WorkGraph 确认保存由宿主直接写入，不调度模型轮次。 |
| workgraph-design-principles | 计数与 §1 列表一致，摘要链条与 §4.2 一致，删除未引用的文献。 |
