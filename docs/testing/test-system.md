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

`go test ./internal/storage -run TestDatabaseIntegration -count=1` 总是执行 SQLite；
设置 `NEXUS_TEST_POSTGRES_URL` 后，同一套迁移、Room owner/CAS、Goal JSON/时间/CAS/事件失败回滚、
Execution 幂等事件用例也在 PostgreSQL 执行。每次创建独立 schema，并在清理阶段删除。
变量缺失时仅 PostgreSQL 子测试跳过；配置存在但连接或迁移失败必须报错。
后端 CI 使用 PostgreSQL 17 service，显式设置变量，不允许该分支跳过。

现有 SQLite 专项测试继续保留。双驱动集成套件覆盖共用仓储边界，不代表所有
SQLite 专项用例都适用于 PostgreSQL；例如 SQLite PRAGMA 与历史编号修复有自己的测试。

`go test ./cmd/nexus-server -run TestPreviousReleaseDatabaseUpgrade -count=1`
使用冻结的 v0.2.5 合成数据快照，经真实启动迁移函数升级，校验数据与重复启动。
来源和重建方法见 `cmd/nexus-server/testdata/README.md`。不读取已安装 App 的数据。

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
