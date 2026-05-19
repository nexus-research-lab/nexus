# Nexus macOS Shell

这是 Nexus 桌面 App 的 macOS shell。

当前形态：

- SwiftPM 可执行程序，使用 AppKit + WKWebView。
- 开发模式下从仓库根目录启动 `go run ./cmd/nexus-server`。
- Bundle 模式下从 `.app/Contents/MacOS/nexus-server` 启动 Go sidecar。
- Shell 自动分配 loopback 随机端口。
- Sidecar 通过 `WEB_DIST_DIR` 托管 `web/dist`，WebView 访问同源 `http://127.0.0.1:<port>/`。
- Shell 在 document start 注入 `window.__NEXUS_DESKTOP_RUNTIME__`，前端优先使用注入的 API / WebSocket 地址。
- 桌面运行数据写入 `~/Library/Application Support/Nexus`，日志写入 `~/Library/Logs/Nexus`。
- Shell 会在 `~/Library/Application Support/Nexus/NexusSidecar.pid.json` 记录当前 sidecar；下次启动前会清理同 bundle 路径下的崩溃遗留进程。
- Shell 会把本地 session token 同步进 WKWebView cookie store，保证 WebSocket 握手也能通过本地 API 校验。
- Shell 在正式签名包中优先使用 macOS Keychain 持久化 connector credentials encryption key；开发模式和 ad-hoc 本地包默认直接使用 `~/Library/Application Support/Nexus/config/connector-credentials.key` 的 0600 本地密钥，避免反复重签后 Keychain ACL 弹密码或阻塞启动。sidecar 通过 `CONNECTOR_CREDENTIALS_KEY` 使用现有 Go 加密存储。
- Shell 负责单实例、Dock 重新打开、标准菜单、外链拦截和 `nexus://` URL scheme。
- Shell 使用 `NSVisualEffectView` material 承载 WKWebView：主窗口使用 `windowBackground` material，launcher 使用 `popover` material 和圆角承载面，WKWebView under-page 背景保持透明。
- Shell 默认注册 `Option + Space` 全局快捷键，可从系统任意位置唤起独立紧凑 launcher 浮层；窗口菜单也会展示“显示启动器”入口，浮层支持失焦隐藏和 Escape 关闭。
- 设置页可读取全局快捷键注册状态、显示冲突失败原因，并录制、开关或恢复默认快捷键。
- Shell 会按窗口职责加载 `app.html`、`launcher.html`、`settings.html`、`oauth-callback.html`，并用 `desktop_route` 把原始业务路由交给前端；sidecar 静态 fallback 支持直接刷新这些业务路径，轻入口不会预拉主工作区页面 chunk，主入口也不会在首屏预拉 launcher、login、Lottie、markdown、settings、OAuth 或 Room 重型 chunk。
- 最小 native bridge 已支持版本读取、外链打开、日志导出、主窗口路由打开、launcher 关闭和全局快捷键状态读写。
- 日志导出包会包含 `diagnostics.json`，记录版本、系统、bundle、runtime URL、关键目录和本地文件存在性；启动失败会在 `~/Library/Logs/Nexus` 写入 `startup-failure-*.json`。
- Shell 会写 `[Nexus Startup]` 冷启动时间线，覆盖 sidecar、窗口、WebView navigation、Web ready 和 reveal；日志导出的 `diagnostics.json` 会带上 `startup_timeline`。
- 窗口遮挡、最小化和恢复事件会进入启动时间线，便于继续验证 occlusion 下的 WebView 行为。
- WebView 内容进程终止时，Shell 会记录 `webview.content_process_terminated`、写入 `~/Library/Logs/Nexus/webcontent-terminated-*.json` 并 reload 当前路由，避免 WebContent crash 后停在空白窗口。
- Shell 会记录外链打开、未知 scheme 阻断、右键菜单抑制和 launcher 关闭原因，便于桌面 QA 追踪 native 行为。
- 前端 ready signal 会带 source 和 performance marks；隐藏窗口 rAF 被节流时会用短 timer 兜底，避免主窗口等待 ready 时只能靠原生 fallback reveal。sidecar 会记录桌面 Web 静态资源请求摘要；两边都只记录 path 和 query key，不记录 OAuth code/state/token 等 query value。
- 首屏通过前端 ready signal 后再显示窗口，避免直接暴露 WebView 白屏。
- 桌面 OAuth 默认使用 `nexus://connectors/oauth/callback`，由 shell 转回本地 WebView 回调页。

## 开发命令

```bash
scripts/desktop/build-macos-dev.sh
scripts/desktop/run-macos-dev.sh
swift scripts/desktop/generate-macos-icon.swift
scripts/desktop/build-macos-app.sh
scripts/desktop/run-macos-app.sh
scripts/desktop/smoke-macos-app.sh
scripts/desktop/package-macos-app.sh
```

`run-macos-dev.sh` 会先构建前端，再启动 Swift shell。首次启动会初始化桌面专用 SQLite 数据库。
`generate-macos-icon.swift` 会从 `desktop/macos/Resources/AppIconSource.png` 生成 `desktop/macos/Resources/AppIcon.icns`，用于 `.app` 的 Finder / Dock 图标。
`build-macos-app.sh` 会组装 `desktop/macos/.build/app/Nexus.app`，其中包含 Swift shell、Go sidecar、`web/dist`、`db/migrations` 与内置 `skills`。
`smoke-macos-app.sh` 会启动已组装 `.app`，校验 ad-hoc Keychain 旁路、主窗口 ready reveal、launcher reveal、material 标记和退出后 sidecar 无残留。
`package-macos-app.sh` 会先构建 `.app`、跑 smoke，再输出 zip/dmg、sha256 和 metadata。
人工 macOS app 验收步骤维护在 `docs/specs/desktop-app-qa-checklist.md`。

本地验证 Keychain 时可以显式设置：

```bash
NEXUS_DESKTOP_KEYCHAIN_MODE=keychain scripts/desktop/run-macos-app.sh
```

默认 `auto` 会在 ad-hoc 本地包中绕开 Keychain。正式签名、公证后的包再验证 Keychain 不降级。

## App 打包

没有 Developer ID 时，当前包仍是 ad-hoc 签名且未公证：

```bash
make app-dmg
```

默认输出到 `desktop/macos/.build/package/`：

- `Nexus-macos-<version>-<build>.dmg`
- `Nexus-macos-<version>-<build>.dmg.sha256`
- `Nexus-macos-<version>-<build>.dmg.metadata.json`

安装前先校验 sha256：

```bash
cd desktop/macos/.build/package
shasum -a 256 -c Nexus-macos-<version>-<build>.dmg.sha256
```

打开 dmg 后，把 `Nexus.app` 拖到同一窗口里的 `Applications`。因为当前包是 ad-hoc 签名且未公证，macOS 可能拦截首次打开；可信构建优先用 Finder 右键 Open。仅本地测试机器可在校验 sha256 后清理 quarantine：

```bash
xattr -dr com.apple.quarantine /Applications/Nexus.app
```

卸载或重置应用数据时，先退出 Nexus，再按需要删除：

- `/Applications/Nexus.app`
- `~/Library/Application Support/Nexus`
- `~/Library/Logs/Nexus`

## 当前边界

- 还没有 Developer ID 签名、公证和自动更新；当前 macOS 包是 ad-hoc 签名。
- 还没有由 Go 协议真相源生成的 desktop bridge schema。
- 还没有更完整的快捷键冲突引导、逐项 secret 级 Keychain API、occlusion 长时间/异常路径验证和多窗口生命周期细化。
