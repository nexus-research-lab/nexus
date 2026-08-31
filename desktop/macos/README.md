# Nexus macOS Shell

这是 Nexus 桌面 App 的 macOS shell。

当前形态：

- SwiftPM 可执行程序，使用 AppKit + WKWebView。
- 开发模式下从仓库根目录启动 `go run ./cmd/nexus-server`；启动前会校验 `web/dist` 不早于 Web 源码、入口和构建配置，避免误跑旧前端。
- Bundle 模式下从 `.app/Contents/MacOS/nexus-server` 启动 Go sidecar，并优先使用 `.app/Contents/Resources/bin/nxs` 作为 `nxs` runtime。
- Shell 自动分配 loopback 随机端口。
- Sidecar 通过 `WEB_DIST_DIR` 托管 `web/dist`，WebView 访问同源 `http://127.0.0.1:<port>/`。
- Shell 在 document start 注入 `window.__NEXUS_DESKTOP_RUNTIME__`，前端优先使用注入的 API / WebSocket 地址。
- 桌面运行数据统一写入 `~/.nexus`；宿主日志写入 `~/.nexus/app/logs`，用户 runtime 数据写入 `~/.nexus/users/<owner>/runtime`。
- 设置页可以迁移完整状态根；确认后 shell 退出 sidecar、离线复制并直接重启。新实例健康后才删除旧根，启动失败会通过 `UserDefaults` 中的根指针自动回滚。
- Shell 会在 `~/.nexus/NexusSidecar.pid.json` 记录当前 sidecar；下次启动前会清理同 bundle 路径下的崩溃遗留进程。
- Shell 会把本地 session token 同步进 WKWebView cookie store，保证 WebSocket 握手也能通过本地 API 校验。
- Shell 在正式签名包中优先使用 macOS Keychain 持久化 connector credentials encryption key；开发模式和 ad-hoc 本地包默认直接使用 `~/.nexus/app/config/connector-credentials.key` 的 0600 本地密钥，避免反复重签后 Keychain ACL 弹密码或阻塞启动。sidecar 通过 `CONNECTOR_CREDENTIALS_KEY` 使用现有 Go 加密存储。
- Shell 负责单实例、Dock 重新打开、标准菜单、外链拦截和 `nexus://` URL scheme；冷启动和重复启动已有实例默认显示 launcher，Dock 重新打开只恢复现有主窗口，不主动改写当前路由。
- Shell 使用 `NSVisualEffectView` material 承载 WKWebView：主窗口使用 `windowBackground` material，WKWebView under-page 背景保持透明。
- 主窗口使用 titlebar-only 原生框保留 16pt 系统圆角，标准 traffic lights 在每次原生布局后对齐 24pt 中线，让 full-size Web Header 获得 48pt 高度；宿主把窗口按钮尾部安全区和红色按钮双轴中心注入 Web，使 NEXUS、Launcher 灯组、折叠动作和红灯共用水平中线且上下等距，并由 X 中心把侧栏 Dock 图标对齐红灯。Web 只同步窗口手势面与编辑控件矩形，`NSWindow` 用 AppKit 事件跟踪仲裁完整鼠标序列：4pt 内松手仍向 WKWebView 分发原始点击，越过阈值则把原始 mouse-down 交给系统窗口拖动，双击执行缩放。标签、按钮和菜单因此同时支持点击与按住拖窗，输入控件不被接管。
- 主窗口保持 `1280×820` 默认启动尺寸；常规屏幕可缩小到 `360×520`，极小可用工作区回退到 `320×480`，由 Web 层切换为手机布局。
- Shell 不再默认注册 `Option + Space` 全局唤起；窗口菜单仍保留“显示启动器”入口，设置页不再展示启动器快捷键配置。
- Shell 会按窗口职责加载 `app.html`、`settings.html`、`oauth-callback.html`，并用 `desktop_route` 把原始业务路由交给前端；`/launcher` 由主窗口 `app.html` 承载，sidecar 静态 fallback 支持直接刷新 `/launcher`、`/app`、`/settings` 和 OAuth callback。
- 最小 native bridge 已支持版本读取、状态根目录选择与完整迁移、外链打开、日志导出、主窗口路由打开和全局快捷键状态读写。
- 日志导出包会包含 `diagnostics.json`，记录版本、系统、bundle、runtime URL、关键目录和本地文件存在性；启动失败会在 `~/.nexus/app/logs` 写入 `startup-failure-*.json`。
- 启动、主窗口和更新失败统一说明“发生了什么、已有数据是否受影响、接下来能做什么”；底层错误、诊断路径和进程输出只进入日志或诊断报告，不直接显示给用户。更新检查与下载/校验失败按实际阶段分别说明，后者在安装程序启动前不会替换当前 App。
- Shell 会写 `[Nexus Startup]` 冷启动时间线，覆盖 sidecar、窗口、WebView navigation、Web ready 和 reveal；日志导出的 `diagnostics.json` 会带上 `startup_timeline`。
- 窗口遮挡、最小化和恢复事件会进入启动时间线；恢复探针受导航代次栅栏保护，不会在显式路由仍启动时二次 reload。
- WebView 内容进程终止时，Shell 会记录 `webview.content_process_terminated`、写入 `~/.nexus/app/logs/webcontent-terminated-*.json` 并 reload 当前路由，避免 WebContent crash 后停在空白窗口。
- Shell 会记录外链打开、未知 scheme 阻断和右键菜单抑制，便于桌面 QA 追踪 native 行为。
- 前端 ready signal 会带 source 和 performance marks；隐藏窗口 rAF 被节流时会用短 timer 兜底，避免主窗口等待 ready 时只能靠原生 fallback reveal。sidecar 会记录桌面 Web 静态资源请求摘要；两边都只记录 path 和 query key，不记录 OAuth code/state/token 等 query value。
- 首屏通过前端 ready signal 后再显示窗口，避免直接暴露 WebView 白屏。
- 桌面 OAuth 默认使用 `http://127.0.0.1:34343/capability/connectors/oauth/callback`，由本地 sidecar 接收 provider 回调；GitHub 在桌面包中走 Device Flow，只需要 `NEXUS_DESKTOP_GITHUB_CLIENT_ID` 注入公开 Client ID，不打包 Client Secret。

## 开发命令

```bash
scripts/desktop/build-macos-dev.sh
scripts/desktop/run-macos-dev.sh
swift test --package-path desktop/macos
swift scripts/desktop/generate-macos-icon.swift
scripts/desktop/build-macos-app.sh
scripts/desktop/run-macos-app.sh
scripts/desktop/smoke-macos-app.sh
scripts/desktop/package-macos-app.sh
```

`run-macos-dev.sh` 会先构建前端，再启动 Swift shell。直接运行 Swift shell 时若 `web/dist` 缺失或已过期，启动会明确失败并提示执行 `make app-run-dev`；正式 `.app` 内的打包资源不参与该开发态时效校验。首次启动会初始化桌面专用 SQLite 数据库。
`generate-macos-icon.swift` 会从 `desktop/macos/Resources/AppIconSource.png` 生成 `desktop/macos/Resources/AppIcon.icns`，用于 `.app` 的 Finder / Dock 图标。
`build-macos-app.sh` 会组装 `desktop/macos/.build/app/Nexus.app`，其中包含 Swift shell、Go sidecar、`web/dist`、`db/migrations` 与内置 `skills`。
`smoke-macos-app.sh` 会启动已组装 `.app`，校验 ad-hoc Keychain 旁路、主窗口默认 launcher ready reveal、显式 `/app` 路由 ready、material 标记和退出后 sidecar 无残留。
`package-macos-app.sh` 会先构建目标架构的 `.app`、下载并预置同架构的 `nxs` runtime、跑 smoke，再输出 zip/dmg、sha256 和 metadata。
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

`make app-dmg` 构建当前 Mac 的原生架构包；在 Apple Silicon 开发机上需要 Intel 包时运行：

```bash
make app-dmg-intel
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

`build-macos-app.sh` 在 Developer ID 签名时默认启用 hardened runtime 和 timestamp；`package-macos-app.sh` 会先校验 Swift shell、Go sidecar、`nexusctl`、`nxs` 与 `rg` 的目标架构，再提交 `.app` 公证并 staple，最后生成 dmg。dmg 默认也会提交并 staple；如果只想公证包内 `.app`，可设置 `NEXUS_DESKTOP_NOTARIZE_DMG=0`。如果用证书 SHA-1 而不是完整名称作为签名 identity，同时设置 `NEXUS_DESKTOP_CODESIGN_DEVELOPER_ID=1`。

GitHub Actions 的 `macOS Desktop Build` workflow 会分别在 Apple Silicon 与 Intel runner 上构建、smoke 并生成 ad-hoc dmg 验证打包路径，但不会使用 Apple 证书或上传 Release。`Publish Release` workflow 会为两个架构分别导入 Developer ID `.p12`，并把各自公证后的 dmg、sha256、metadata 上传到 Release。仓库需要配置：

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

- `Nexus-macos-arm64-<version>-<build>.dmg`
- `Nexus-macos-arm64-<version>-<build>.dmg.sha256`
- `Nexus-macos-arm64-<version>-<build>.dmg.metadata.json`
- `Nexus-macos-intel-<version>-<build>.dmg`
- `Nexus-macos-intel-<version>-<build>.dmg.sha256`
- `Nexus-macos-intel-<version>-<build>.dmg.metadata.json`

安装前先校验 sha256：

```bash
cd desktop/macos/.build/package
shasum -a 256 -c Nexus-macos-<architecture>-<version>-<build>.dmg.sha256
```

打开 dmg 后，把 `Nexus.app` 拖到同一窗口里的 `Applications`。ad-hoc 本地测试包可能被 macOS 拦截首次打开；可信构建优先用 Finder 右键 Open。仅本地测试机器可在校验 sha256 后清理 quarantine：

```bash
xattr -dr com.apple.quarantine /Applications/Nexus.app
```

应用启动后会检测一次 GitHub Release 中的 macOS metadata，并每 4 小时在后台复查；也可以从应用菜单选择“检查更新...”。仅桌面侧栏会在宿主确认有新版本时显示更新入口，点击后通过桌面桥直接启动原生下载流程。更新器会先按当前 CPU 架构选择 `arm64` 或 `intel` 资产，不会在两个安装包之间取任意一个。只有匹配架构的 metadata 标记 macOS 包已 Developer ID 签名并公证时，Shell 才会提供自动下载安装：下载对应 dmg 或 zip 包及 sha256 到 `~/.nexus/app/cache/updates`，校验 sha256、Bundle Identifier、`codesign --verify --deep --strict` 与 `spctl --assess --type execute` 全部通过后，才提示退出、替换当前 `.app` 并自动重新打开。新版本首次启动成功后会清理旧的更新缓存目录；用户选择“稍后”时，当前版本的已下载包会保留。更新器不会自动移除 quarantine；如果当前 App 不在可替换位置，或者更新包未标记为可通过 Gatekeeper 自动安装，会退回打开下载页手动处理。

卸载或重置应用数据时，先退出 Nexus，再按需要删除：

- `/Applications/Nexus.app`
- `~/.nexus`

## 当前边界

- 还没有 Sparkle；内置自动更新器依赖 Release metadata、sha256、Developer ID 签名、公证和 Gatekeeper 本地校验。
- 还没有由 Go 协议真相源生成的 desktop bridge schema。
- 还没有更完整的快捷键冲突引导、逐项 secret 级 Keychain API、occlusion 长时间/异常路径验证和多窗口生命周期细化。
