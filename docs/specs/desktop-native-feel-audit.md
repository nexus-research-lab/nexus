# Desktop Native Feel Audit

## 1. 目标

本文档用于跟踪 Nexus macOS 桌面 App 的 native feel 阻塞项。判断标准不是“Web 页面能不能被包起来”，而是原生 shell 是否接管了桌面应用应该承担的生命周期、窗口、系统集成、权限、发布和可诊断能力。

状态说明：

- `Green`：已经实现并通过本地验证。
- `Yellow`：已有骨架，但还需要体验或异常路径补齐。
- `Red`：发布 macOS app 前仍缺失。

## 2. 当前审计

| 类别 | 状态 | 当前结论 | 下一步 |
| --- | --- | --- | --- |
| Native shell owns lifecycle | Green | Swift AppKit shell 已启动 Go sidecar，并在 Quit / SIGTERM / SIGINT 时停止 sidecar；关闭窗口仅隐藏，不杀进程；启动前会按 PID 记录清理崩溃后遗留的 bundled sidecar；启动失败会写诊断报告；WebView 内容进程终止会记录时间线并自动 reload。 | 后续补 crash report。 |
| Single instance | Green | 使用 `~/Library/Application Support/Nexus/NexusDesktop.lock` 做单实例锁；重复启动会通知已运行实例拉起 launcher。 | public beta 前补多用户/多 bundle identifier 策略。 |
| Dock and reopen behavior | Green | 冷启动、Dock 点击、重复启动、`nexus://open` 和 `nexus://launcher` 都默认显示主窗口完整 launcher 首页；用户在 launcher 内选择进入工作台后才进入 `/app`。 | 多窗口后补 settings / launcher 恢复策略。 |
| Standard macOS menus | Green | 已补 About、Settings、Hide、Quit、Edit、Window、Reload 等标准菜单；启动器入口保留在菜单中。 | 后续按 macOS Human Interface Guidelines 做菜单分组精修。 |
| System WebView boundary | Yellow | WKWebView 只允许同源本地页面留在内部；外部 `http` / `https` / `mailto` 统一交给系统打开，未知 scheme 阻断；首屏由 React ready signal 后再 reveal；ready signal 已处理隐藏窗口 rAF 可能被节流的问题；窗口内容已由 `NSVisualEffectView` material 承载，WebView 背景透明；外链打开、popup 外链、未知 scheme 阻断会写入启动时间线。 | 补真实输入法与键盘导航验证。 |
| Default browser affordances | Yellow | 默认右键菜单已关闭，返回/前进手势关闭；右键菜单抑制会进入诊断时间线。 | 继续核对链接预览、拖拽、文本输入、IME、Tab / Escape。 |
| Runtime config injection | Green | Shell 在 document start 注入 API、WebSocket、session token、版本和平台信息。 | 后续把 bridge schema 迁到协议生成。 |
| Local API protection | Green | Go sidecar 已校验桌面 session token；HTTP 使用 header，WebSocket 可通过 subprotocol 或 WKWebView 本地 cookie 通过校验。 | 增加 token rotate / sidecar restart 设计。 |
| URL scheme | Yellow | `Info.plist` 注册 `nexus://`，shell 能把 OAuth callback 转到 WebView 回调页。 | 联调真实 provider，确认 provider 后台已登记 custom scheme。 |
| OAuth desktop redirect | Yellow | 桌面 sidecar 设置 `CONNECTOR_OAUTH_REDIRECT_URI=nexus://connectors/oauth/callback`，后端 state 表记录 `redirect_kind`，前端授权与回调按 desktop runtime 使用相同 redirect URI。 | 真实 provider 联调后转 Green。 |
| Native bridge | Yellow | 当前支持版本读取、外链打开、日志导出、主窗口路由打开和全局快捷键状态读写。 | 用 Go 协议真相源生成 TS / Swift bridge 类型。 |
| Secure storage | Yellow | macOS shell 在正式签名包中优先用 Keychain 生成并持久化 connector credentials encryption key；开发模式和 ad-hoc 本地包默认直接使用 0600 本地密钥，避免反复重签后 Keychain ACL 弹密码或阻塞启动。Go sidecar 使用该 key 加密 OAuth client secret 和 connector credentials。 | 正式签名包验证 Keychain 不降级；后续补 bridge 级 `read_secret` / `write_secret`，把需要原生直接访问的敏感字段逐项迁入 Keychain。 |
| Global shortcut / launcher | Yellow | `Option + Space` 不再默认注册，历史默认组合会在启动时关闭；窗口菜单和 `nexus://launcher` 都把主窗口导航到完整 launcher 首页；设置页已移除启动器快捷键配置。 | 后续若恢复全局快捷键，再补冲突引导和恢复体验。 |
| Multi-entry WebView | Green | Vite 已输出 `app.html`、`settings.html`、`oauth-callback.html` 三个桌面 entry；Swift shell 按窗口和 `nexus://` URL 选择入口，并用 `desktop_route` 传递业务路由；Go sidecar fallback 支持直接刷新 `/`、`/app`、`/settings` 和 OAuth callback；settings/OAuth 轻入口已拆开，不预拉主工作区页面 chunk。 | 后续继续做真实冷启动计时和多窗口生命周期细化。 |
| Packaging | Yellow | 本地脚本可生成包含 Swift shell、Go sidecar、`web/dist`、migrations、内置 skills 的 `.app`，并做 ad-hoc 签名；`package-macos-app.sh` 可生成 zip/dmg、sha256 和 metadata，并强制跑 smoke；GitHub `Publish Release` 已增加独立 macOS job，把 dmg 作为同一个 tag 的 Release asset 上传。 | 无 Developer ID 阶段继续用 ad-hoc 签名；公开发布前补 Developer ID 签名、公证和 staple。 |
| Updates | Yellow | macOS 原生壳已支持启动后按 24 小时节流检测 GitHub Release / macOS metadata，应用菜单提供“检查更新...”手动入口，发现新版本时用原生弹窗提示打开下载页；当前不会自动下载或安装。 | public beta 前接 Sparkle 或等价方案，并完成 Developer ID 签名、公证、appcast 和更新包签名。 |
| Diagnostics | Yellow | 日志写入 `~/Library/Logs/Nexus`，设置页可触发日志导出；导出包包含机器可读 `diagnostics.json`，启动失败会落 `startup-failure-*.json` 并在错误弹窗中提示路径；Swift shell 已记录 `Nexus Startup` 时间线，覆盖 sidecar、window、WebView navigation、Web ready/reveal、窗口遮挡/最小化、外链/阻断、右键菜单抑制和 WebContent 进程终止；WebContent 终止会额外写 `webcontent-terminated-*.json`；Web ready payload 带 performance marks，Go 静态托管记录桌面 Web 资源请求摘要。 | 加符号化 crash report 和更完整的 startup failure UI。 |

## 3. App 发布前必须变 Green

- App 生命周期：Quit、Close、Dock reopen、重复启动、异常退出都可解释。
- WebView 边界：外链、OAuth、未知 scheme、右键菜单、复制粘贴、输入法、首屏 ready gate 通过手测。
- 本地 API 安全：无 token 请求不能访问 `/nexus/v1/*`，WebSocket 不能绕过。
- 日志导出：无需仓库上下文即可导出 shell / sidecar 日志和机器可读 diagnostics。
- Bundle 独立性：双击 `.app` 不依赖 `go run`、开发端口或仓库根目录。

## 4. Public beta 前必须变 Green

- Developer ID 签名、公证、staple；这是 Apple 账号/证书外部依赖。
- 自动更新。
- Keychain：connector credentials encryption key 已完成；开发/ad-hoc 包默认走本地 0600 文件，正式签名包需确认不触发本地文件降级，逐项 secret API 仍需补齐。
- URL scheme + OAuth redirect kind 真实 provider 闭环。
- 冷启动白闪治理、首屏 ready gate 和 occlusion 长时间/异常路径验证。
- 全局快捷键和 launcher。
- 崩溃日志与可诊断启动失败报告。

## 5. 最近验证快照

2026-05-19：

- Windows 原生壳进入第一阶段：新增 `desktop/windows/Nexus.Desktop`，使用 WPF + WebView2 承载 `web/dist/app.html`，默认 `desktop_route=/` 显示完整 launcher。
- Windows shell 已具备 Go sidecar 监管骨架：随机 loopback 端口、`NEXUS_DESKTOP_SESSION_TOKEN`、`WEB_DIST_DIR`、SQLite 本地数据目录、日志目录、DPAPI connector credentials key 和 OAuth custom scheme 环境变量均由壳注入。
- Windows bridge 先覆盖版本读取、外链打开、日志导出、主窗口路由打开和全局快捷键状态占位；当前不注册全局快捷键，也不暴露启动器快捷键配置。
- Windows 生命周期骨架已补单实例 mutex、named pipe 二次启动唤起、`nexus://launcher/open/settings/connectors/oauth/callback` 路由解析，以及 WebView2 外链打开、未知 scheme 阻断和 WebContent process failed 时间线。
- Windows 构建入口新增 `scripts/desktop/build-windows-app.ps1`、`scripts/desktop/smoke-windows-app.ps1`、`make app-win-build`、`make app-win-smoke` 和 `make app-win-package`，默认组装到 `desktop/windows/.build/app/Nexus/`，可额外输出 zip/sha256。当前机器缺少 `dotnet` 和 `pwsh`，本轮只能做静态验证，实际 WPF/WebView2 构建与 smoke 需要在 Windows 环境补跑。
- GitHub `Publish Release` workflow 复用现有发布入口，新增 `macos_app` job 在 macOS runner 上执行 `scripts/desktop/package-macos-app.sh`，并把 dmg、sha256、metadata 交给最终 `release` job 统一上传到同一个 GitHub Release。
- 默认入口改为 launcher：冷启动、Dock reopen、重复启动已有实例、`nexus://open` 和 `nexus://launcher` 都不再直接进入 `/app`。
- 本轮调整后，默认 launcher 不再是紧凑浮层，而是主窗口 `/` 完整首页；smoke 改为验证 `main_window.created` + `web.ready location_path=/`，再分别通过 `nexus://open` 和 `nexus://launcher` 验证仍回到 `/`。
- 桌面 smoke 脚本在启动前显式向 LaunchServices 注册 `.app`，并在 custom scheme 未投递时回退到 shell 内部的 launcher 分布式通知，避免干净 CI runner 上 `nexus://launcher` 不稳定导致 smoke 失败。
- GitHub macOS job 开启 `NEXUS_DESKTOP_SMOKE_ALLOW_FALLBACK=1`，允许慢 runner 上主窗口先以 `fallback_timeout` reveal，但仍要求后续 `web.ready` 到达，并继续拦截 WebContent crash 和 startup failure；本地默认 smoke 仍保持严格模式。
- macOS app 发布仍标记签名状态：ad-hoc signing、未 notarize、metadata 保留 `developer_id=false` 和 `notarized=false`。
- 桌面交互 QA 清单新增 `docs/specs/desktop-app-qa-checklist.md`，覆盖 IME、Tab/Escape、复制粘贴、右键菜单、外链、未知 scheme、真实 OAuth provider 和诊断反馈格式。
- WebView 外链打开、popup 外链、未知 scheme 阻断和右键菜单抑制进入 `Nexus Startup` 时间线；WebContent 终止会额外写 `webcontent-terminated-*.json`，日志导出的 `diagnostics.json` 增加 URL scheme 声明检查。

2026-05-15：

- `swift build --package-path desktop/macos -c release`
- `pnpm --dir web run typecheck`
- `pnpm --dir web run lint`
- `scripts/desktop/build-macos-app.sh`
- `plutil -lint desktop/macos/.build/app/Nexus.app/Contents/Info.plist`
- `codesign --verify --deep --strict desktop/macos/.build/app/Nexus.app`
- `scripts/desktop/smoke-macos-app.sh`
- `scripts/desktop/package-macos-app.sh`
- 打包 `.app` 手动烟测：启动主窗口、打开 `nexus://settings`、确认 settings bundle 与偏好接口返回 200、退出后无 `Nexus` / `nexus-server` 残留。
- 生命周期补充烟测：启动时写入 `NexusSidecar.pid.json`，正常退出后删除 PID 记录。
- Multi-entry WebView 烟测：`/` 命中 `launcher` chunk，`/app` 命中 `app` chunk，`/settings` 命中 `settings` chunk，`/capability/connectors/oauth/callback` 命中 `oauth_callback` chunk；`nexus://settings` 和 `nexus://launcher` 均落到对应 HTML entry。
- 轻入口预加载收口：`launcher.html` / `settings.html` 预加载条目收敛到 16 条，`oauth-callback.html` 收敛到 13 条。
- 主入口首屏收口：`app.html` 预加载条目为 34 条，且不再引用 `launcher-page`、`login-page`、`lottie`、`markdown-renderer`、`settings-page`、`connector-oauth`、`room-page` 等非首屏重型 chunk；loading fallback 使用纯 CSS，不再引入 Lottie runtime。
- 冷启动观测链路：shell 已接入 `[Nexus Startup]` 分段事件；manual log export 的 `diagnostics.json` 包含 `startup_timeline`；Web `web.ready` 只上报 path、source 和 performance marks，不记录 OAuth query value；sidecar 对桌面 Web HTML / asset 请求记录 method、path、kind、target、status、bytes、duration_ms。
- 冷启动实机 smoke：`webview.navigation_finished` 2.42s，`webview.bridge_probe` 确认 `has_lifecycle_handler=true` / `has_runtime=true`，`web.ready source=timer_fallback` 2.65s，`main_window.revealed source=web.ready` 2.65s；未再触发 3s 原生 fallback reveal。
- Keychain 启动热路径收口：开发模式和 ad-hoc 本地包不会访问 Keychain；`NEXUS_DESKTOP_KEYCHAIN_MODE=keychain|file|auto` 可强制切换，用于正式签名包验证。
- 原生 material 收口：主窗口使用 `NSVisualEffectView.Material.windowBackground`，launcher 使用 `popover` material 和圆角承载面；WKWebView 设置透明 under-page 背景；窗口 occlusion、miniaturize / deminiaturize 进入启动时间线。
- material / Keychain / launcher 复合 smoke：ad-hoc `.app` 启动时 `sidecar.credentials_key_ready` 156.8ms，`storage=file reason=ad_hoc_signature`，未触发 Keychain 密码弹窗；主窗口 `main_window.created material=windowBackground`，`web.ready source=timer_fallback` 1.47s 后 `main_window.revealed source=web.ready`；`nexus://launcher` 创建 `launcher_window.created material=popover`，`web.ready source=after_paint` 后 reveal；主窗口和 launcher 均记录到 `occlusion_changed`。
- 桌面 smoke 脚本：新增 `scripts/desktop/smoke-macos-app.sh`，自动启动最终 `.app`、校验 credentials storage、主窗口 ready reveal、launcher ready reveal、material 标记、无 fallback / WebContent crash / startup failure，并确认退出后 bundled sidecar 无残留。
- 无 Developer ID app 打包：新增 `scripts/desktop/package-macos-app.sh`，固定版本号/构建号后构建 `.app`、校验 plist / codesign、执行 smoke，再输出 `Nexus-macos-<version>-<build>` 的 zip 或 dmg、`.sha256` 和 `.metadata.json`；metadata 明确 `signing.kind=ad-hoc`、`notarized=false`、`keychain.expected_storage=file`。
