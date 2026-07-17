# Nexus macOS Shell

这是 Nexus 桌面 App 的 macOS shell。

当前形态：

- SwiftPM 可执行程序，使用 AppKit + WKWebView。
- 开发模式下从仓库根目录启动 `go run ./cmd/nexus-server`。
- Bundle 模式下从 `.app/Contents/MacOS/nexus-server` 启动 Go sidecar，并优先使用 `.app/Contents/Resources/bin/nxs` 作为 `nxs` runtime。
- Shell 自动分配 loopback 随机端口。
- Sidecar 通过 `WEB_DIST_DIR` 托管 `web/dist`，WebView 访问同源 `http://127.0.0.1:<port>/`。
- Shell 在 document start 注入 `window.__NEXUS_DESKTOP_RUNTIME__`，前端优先使用注入的 API / WebSocket 地址。
- 桌面运行数据统一写入 `~/.nexus`，日志写入 `~/.nexus/logs`。
- Shell 会在 `~/.nexus/NexusSidecar.pid.json` 记录当前 sidecar；下次启动前会清理同 bundle 路径下的崩溃遗留进程。
- Shell 会把本地 session token 同步进 WKWebView cookie store，保证 WebSocket 握手也能通过本地 API 校验。
- Shell 在正式签名包中优先使用 macOS Keychain 持久化 connector credentials encryption key；开发模式和 ad-hoc 本地包默认直接使用 `~/.nexus/config/connector-credentials.key` 的 0600 本地密钥，避免反复重签后 Keychain ACL 弹密码或阻塞启动。sidecar 通过 `CONNECTOR_CREDENTIALS_KEY` 使用现有 Go 加密存储。
- Shell 负责单实例、Dock 重新打开、标准菜单、外链拦截和 `nexus://` URL scheme；冷启动和重复启动已有实例默认显示 launcher，Dock 重新打开只恢复现有主窗口，不主动改写当前路由。
- Shell 使用 `NSVisualEffectView` material 承载 WKWebView：主窗口使用 `windowBackground` material，WKWebView under-page 背景保持透明。
- Shell 不再默认注册 `Option + Space` 全局唤起；窗口菜单仍保留“显示启动器”入口，设置页不再展示启动器快捷键配置。
- Shell 会按窗口职责加载 `app.html`、`settings.html`、`oauth-callback.html`，并用 `desktop_route` 把原始业务路由交给前端；`/launcher` 由主窗口 `app.html` 承载，sidecar 静态 fallback 支持直接刷新 `/launcher`、`/app`、`/settings` 和 OAuth callback。
- 最小 native bridge 已支持版本读取、外链打开、日志导出、主窗口路由打开和全局快捷键状态读写。
- 日志导出包会包含 `diagnostics.json`，记录版本、系统、bundle、runtime URL、关键目录和本地文件存在性；启动失败会在 `~/.nexus/logs` 写入 `startup-failure-*.json`。
- Shell 会写 `[Nexus Startup]` 冷启动时间线，覆盖 sidecar、窗口、WebView navigation、Web ready 和 reveal；日志导出的 `diagnostics.json` 会带上 `startup_timeline`。
- 窗口遮挡、最小化和恢复事件会进入启动时间线，便于继续验证 occlusion 下的 WebView 行为。
- WebView 内容进程终止时，Shell 会记录 `webview.content_process_terminated`、写入 `~/.nexus/logs/webcontent-terminated-*.json` 并 reload 当前路由，避免 WebContent crash 后停在空白窗口。
- Shell 会记录外链打开、未知 scheme 阻断和右键菜单抑制，便于桌面 QA 追踪 native 行为。
- 前端 ready signal 会带 source 和 performance marks；隐藏窗口 rAF 被节流时会用短 timer 兜底，避免主窗口等待 ready 时只能靠原生 fallback reveal。sidecar 会记录桌面 Web 静态资源请求摘要；两边都只记录 path 和 query key，不记录 OAuth code/state/token 等 query value。
- 首屏通过前端 ready signal 后再显示窗口，避免直接暴露 WebView 白屏。
- 桌面 OAuth 默认使用 `http://127.0.0.1:34343/capability/connectors/oauth/callback`，由本地 sidecar 接收 provider 回调；GitHub 在桌面包中走 Device Flow，只需要 `NEXUS_DESKTOP_GITHUB_CLIENT_ID` 注入公开 Client ID，不打包 Client Secret。

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
`smoke-macos-app.sh` 会启动已组装 `.app`，校验 ad-hoc Keychain 旁路、主窗口默认 launcher ready reveal、显式 `/app` 路由 ready、material 标记和退出后 sidecar 无残留。
`package-macos-app.sh` 会先构建 `.app`、下载并预置当前平台的 `nxs` runtime、跑 smoke，再输出 zip/dmg、sha256 和 metadata。
人工 macOS app 验收步骤维护在 `docs/specs/desktop-app-qa-checklist.md`。

本地验证 Keychain 时可以显式设置：

```bash
NEXUS_DESKTOP_KEYCHAIN_MODE=keychain scripts/desktop/run-macos-app.sh
```

默认 `auto` 会在 ad-hoc 本地包中绕开 Keychain。正式签名、公证后的包再验证 Keychain 不降级。

## App 打包

本地测试包默认使用 ad-hoc 签名且不公证：

```bash
make app-dmg
```

正式对外分发时，使用 Apple Developer 账号下的 `Developer ID Application` 证书签名，并通过 Apple notary service 公证。先在钥匙串确认本机已有证书：

```bash
security find-identity -v -p codesigning | grep "Developer ID Application"
```

首次配置公证凭据时，把 Apple ID、Team ID 和 App 专用密码存进本机钥匙串 profile：

```bash
xcrun notarytool store-credentials nexus-notary \
  --apple-id "you@example.com" \
  --team-id "TEAMID" \
  --password "app-specific-password"
```

正式打包命令：

```bash
export NEXUS_DESKTOP_CODESIGN_IDENTITY="Developer ID Application: Your Name (TEAMID)"
export NEXUS_DESKTOP_NOTARIZE=1
export NEXUS_DESKTOP_NOTARY_PROFILE=nexus-notary
make app-dmg
```

`build-macos-app.sh` 在 Developer ID 签名时默认启用 hardened runtime 和 timestamp；`package-macos-app.sh` 会先提交 `.app` 公证并 staple，再生成 dmg。dmg 默认也会提交并 staple；如果只想公证包内 `.app`，可设置 `NEXUS_DESKTOP_NOTARIZE_DMG=0`。如果用证书 SHA-1 而不是完整名称作为签名 identity，同时设置 `NEXUS_DESKTOP_CODESIGN_DEVELOPER_ID=1`。

GitHub Actions 的 `macOS Desktop Build` workflow 会在 PR/main 上构建、smoke 并生成 ad-hoc dmg 验证打包路径，但不会使用 Apple 证书或上传 Release。`Publish Release` workflow 会在 macOS job 中导入 Developer ID `.p12` 到临时 keychain，并把公证后的 dmg、sha256、metadata 上传到 Release。仓库需要配置：

| 名称 | 类型 | 说明 |
|------|------|------|
| `MACOS_DEVELOPER_ID_APPLICATION` | Repository variable 或 secret | `Developer ID Application: Your Name (TEAMID)` |
| `MACOS_DEVELOPER_ID_CERTIFICATE_BASE64` | Secret | Developer ID Application `.p12` 的 base64 内容 |
| `MACOS_DEVELOPER_ID_CERTIFICATE_PASSWORD` | Secret | 导出 `.p12` 时设置的密码 |
| `APPLE_NOTARY_APPLE_ID` | Secret | Apple Developer 账号邮箱 |
| `APPLE_NOTARY_TEAM_ID` | Secret | Team ID |
| `APPLE_NOTARY_PASSWORD` | Secret | Apple ID App 专用密码 |

导出 `.p12` 后可用下面命令生成 secret 内容：

```bash
base64 -i DeveloperIDApplication.p12 | pbcopy
```

打包默认从 bridge runtime release 的 `nxs-stable` 通道下载并预置当前平台的 `nxs` runtime。可通过 `NEXUS_DESKTOP_NXS_RELEASE` 固定到某个 `nxs-v*` 版本。如目标 release 不是公开可匿名下载，需配置 `NEXUS_DESKTOP_NXS_DOWNLOAD_TOKEN`，或在 GitHub Actions 中配置 `NEXUS_NXS_RUNTIME_RELEASE_TOKEN` secret。临时关闭预置 runtime 可设置 `NEXUS_DESKTOP_BUNDLE_NXS_RUNTIME=0`，此时运行时必须通过 `NEXUS_NXS_COMMAND_PATH` 指向可执行的 `nxs`。

默认输出到 `desktop/macos/.build/package/`：

- `Nexus-macos-<version>-<build>.dmg`
- `Nexus-macos-<version>-<build>.dmg.sha256`
- `Nexus-macos-<version>-<build>.dmg.metadata.json`

安装前先校验 sha256：

```bash
cd desktop/macos/.build/package
shasum -a 256 -c Nexus-macos-<version>-<build>.dmg.sha256
```

打开 dmg 后，把 `Nexus.app` 拖到同一窗口里的 `Applications`。ad-hoc 本地测试包可能被 macOS 拦截首次打开；可信构建优先用 Finder 右键 Open。仅本地测试机器可在校验 sha256 后清理 quarantine：

```bash
xattr -dr com.apple.quarantine /Applications/Nexus.app
```

应用启动后会按 24 小时节流后台检测 GitHub Release 中的 macOS metadata；也可以从应用菜单选择“检查更新...”。只有 metadata 标记 macOS 包已 Developer ID 签名并公证时，Shell 才会提供自动下载安装：下载 `Nexus-macos-*.dmg` 或 zip 包及对应 sha256 到 `~/.nexus/cache/updates`，校验 sha256、Bundle Identifier、`codesign --verify --deep --strict` 与 `spctl --assess --type execute` 全部通过后，才提示退出、替换当前 `.app` 并自动重新打开。更新器不会自动移除 quarantine；如果当前 App 不在可替换位置，或者更新包未标记为可通过 Gatekeeper 自动安装，会退回打开下载页手动处理。

卸载或重置应用数据时，先退出 Nexus，再按需要删除：

- `/Applications/Nexus.app`
- `~/.nexus`

## 当前边界

- 还没有 Sparkle；内置自动更新器依赖 Release metadata、sha256、Developer ID 签名、公证和 Gatekeeper 本地校验。
- 还没有由 Go 协议真相源生成的 desktop bridge schema。
- 还没有更完整的快捷键冲突引导、逐项 secret 级 Keychain API、occlusion 长时间/异常路径验证和多窗口生命周期细化。
