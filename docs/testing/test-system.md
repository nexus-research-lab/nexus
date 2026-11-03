# 测试体系与运行入口

测试断言可观察行为：输入输出、用户动作、请求参数、版本、持久化结果和恢复边界。
静态工程约束使用 ESLint / TypeScript AST / CSS 工具链；不通过匹配业务源码、
注释措辞或 Tailwind class 证明行为。视觉陈列使用 Gallery，不要求组件登记覆盖。

## 日常门禁

- `make check-go`：仓库规定的增量 Go 门禁；不要默认全量运行。
- `pnpm --dir web run check`：ESLint、类型检查、静态工具测试、全部共置 Vitest 和生产构建。
- `go generate ./internal/protocol && git diff --exit-code -- web/src/types/generated/protocol.ts`：协议生成漂移，后端 CI 强制执行。
- `pnpm --dir web run test:browser`：保留的浏览器流程测试，手动运行。

Vitest 统一加载业务 TS 模块；迁移来的 MJS 用例直接 import，并声明 Node 环境。
组件继续使用 jsdom / Testing Library。布局算法、滚动锁、焦点恢复和用户设置属于
行为；装饰性 class、颜色、字号和阴影不属于测试断言。

## 数据库与升级

`go test ./internal/storage -run 'Test(Database|WorkGraph.*)Integration' -count=1 -timeout=3m` 总是执行 SQLite；
设置 `NEXUS_TEST_POSTGRES_URL` 后，同一套迁移、Room owner/CAS、Goal JSON/时间/CAS/事件失败回滚、
Execution 幂等事件用例也在 PostgreSQL 执行。每次创建独立 schema，并在清理阶段删除。
变量缺失时仅 PostgreSQL 子测试跳过；配置存在但连接或迁移失败必须报错。
后端 CI 使用 PostgreSQL 17 service，显式设置变量，不允许该分支跳过。

WorkGraph 集成用例还覆盖批量派发在后续审计 INSERT 失败时的整批回滚、同请求并发重放、
不同请求竞争同一 Execution revision、Draft revision/编辑绑定竞争、精确撤销，以及历史摘要和
Runtime Graph 详情窗口。并发调用使用两个独立数据库连接池，不靠单个 SQLite 连接池串行化。
升级用例先迁移到 158 并写入合成 WorkGraph，再升级到当前版本，检查冻结回填范围、失败回滚、
重试幂等和不覆盖较新的运行事实。用例不运行真实 provider，也不读取安装版数据库。

使用已有专用测试 PostgreSQL 时显式设置 `NEXUS_TEST_POSTGRES_URL`（URL 格式）；
账号须能创建/删除 schema。也可使用 `initdb` + `pg_ctl` 创建仅监听临时 Unix socket 的本地实例，
用 `postgresql:///postgres?host=/absolute/temporary/socket&port=55483` 连接，结束后停止该实例。
不要将测试连接指向安装版或其他业务数据库。运行日志必须确认 PostgreSQL 子测试确实执行，
未配置变量时的整体 PASS 不等于 PostgreSQL 验收通过。

现有 SQLite 专项测试继续保留。双驱动集成套件覆盖共用仓储边界，不代表所有
SQLite 专项用例都适用于 PostgreSQL；例如 SQLite PRAGMA 与历史编号修复有自己的测试。

`go test ./cmd/nexus-server -run TestPreviousReleaseDatabaseUpgrade -count=1`
使用冻结的 v0.2.5 合成数据快照，经真实启动迁移函数升级，校验数据与重复启动。
来源和重建方法见 `cmd/nexus-server/testdata/README.md`。不读取已安装 App 的数据。

### WorkGraph 长历史读取基准

`TestWorkGraphReadProjectionIntegration` 在两种数据库上对比完整读取和 SQL 窗口读取的
最终主图、详情窗口、原始连线身份、产物、可见 total 与 truncated。夹具包含真实 managed
Plan 和 coordination lane，避免运行记录全部被可见性过滤后产生空图一致的假阳性。
原始位置序号允许因裁剪出现间隔，但保留节点的相对顺序和其余字段必须一致。
测试从已有运行记录的 161 数据库升级，覆盖索引创建、owner/session/graph 边界、来源被裁剪的
普通连线及原始目标在读取范围外的控制边；重试和回环边不按普通详情目标裁剪。

```sh
go test ./internal/storage -run '^TestWorkGraphReadProjectionIntegration$' -count=1
go test ./internal/storage -run '^$' -bench '^BenchmarkWorkGraphRuntimeRead$' -benchtime=3x -count=3 -timeout=15m
```

PostgreSQL 仍需上文的显式测试连接。首次调查可用 `-benchtime=1x -count=1`，再用 benchmark
路径筛选数据库、`detail`/`mixed`、`1000`/`10000`/`50000` 和 `full`/`window`；不能把单次结果当稳定分布。
每 100 条合成工具记录附带一个 Agent round，包含失败/最后成功、retry/loop_back、详情父子关系
和后到产物；mixed 另增加可见动作和失败密度。工具结果正文固定，建库、迁移、准备语句批量写入、
ANALYZE、一次预热、完整/窗口投影一致性检查及 JSON 大小计算均不计入 `ns/op`。

测量的是数据库及 OS 缓存已预热的串行仓储读取，包含取消上下文、SQL、扫描/解析、排序和产物关联。
`B/op`/`allocs/op` 是 Go 分配总量，不是峰值内存，也不包含数据库进程内存；`json-B/op` 是内部仓储
结果序列化大小，不是 HTTP 响应或模型 token。`nodes/op`/`edges/op` 是返回事实数。
单次读取超过 45 秒必须失败并记录超时，不能把取消时间当完成耗时。完整/窗口都运行时，每个规模
都对比最终投影；仅筛选 window 时只测读取，投影验证使用独立集成测试。
环境、样本数、原始输出及对照结论保留在本地或 CI artifact；不据此宣称冷启动、浏览器绘制、
真实模型 token 或端到端耗时已改善。

## 命令调用诊断

在隔离开发实例上设置 `LOG_FORMAT=json`，日志级别为 `info` 或 `debug`，从配置的 `LOG_PATH`
收集 JSON 行。`nexus.command.call` 的 start/finish 通过随机 `call_id` 配对，request hash
在同 owner/session/agent/domain 内可跨 round 关联；round hash 区分物理轮次。上述 hash 只用于诊断。
命令入口诊断不包含输入、产物、错误正文或原始身份；同一应用日志中的其他事件不属于该承诺。

```sh
go run ./scripts/workgraph-command-report \
  --label 'local-synthetic-batch' \
  --environment 'synthetic commands; macOS arm64; no provider' \
  /absolute/path/commands.jsonl > /absolute/path/report.json
```

支持传多个未压缩日志文件。汇总按 domain/action/operation/status/outcome/reason 分组，报告样本数、
重复请求观察和宿主时长的均值、p50/p95（nearest-rank）；重复日志行不会重复计数。
缺少 finish、记录相互冲突、缺少 outcome 或时长时保留 unknown/null；同时报告格式错误、
未知 schema、缺少 start 和缺失字段。两个事件都丢失的调用无法计数，日志不是 durable receipt。
目前精确操作标签来自 Goal/Execution registry；其他领域及到达 registry 前被拒绝的请求可能显示 unknown。

`host_duration_ms` 包括入口处理、start 日志、领域调用与结果序列化，不含模型生成/网络或 finish 日志写入；
它不能替代端到端时长。单工具 token 固定留空，需另外保存真实 provider/round usage。
重复请求观察不能用来推断业务副作用被重放。完整合同见 [权限与 MCP 规范](../specs/permission-runtime-spec.md#61-命令诊断与业务回执)。

采集器和离线入口检查：

```sh
go test ./internal/mcp/command ./scripts/workgraph-command-report -count=1
go test ./internal/mcp/command -run '^$' -bench '^BenchmarkCommandDiagnostics$' -benchmem -benchtime=200ms -count=3
```

后一个基准只比较内存内命令包装层与写入 `io.Discard` 的 JSON 诊断编码，不包含数据库、文件日志
I/O、真实模型或 token；它用于检查诊断自身成本，不能证明 WorkGraph 端到端收益。

真实优化对照需要固定源码版本/工作树快照、模型/provider、预算和输入数据，并单独保存每轮原始 usage：

| 场景 | 对照 | 相同的正确性要求 |
| --- | --- | --- |
| 8 个独立 Ready Work Items | 单项 `assign_work` 与 `assign_work_batch` | 相同显式 owner、Attempt、outbox 和最终责任；不得把未来依赖项提前派发 |
| 一次交付及审核 | 重复读取 contract/context 与复用已提供的准确 schema/context | 同一 Plan/binding；动态权限仍在调用时复核 |
| 字段错误后纠正 | 无效输入 → 纠正 → 有效请求 | 无效请求没有写入；新结果需 domain receipt 证明 |

每份报告注明样本量、失败/unknown、日志缺失、预热条件、数据库和端到端墙钟时间。
合成结果只验采集/统计；真实模型没有运行时，不填 token 节省比例。

## 真实后端冒烟

先安装 Chromium 并构建 Web，再运行：

```sh
pnpm --dir web exec playwright install chromium
pnpm --dir web run build
NEXUS_BROWSER_SMOKE=1 NEXUS_SMOKE_NXS_BINARY=/absolute/path/to/nxs \
  go test ./internal/app/server -run '^TestBrowserSmoke$' -count=1 -timeout=3m
```

测试使用临时状态根、数据库、真实 HTTP/WS 路由和真实 nxs。仅模型 Provider 使用本地
固定流式响应；浏览器不拦截 Nexus API。覆盖本地桌面凭据拒绝/认证、发送、回复与刷新后的历史。
它不证明在线 Control 密码登录、多人 Relay 或原生 WebView 行为，也不因此删除这些领域
仍有独立价值的 mock 流程用例。只有被真实冒烟实际覆盖的冗余流程才应删除。

`.github/workflows/backend-browser-smoke.yml` 在 nightly 和手动触发时运行，不进入 PR 门禁。
运行时下载复用已有 `NEXUS_NXS_RUNTIME_RELEASE_TOKEN`；失败保留 Playwright trace 和截图。

## 平台边界

后端 CI 在 macOS 与 Windows 执行 confinedfs、desktopinstance、runtime、clientopts 和 auth
目标包。真实沙箱、进程恢复和原生宿主验收仍使用既有 desktop sandbox 工作流；
平台包测试通过不能代替原生发布验收。

### WorkGraph 恢复入口与结果钩子

- `go test ./internal/service/orchestration ./internal/storage/workspace ./internal/handler/execution ./internal/runtime ./internal/service/orchestration/runtimehook` 覆盖精确恢复授权、稳定请求 ID、丢失受理回执、旧目标失效、单项越过旧暂停与后续停止、Stop 严格解析与路由能力边界。
- `NEXUS_TEST_POSTGRES_URL=... go test -race ./internal/storage -run 'TestWorkGraphRuntimeResultIntegration|TestWorkGraphUpgradeIntegration' -count=1 -v` 在临时 SQLite 与独立 PostgreSQL schema 上验证结果先持久接收、领域提交前/后失败、回执落盘失败、重建服务后的对账、交付/审核工具竞态、冲突结论及终态拒绝。PostgreSQL 未配置时明确 skip，不计入双数据库验收；禁止使用安装版数据库。
- 前端：`pnpm test src/features/conversation/shared/execution src/shared/ui/markdown/core/markdown-comments.test.tsx src/shared/ui/markdown/streaming`，另跑 `pnpm run check`。浏览器检查当前/历史/stale 的入口、普通/放大图共享状态、unknown 只读核对及注释流式隐藏。
- 可选真实 runtime/provider：显式设置 `NEXUS_TEST_RESULT_RUNTIME_BINARY`（nxs 绝对路径）、`NEXUS_TEST_RESULT_MODEL` 与测试 Provider 的 `ANTHROPIC_BASE_URL` / 凭据环境，再执行 `go test ./internal/storage -run '^TestWorkGraphResultRuntimeAcceptance$' -count=1 -v -timeout 25m`。`NEXUS_TEST_RESULT_SAMPLES` 为 1–5，缺省 1；默认门禁跳过，不读取个人应用配置或自动调用收费模型。每轮关闭 thinking 并使用临时 CWD/config/数据库，显式工具路径复用真实 nexus.command parser 与领域服务，钩子路径复用生产路由/声明解析/领域回执。输出仅含模型、调用数、终态 usage、耗时与结果校验，不输出凭据或完整上下文。
- 上述 provider 实验是相同小型合成交付/审核的有界对照，不等同真实应用会话、长任务、安装版升级或 macOS/Windows 原生宿主恢复验收。缓存、Provider 排队与模型行为会影响 token/耗时；未经实测不能只凭减少工具数宣称性能收益。

### WorkGraph 声明派发、检查点、模板与应用恢复

- `NEXUS_TEST_POSTGRES_URL=... go test -race ./internal/storage -run 'TestWorkGraph(DispatchPolicy|Checkpoint|Template|Upgrade|RuntimeResult|ReadProjection)Integration$' -count=1 -timeout=5m` 验证依赖验收前不派发、并发唯一责任、同请求意图、暂停/恢复、进展与验收分离、接管后证据保留、类型参数及精确模板版本统计。使用独立 PostgreSQL schema；SQLite 同时执行。158/161 升级夹具用冻结的旧结构 SQL 写入旧数据，不能用当前 repository 伪装旧版。
- `NEXUS_WORKGRAPH_APP_ACCEPTANCE=1 NEXUS_SMOKE_NXS_BINARY=/absolute/nxs go test ./internal/app/server -run 'TestWorkGraph(ApplicationAcceptance|ProcessRestartAcceptance)$' -count=1 -v -timeout=3m` 使用完整 AppServices、真实 runtime Manager/nxs 和本地脚本化 Provider；状态、数据库、workspace 和测试 Provider 都是临时资源。DM/Room 各执行两步依赖工作和独立审核，通过实际队列和结果 hook 检查唯一 SQL 写入及正常清理。
- 重启用例启动独立宿主子进程，在真实 Provider 请求在途时杀死它，并分别覆盖用户停止后杀进程；新进程重开同一临时状态，核对检查点、unknown 效果、停止暂停和重复恢复请求的同一 durable 回执。显式恢复必须真的请求 Provider 并结束，不能仅凭入队或瞬时 idle 判成功；恢复检查不提交、不验收、不重放模拟外部操作。此用例不等同原生桌面壳退出、Windows 或安装版数据库验收。
- 多节点真实模型对照：提供上一节的 runtime/model/provider 环境，并设置 `NEXUS_TEST_PIPELINE_RUNTIME=1`，执行 `go test ./internal/storage -run '^TestWorkGraphPipelineRuntimeAcceptance$' -count=1 -v -timeout=18m`。固定三节点硬依赖算术链，每个节点各有交付/审核轮，运行三组 tool/hook 配对、交替先后顺序。两条路径统一配置 `MaxTurns=8`、每轮 3 分钟；SDK 终态报告的 `num_turns` 原样保留，不将配置值当作实测轮次上限。结果须有三份 Submission/Acceptance 和最终 Completion。Queue consumer 在该模型实验中是受控端口，应用队列证据由上一项提供。
- 时间口径为 Query→Result，排除进程启动；token 是实际终态 usage（含缓存），不换算金额。hook 回退显式工具、重复轮次和 runtime 错误必须保留；超时记录已用时间且标记 usage 缺失，不能填零或作为成功样本。分开报告完整配对、失败/不完整运行与预算调整，不反复筛选通过样本。日志、截图及测量报告放在本地或 CI artifact，不提交真实凭据或验收产物。
- 前端行为：`pnpm test src/features/conversation/shared/execution src/features/capability/workgraph-distillations`，并执行 `pnpm run check`。浏览器覆盖检查点的未知效果提示、派发状态、模板参数/示例/统计，以及宽窄屏和明暗主题。
