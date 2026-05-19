# Desktop App Architecture Spec

## 1. 文档目标

本文档规划 Nexus 后续发布桌面 App 时的架构边界。

目标不是把现有 Web UI 套进一个壳，也不是照搬 Raycast 的四层实现，而是在当前 Go 后端和 React 前端之上，补齐真正桌面 App 需要负责的窗口、生命周期、系统集成、发布和更新链路。

参考资料：

- `yetone/native-feel-skill`
- Raycast 2.0 技术拆解
- Raycast Beta.app bundle 结构逆向记录

## 2. 架构结论

Nexus 第一阶段采用三层结构：

```text
Native Shell
  macOS: Swift + AppKit
  后续 Windows: C# + WPF / WinUI

System WebView
  macOS: WKWebView
  后续 Windows: WebView2

Go Sidecar
  复用当前 nexus-server / migration / runtime / storage
```

暂不引入 Node backend，也暂不引入 Rust core。

### 2.1 为什么不是 Electron / Tauri

Nexus 的桌面形态需要原生窗口、全局快捷键、菜单栏、系统通知、URL scheme、Keychain、自动更新和多屏焦点行为。这些能力在 Electron / Tauri 抽象层里可以做，但越到 native feel 的细节越会被抽象层限制。

第一阶段只做 macOS 时，更合理的选择是直接使用 Swift + AppKit，把跨平台抽象留到未来 Windows shell，而不是先引入一层桌面运行时。

### 2.2 为什么不是 Raycast 的 Node backend

Raycast 的 Node backend 服务于它的 JS/TS 扩展生态和历史业务代码。Nexus 当前真相源已经是 Go：

- 服务入口：`cmd/nexus-server`
- 迁移入口：`cmd/nexus-migrate`
- 协议真相源：`internal/protocol`
- 前端运行时地址解析：`web/src/config/options.ts`
- WebSocket 生命周期：`web/src/app/router/app-router.tsx`

因此 Raycast 的 Layer 3 在 Nexus 中应映射为 Go sidecar。业务后端不迁移到 Node，不新增第二套业务真相源。

### 2.3 为什么暂不引入 Rust core

Rust core 适合承载文件索引、模糊搜索、CPU 密集任务、跨端共享模型或需要 UniFFI 绑定的核心逻辑。Nexus 当前更急的是把桌面生命周期和本地运行链路跑通。

只有出现明确性能热点或跨端复用需求时，才新增 Rust core。新增前必须回答：

- 哪个 Go / TS 实现已经成为可测量瓶颈？
- Rust 是否需要被 Swift / Windows shell 直接调用？
- 这段逻辑是否需要与未来移动端或服务端共享？
- 新增进程或 dylib 后的错误路径、日志和发布成本是否可控？

## 3. 层级职责

### 3.1 Native Shell

Native Shell 是桌面 App 的主进程。它不承载业务逻辑，只负责 WebView 无法可靠完成的系统职责：

- App 生命周期：启动、恢复、隐藏、退出、单实例。
- 窗口管理：launcher、主工作区、settings、OAuth callback 结果窗口。
- 全局快捷键：唤起 launcher 或主窗口。
- 菜单栏 / Dock：状态、偏好设置、退出入口。
- 系统集成：URL scheme、通知、剪贴板、原生文件选择、拖拽文件 URL。
- 安全存储：Keychain 保存本地 token、connector client secret 这类敏感字段。
- 进程监管：启动、健康检查、停止、重启 Go sidecar。
- 发布能力：签名、公证、自动更新、崩溃上报、日志导出。

Native Shell 不直接读写 Nexus 业务数据库，不实现 DM / Room / Skill / Connector 业务规则。

### 3.2 System WebView

WebView 是 React 前端的渲染面，不是浏览器容器。

桌面端当前已经拆成多 entry：

- `launcher.html`：唤起入口，首屏轻、热启动快。
- `app.html`：连续工作区，承载 `/app`、Room、Contacts、Skills、Connectors。
- `settings.html`：设置窗口，未来可以独立生命周期。
- `oauth-callback.html`：OAuth 回调完成页，减少主窗口路由污染。

多 entry 的目标是让不同窗口按需加载，不让一个巨大 SPA 成为所有窗口的冷启动成本。

Shell 通过物理 HTML entry 承载窗口职责，通过 `desktop_route` query 传递原始业务路由。Go sidecar 的静态 fallback 负责把直接访问或刷新后的业务路径重新落回正确 entry：`/` 使用 `launcher.html`，`/app` 和连续工作区路由使用 `app.html`，`/settings` 使用 `settings.html`，OAuth callback 使用 `oauth-callback.html`。

轻入口使用独立 router 和收窄的 `modulepreload` 白名单：launcher、settings、OAuth callback 不互相静态引用页面组件，也不预拉主工作区页面 chunk；主 `app.html` 保持更积极的预加载，用于连续工作区的导航体验。

主入口也必须控制首屏静态依赖。`app.html` 不应因为兜底路由、loading 动画或桌面轻窗口而预拉 launcher、settings、OAuth callback、Room、markdown renderer、Lottie runtime 等非首屏重型 chunk。loading fallback 使用 CSS-only 实现；Lottie、markdown、OAuth dialog 这类功能依赖留在页面级 lazy chunk 内。

Vite 手动分包只保留真正跨入口共享且稳定的 runtime vendor，例如 React 和基础 UI 包。不要为了“看起来分类清晰”强行拆出 `vendor-lottie`、`vendor-markdown` 这类领域 vendor；在 pnpm 软链路径和 Rolldown 依赖图下，这类手动 chunk 可能反向承载 React runtime，导致主入口 HTML 预加载并不需要的重型依赖。

WebView 必须遵守 native feel 约束：

- 背景透明，由 shell 提供 `NSVisualEffectView` / 平台材料。
- 禁止 WebKit 默认右键菜单和链接预览。
- 启动前等待首帧，避免白屏 / 黑屏闪烁。
- hidden / occluded 状态下避免 WebKit timer 被过度节流。
- 输入法、焦点、Escape、Tab、滚动惯性按平台行为验收。
- 避免在桌面 chrome 上使用 `cursor: pointer`、Web 风格 modal、smooth scroll。

冷启动诊断必须是一条连续链路，而不是分散日志。第一阶段记录：

- Swift shell：`app.did_finish_launching`、single instance、sidecar config / process / health、window create、WebView load / navigation、`web.ready`、window reveal。
- Web：`bootstrap.module_loaded`、runtime options hydrate、React render scheduled、ready after paint、`web.ready`，通过 `window.webkit.messageHandlers.nexusDesktopLifecycle` 送回 shell。
- 隐藏窗口里的 WKWebView 可能节流 `requestAnimationFrame`，因此 ready signal 不能只依赖双层 rAF。前端应使用 rAF 优先、短 timer 兜底、单次去重的策略，并在 payload 中带 `source`，让 shell 能区分 `after_paint` 与 `timer_fallback`。
- Go sidecar：桌面 Web 静态托管记录 HTML fallback 和 asset 请求摘要，包括请求 kind、target、status、bytes 和 duration。

启动日志不得记录 OAuth code、state、token 或完整 query value。需要诊断路由时只记录 path 和 query key。

macOS shell 使用原生 material view 作为 WebView 的承载面，而不是把 WebView 直接塞进窗口。主工作区使用 `windowBackground` material，launcher 浮层使用 `popover` material，并保持 WebView under-page 背景透明。窗口遮挡、最小化和恢复事件必须进入时间线，用于后续判断 hidden / occluded 状态下的 timer 和 reload 行为。

### 3.3 Go Sidecar

Go Sidecar 复用当前服务端能力，是本地 App 的业务真相源。

职责：

- 自动执行 Goose migration。
- 启动 `nexus-server`，监听 loopback 随机端口。
- 维护 HTTP API `/nexus/v1/...`。
- 维护 WebSocket `/nexus/v1/chat/ws`。
- 管理 SQL、transcript、overlay、workspace。
- 运行 DM / Room / automation / channels / skills / connectors。
- 暴露本地健康检查和版本信息。

Go Sidecar 不应该绑定固定 `8010`。Shell 负责选择可用端口，并把运行时地址注入 WebView。

推荐启动形态：

```text
NEXUS_APP_MODE=desktop
NEXUS_DATA_DIR=~/Library/Application Support/Nexus
NEXUS_LOG_DIR=~/Library/Logs/Nexus
NEXUS_DESKTOP_SESSION_TOKEN=<shell-generated>
PORT=<random-loopback-port>
```

HTTP / WebSocket 请求需要校验 shell 生成的本地 session token，避免本机其他进程随意调用本地 API。HTTP 请求优先使用注入 header；WebSocket 握手使用 subprotocol，并由 WKWebView 本地 cookie 兜底，避免不同 WebKit 版本对自定义 subprotocol 的处理差异影响连接。

## 4. Bundle 结构

macOS 第一阶段 bundle 建议：

```text
Nexus.app/
  Contents/
    Info.plist
    MacOS/
      Nexus                 # Swift/AppKit shell
      nexus-server          # Go sidecar
      nexus-migrate         # 可选，或合并到 server 启动前执行
    Resources/
      Web/
        index.html
        app.html
        launcher.html
        settings.html
        oauth-callback.html
        assets/
      db/
        migrations/
      skills/
      AppIcon.icns
    Frameworks/
      Sentry.framework      # 可选
```

`index.html` 保留给浏览器开发和历史兼容；桌面窗口优先加载对应 entry。

## 5. 本地目录约定

桌面 App 不使用仓库根目录作为运行数据目录。

macOS：

```text
~/Library/Application Support/Nexus/
  nexus.db
  workspace/
  skills/
  cache/

~/Library/Logs/Nexus/
  shell.log
  sidecar.log
  runtime.log
```

敏感数据优先进入 Keychain。必须落 SQL 时，沿用现有加密机制，缺少密钥时不能静默降级为明文。桌面模式下，Native Shell 负责生成并注入 `CONNECTOR_CREDENTIALS_KEY`，Go sidecar 只接收运行时环境变量，不负责生成或明文持久化该密钥。

Keychain 策略按签名形态区分：

- 正式签名包默认使用 macOS Keychain，保证长期身份稳定。
- 开发模式和 ad-hoc 本地包默认使用 `~/Library/Application Support/Nexus/config/connector-credentials.key`，文件权限为 0600。ad-hoc 包每次构建都会改变代码签名身份，Keychain 旧 ACL 可能反复要求用户输入密码，不能放在启动热路径。
- `NEXUS_DESKTOP_KEYCHAIN_MODE=keychain|file|auto` 可覆盖默认策略，便于验证正式签名包和回归本地文件路径。

## 6. Bridge 与协议

Nexus 桌面 bridge 必须遵守一个原则：协议源头只有一份。

当前业务协议仍以 `internal/protocol` 为真相源，并通过 `cmd/protocol-tsgen` 生成前端类型。桌面 bridge 新增独立协议包时也应遵守同样规则：

```text
internal/protocol/desktop/
  model_bridge_request.go
  model_bridge_event.go
  typescript_bridge.go
  swift_bridge.go
```

第一阶段 bridge 能力控制在系统边界：

- `app.get_runtime_config`
- `app.open_external_url`
- `app.show_notification`
- `app.pick_directory`
- `app.pick_file`
- `app.reveal_in_finder`
- `app.read_secret`
- `app.write_secret`
- `app.export_logs`
- `app.get_app_version`
- `app.open_route`
- `app.close_launcher`
- `app.get_global_shortcut_status`
- `app.set_global_shortcut_enabled`
- `app.set_global_shortcut_accelerator`
- `app.reset_global_shortcut_accelerator`

禁止通过 bridge 绕过 Go API 直接改业务状态。DM、Room、Skill、Connector 等业务动作仍走 HTTP / WebSocket。

每条 bridge 消息必须包含：

- `schema_version`
- `request_id`
- `kind`
- `payload`

事件和请求使用不同枚举，不混用 `type` 字符串。

## 7. 前端适配边界

前端应保持浏览器部署和桌面部署共用同一套业务代码。

允许新增：

- `src/lib/desktop-bridge/`
- `src/config/desktop-runtime.ts`
- `src/types/generated/desktop-bridge.ts`

不允许新增：

- 专门为桌面复制一套 DM / Room 页面。
- 为桌面写第二套状态管理。
- 在页面组件里散落 `window.webkit` / `chrome.webview` 判断。

运行时地址注入优先顺序：

1. Native Shell 注入的 desktop runtime config。
2. `VITE_API_URL` / `VITE_WS_URL`。
3. 同源 `/nexus/v1` 和 `/nexus/v1/chat/ws`。

这样 Web 部署、Docker 部署和桌面部署可以共用 `web/src/config/options.ts` 的地址解析模式。

## 8. OAuth 与 URL Scheme

桌面 App 必须支持 URL scheme：

```text
nexus://connectors/oauth/callback
```

但第一阶段不强制所有第三方 provider 都迁到 custom scheme。

迁移策略：

- 本地 Web 开发继续支持 `http://localhost:3000/capability/connectors/oauth/callback`。
- 桌面 App 新增 `nexus://connectors/oauth/callback`。
- 后端 state 表必须记录 redirect kind，回调时按创建时的 redirect URI 校验。
- Native Shell 收到 URL event 后，把完整 callback URL 转交给 WebView 或 Go API。

## 9. 发布链路

当前 `Publish Release` workflow 已经负责同一个 tag 下的源码包、Linux/Windows 可运行包、release notes 和 GitHub Release 创建。macOS app 包也属于同一个 Release asset 集合，因此先复用同一个 workflow，但拆成独立 `macos_app` job 运行在 macOS runner 上；最终 `release` job 统一下载并上传全部 assets，避免两个 workflow 同时创建或追加同一个 Release。

流水线：

1. Checkout tag。
2. macOS job 设置 Go / Node / pnpm。
3. 执行 `scripts/desktop/package-macos-app.sh`，CI 环境允许慢 runner 触发主窗口 fallback reveal，但仍必须等到 `web.ready`、launcher ready、无 WebContent crash 和无 startup failure。
4. 脚本构建 `web/dist`、Go sidecar 与 Swift shell，并组装 `.app`。
5. 脚本执行 ad-hoc codesign、plist/codesign 校验和桌面 smoke。
6. 脚本生成 `Nexus-macos-<version>-<build>.dmg`、`.sha256` 与 `.metadata.json`。
7. macOS job 上传临时 workflow artifact。
8. Ubuntu release job 继续生成源码包与 Linux/Windows 可运行包。
9. Ubuntu release job 下载 macOS artifact，并统一上传到 GitHub Release。

等 Developer ID、Notary、公证 staple 和 Sparkle appcast 都进入正式发布阶段后，再考虑拆成独立 `publish-macos-app.yml` 或可复用 workflow。那时 macOS 发布线会有独立证书、secret、失败重试和更新通道。

没有 Developer ID 时，先走 ad-hoc 签名链路，不伪装成正式公证发布：

1. 本地执行 `scripts/desktop/package-macos-app.sh`。
2. 脚本固定版本号和构建号，生成 ad-hoc 签名 `.app`。
3. 执行 plist / codesign 校验和桌面 smoke。
4. 输出 zip 或 dmg、sha256 和 metadata。
5. metadata 必须标记 `signing.kind=ad-hoc`、`notarized=false`，并记录源码 commit 与 dirty 状态。
6. 分发前校验 sha256；测试机首次打开使用 Finder 右键 Open，或在可信内部机器上清理 quarantine。

第一版可以本地构建 `.app` 和 zip/dmg，但 public beta 前签名、公证和自动更新必须完成。

## 10. 验收标准

### 10.1 App 版本

- 双击 `Nexus.app` 可以启动。
- 首次启动可以完成 owner/bootstrap。
- 关闭窗口不会留下失控 sidecar。
- 退出 App 后 Go sidecar 退出。
- WebSocket 在 App 内路由切换时不断开。
- DM、Room、Contacts、Skills、Connectors 主流程可用。
- OAuth 回调能回到 App。
- 日志可以从 App 内导出。
- 本地数据目录不污染仓库。
- `docs/specs/desktop-app-qa-checklist.md` 中的桌面交互、OAuth 和诊断清单有明确通过/失败记录。

### 10.2 Public beta

- Developer ID 签名和 notarization 完成。
- Sparkle 或等价自动更新可用。
- 崩溃日志有符号化路径。
- Keychain 管理敏感 token。
- 全局快捷键可配置。
- 多屏唤起位置正确。
- 冷启动无明显白屏 / 黑屏闪烁。
- Pinyin IME、Tab、Escape、复制粘贴、滚动行为通过手测。
- idle 内存和后台 CPU 有基线记录。

### 10.3 v1.0

- macOS App 发布链路完全自动化。
- Windows shell 是否启动进入单独决策。
- bridge schema 有版本兼容策略。
- 升级、回滚、卸载、迁移、断网、权限拒绝都有测试记录。
- native feel audit 中 A 到 G 类阻塞项必须全部为 green。

## 11. 实施顺序

### 阶段 1：架构骨架

- 新增 `desktop/macos`。
- 建立 Swift/AppKit shell。
- 内嵌 `WKWebView` 加载本地 `web/dist`。
- Go sidecar 随 shell 启停。
- Shell 注入 API / WS 地址。

### 阶段 2：App Bundle

- 生成 `Nexus.app` bundle。
- Bundle 内包含 Swift shell、Go sidecar、`web/dist`、`db/migrations`。
- 使用 ad-hoc 签名支持本机验证。
- 从 bundle 直接启动，不能依赖 `go run`、仓库根目录或开发端口。
- App 退出、SIGTERM / SIGINT 终止时必须同步停止 Go sidecar。

### 阶段 3：协议与安全

- 新增 desktop bridge schema。
- 生成 TS / Swift 类型。
- 加本地 session token。
- 日志、健康检查和错误展示闭环。

### 阶段 4：桌面体验

- 全局快捷键。
- 菜单栏和 Dock 行为。
- URL scheme。
- Keychain。
- 原生文件选择 / 通知 / 外部链接。
- WebView 白屏、右键菜单、输入法、焦点、滚动修正。
- 维护 macOS app QA checklist，记录 IME、Tab/Escape、复制粘贴、外链、未知 scheme、OAuth 和诊断反馈结果。

### 阶段 5a：无 Developer ID app 发布

- 构建 ad-hoc `.app`。
- 运行桌面 smoke。
- 生成 zip/dmg、sha256、metadata。
- 文档写清 Gatekeeper 限制、安装路径、数据目录、日志目录和重置方式。

### 阶段 5b：正式发布

- Xcode archive。
- 签名、公证。
- `.dmg` / `.zip`。
- Sparkle appcast。
- GitHub Release artifact。

## 12. 明确不做

- 不恢复旧 Python 运行链路。
- 不把 Go 后端迁移到 Node。
- 不在第一阶段引入 Rust core。
- 不用 Electron / Tauri 做第一版桌面 App。
- 不为桌面复制第二套业务前端。
- 不让 Native Shell 直接读写业务数据库。

## 13. 一句话总结

Nexus 桌面 App 应该是一个原生 shell 监管 Go sidecar，并用系统 WebView 渲染现有 React 产品面的本地应用；native 负责系统感，Go 负责业务真相源，WebView 负责高迭代 UI。
