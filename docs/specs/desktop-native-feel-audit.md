# Desktop Native Feel Audit

## 1. 目标

本文档用于跟踪 Nexus macOS 桌面 App 的 native feel 阻塞项。判断标准不是“Web 页面能不能被包起来”，而是原生 shell 是否接管了桌面应用应该承担的生命周期、窗口、系统集成、权限、发布和可诊断能力。

状态说明：

- `Green`：已经实现并通过本地验证。
- `Yellow`：已有骨架，但还需要体验或异常路径补齐。
- `Red`：发布 dogfood 前仍缺失。

## 2. 当前审计

| 类别 | 状态 | 当前结论 | 下一步 |
| --- | --- | --- | --- |
| Native shell owns lifecycle | Green | Swift AppKit shell 已启动 Go sidecar，并在 Quit / SIGTERM / SIGINT 时停止 sidecar；关闭窗口仅隐藏，不杀进程；启动前会按 PID 记录清理崩溃后遗留的 bundled sidecar；启动失败会写诊断报告；WebView 内容进程终止会记录时间线并自动 reload。 | 后续补 crash report。 |
| Single instance | Green | 使用 `~/Library/Application Support/Nexus/NexusDesktop.lock` 做单实例锁；重复启动会通知已运行实例拉起主窗口。 | public beta 前补多用户/多 bundle identifier 策略。 |
| Dock and reopen behavior | Green | Dock 点击和重复启动都会重新显示主窗口。 | 多窗口后补 settings / launcher 恢复策略。 |
| Standard macOS menus | Green | 已补 About、Settings、Hide、Quit、Edit、Window、Reload 等标准菜单；启动器入口保留在菜单中，具体全局快捷键由设置页管理。 | 后续按 macOS Human Interface Guidelines 做菜单分组精修。 |
| System WebView boundary | Yellow | WKWebView 只允许同源本地页面留在内部；外部 `http` / `https` / `mailto` 统一交给系统打开，未知 scheme 阻断；首屏由 React ready signal 后再 reveal；ready signal 已处理隐藏窗口 rAF 可能被节流的问题；窗口内容已由 `NSVisualEffectView` material 承载，WebView 背景透明。 | 补真实输入法与键盘导航验证。 |
| Default browser affordances | Yellow | 默认右键菜单已关闭，返回/前进手势关闭。 | 继续核对链接预览、拖拽、文本输入、IME、Tab / Escape。 |
| Runtime config injection | Green | Shell 在 document start 注入 API、WebSocket、session token、版本和平台信息。 | 后续把 bridge schema 迁到协议生成。 |
| Local API protection | Green | Go sidecar 已校验桌面 session token；HTTP 使用 header，WebSocket 可通过 subprotocol 或 WKWebView 本地 cookie 通过校验。 | 增加 token rotate / sidecar restart 设计。 |
| URL scheme | Yellow | `Info.plist` 注册 `nexus://`，shell 能把 OAuth callback 转到 WebView 回调页。 | 联调真实 provider，确认 provider 后台已登记 custom scheme。 |
| OAuth desktop redirect | Yellow | 桌面 sidecar 设置 `CONNECTOR_OAUTH_REDIRECT_URI=nexus://connectors/oauth/callback`，后端 state 表记录 `redirect_kind`，前端授权与回调按 desktop runtime 使用相同 redirect URI。 | 真实 provider 联调后转 Green。 |
| Native bridge | Yellow | 当前支持版本读取、外链打开、日志导出、主窗口路由打开、launcher 关闭和全局快捷键状态读写。 | 用 Go 协议真相源生成 TS / Swift bridge 类型。 |
| Secure storage | Yellow | macOS shell 在正式签名包中优先用 Keychain 生成并持久化 connector credentials encryption key；开发模式和 ad-hoc 本地包默认直接使用 0600 本地密钥，避免反复重签后 Keychain ACL 弹密码或阻塞启动。Go sidecar 使用该 key 加密 OAuth client secret 和 connector credentials。 | 正式签名包验证 Keychain 不降级；后续补 bridge 级 `read_secret` / `write_secret`，把需要原生直接访问的敏感字段逐项迁入 Keychain。 |
| Global shortcut / launcher | Yellow | macOS shell 已用 Carbon 注册全局快捷键，默认 `Option + Space`；设置页可录制自定义组合键、开关快捷键、恢复默认值并显示注册失败原因；独立浮层窗口、原生菜单和 `nexus://launcher` 都可拉起 Web launcher；浮层内搜索、最近项和目标选择都会交回主窗口导航，失焦和 Escape 会关闭浮层；launcher 已使用独立 entry。 | 补更细的快捷键冲突引导和快捷键冲突恢复体验。 |
| Multi-entry WebView | Green | Vite 已输出 `app.html`、`launcher.html`、`settings.html`、`oauth-callback.html` 四个 entry；Swift shell 按窗口和 `nexus://` URL 选择入口，并用 `desktop_route` 传递业务路由；Go sidecar fallback 支持直接刷新 `/`、`/app`、`/settings` 和 OAuth callback；轻入口 router 已拆开，launcher/settings/OAuth 不再预拉彼此页面 chunk；主入口也已把 login、launcher、Lottie loading 和 markdown vendor 从首屏静态依赖中移出。 | 后续继续做真实冷启动计时和多窗口生命周期细化。 |
| Packaging | Yellow | 本地脚本可生成包含 Swift shell、Go sidecar、`web/dist`、migrations、内置 skills 的 `.app`，并做 ad-hoc 签名；`package-macos-dogfood.sh` 可生成内部 dogfood zip、sha256 和 metadata，并强制跑 smoke。 | 无 Developer ID 阶段继续用 ad-hoc dogfood；公开发布前补 Developer ID 签名、公证、DMG/ZIP。 |
| Updates | Red | 还没有自动更新。 | public beta 前接 Sparkle 或等价方案。 |
| Diagnostics | Yellow | 日志写入 `~/Library/Logs/Nexus`，设置页可触发日志导出；导出包包含机器可读 `diagnostics.json`，启动失败会落 `startup-failure-*.json` 并在错误弹窗中提示路径；Swift shell 已记录 `Nexus Startup` 时间线，覆盖 sidecar、window、WebView navigation、Web ready/reveal、窗口遮挡/最小化和 WebContent 进程终止；Web ready payload 带 performance marks，Go 静态托管记录桌面 Web 资源请求摘要。 | 加 crash report 和更完整的 startup failure UI。 |

## 3. Dogfood 前必须变 Green

- App 生命周期：Quit、Close、Dock reopen、重复启动、异常退出都可解释。
- WebView 边界：外链、OAuth、未知 scheme、右键菜单、复制粘贴、输入法、首屏 ready gate 通过手测。
- 本地 API 安全：无 token 请求不能访问 `/nexus/v1/*`，WebSocket 不能绕过。
- 日志导出：无需仓库上下文即可导出 shell / sidecar 日志和机器可读 diagnostics。
- Bundle 独立性：双击 `.app` 不依赖 `go run`、开发端口或仓库根目录。

## 4. Public beta 前必须变 Green

- Developer ID 签名、公证、staple；这是 Apple 账号/证书外部依赖，无证书阶段只做内部 dogfood。
- 自动更新。
- Keychain：connector credentials encryption key 已完成；开发/ad-hoc 包默认走本地 0600 文件，正式签名包需确认不触发本地文件降级，逐项 secret API 仍需补齐。
- URL scheme + OAuth redirect kind 真实 provider 闭环。
- 冷启动白闪治理、首屏 ready gate 和 occlusion 长时间/异常路径验证。
- 全局快捷键和 launcher。
- 崩溃日志与可诊断启动失败报告。

## 5. 最近验证快照

2026-05-15：

- `swift build --package-path desktop/macos -c release`
- `pnpm --dir web run typecheck`
- `pnpm --dir web run lint`
- `scripts/desktop/build-macos-app.sh`
- `plutil -lint desktop/macos/.build/app/Nexus.app/Contents/Info.plist`
- `codesign --verify --deep --strict desktop/macos/.build/app/Nexus.app`
- `scripts/desktop/smoke-macos-app.sh`
- `scripts/desktop/package-macos-dogfood.sh`
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
- 无 Developer ID dogfood 打包：新增 `scripts/desktop/package-macos-dogfood.sh`，固定版本号/构建号后构建 `.app`、校验 plist / codesign、执行 smoke，再输出 `Nexus-macos-<version>-<build>.zip`、`.sha256` 和 `.metadata.json`；metadata 明确 `signing.kind=ad-hoc`、`notarized=false`、`keychain.expected_storage=file`。
