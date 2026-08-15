# Nexus Windows Desktop

这是 Windows 原生壳的第一阶段骨架，目标是对齐 macOS dogfood app 的边界：原生 shell 负责窗口、WebView2、sidecar 生命周期和 runtime config 注入，业务 UI 继续复用 `web/dist`。

## 架构边界

- Native shell：C# + WPF，负责窗口、单实例、基础 `nexus://` 唤起和后续任务栏、系统菜单、通知、更新。
- WebView：WebView2，只作为 React/Vite UI 的渲染面。
- 窗口 chrome：WPF `WindowChrome` 保留四边缩放；独立 34-DIP 原生栏承载应用图标、前进/后退、文件/编辑/视图/帮助菜单和最小化、最大化/还原、关闭控件，且只有该栏空白区参与拖窗。`WebView2CompositionControl` 从下一行完整铺开并始终保持客户区，Web Header、搜索区和内容不再投影为非客户区；`/app` 无空白 Header、透明拖动条或 caption controls 留白。
- 原生反馈：菜单、更新和启动错误统一使用 Nexus 原生主题与模态对话框；系统 `MessageBox`、菜单默认模板和独立更新窗口不得形成第二套视觉语言。
- Sidecar：复用当前 Go `nexus-server`，由 shell 随机端口启动并注入 `NEXUS_DESKTOP_SESSION_TOKEN`，正式包优先使用 `Resources\bin\nxs.exe` 作为 `nxs` runtime。
- Web UI：复用 `web/dist/app.html`，默认路由为完整 launcher `/launcher`。
- 主窗口保持 `1280×820` 默认启动尺寸；常规屏幕可缩小到 `360×520`，极小可用工作区回退到 `320×480`，由 Web 层切换为手机布局。
- GitHub OAuth：桌面包只注入公开 `NEXUS_DESKTOP_GITHUB_CLIENT_ID`，WebView 内使用 Device Flow 授权码完成连接，不打包 Client Secret。

第一阶段已支持 Inno Setup 安装器、WebView2 Evergreen Runtime bootstrapper、可选 Authenticode 签名和启动后更新检测/下载校验。覆盖安装前，安装器会通过单实例退出协议结束仍驻留托盘的 Nexus，并等待可执行文件解除占用。

## 构建

构建需要 Windows、.NET 8 SDK、Go、Node.js 和 pnpm。生成安装器还需要 Inno Setup 6.3+；安装器会内置 Microsoft Edge WebView2 Evergreen bootstrapper：

```powershell
winget install --id JRSoftware.InnoSetup -e
```

```powershell
pwsh scripts/desktop/build-windows-app.ps1
```

也可以通过 Makefile 构建或直接运行临时测试包；Makefile 目标默认会预置 `nxs` runtime，避免 Agent 启动时找不到 `nxs.exe`：

```powershell
make app-win-build
make app-win-run
make app-win-run APP_WIN_RUN_SKIP_BUILD=1
```

如需临时关闭随包预置 runtime：

```powershell
make app-win-build APP_WIN_BUNDLE_NXS_RUNTIME=0
```

默认输出：

```text
desktop/windows/.build/app/Nexus/
```

启动：

```powershell
desktop/windows/.build/app/Nexus/Nexus.exe
```

烟测已组装 app：

```powershell
pwsh scripts/desktop/smoke-windows-app.ps1
```

构建、烟测并生成安装器 exe、sha256 与 metadata：

```powershell
pwsh scripts/desktop/package-windows-app.ps1
```

package 脚本默认从 bridge runtime release 的 `nxs-stable` 通道下载并预置当前平台的 `nxs` runtime。可通过 `NEXUS_DESKTOP_NXS_RELEASE` 固定到某个 `nxs-v*` 版本。如目标 release 不是公开可匿名下载，需配置 `NEXUS_DESKTOP_NXS_DOWNLOAD_TOKEN`，或在 GitHub Actions 中配置 `NEXUS_NXS_RUNTIME_RELEASE_TOKEN` secret。临时关闭预置 runtime 可设置 `NEXUS_DESKTOP_BUNDLE_NXS_RUNTIME=0`。

package 脚本默认用 self-contained .NET 发布 shell，当前只构建 `win-x64`；安装器允许在 x64-compatible Windows 上运行，也就是 x64 Windows 和支持 x64 仿真的 Windows 11 ARM64。

如需签名，配置以下环境变量后再运行 package 脚本；脚本会签 `Nexus.exe`、`Nexus.dll`、`Resources\nexus-server.exe`、`Resources\bin\nexusctl.exe`、`Resources\bin\nexuscfg.exe`、`Resources\bin\nxs.exe` 和安装器：

```powershell
$env:NEXUS_WINDOWS_SIGNING_CERT_PFX_BASE64 = "<base64 pfx>"
$env:NEXUS_WINDOWS_SIGNING_CERT_PASSWORD = "<pfx password>"
$env:NEXUS_WINDOWS_TIMESTAMP_SERVER = "http://timestamp.digicert.com"
```

默认输出：

```text
desktop/windows/.build/package/Nexus-windows-<version>-<build>.metadata.json
desktop/windows/.build/package/NexusSetup-<version>-<build>.exe
desktop/windows/.build/package/NexusSetup-<version>-<build>.exe.sha256
```

注册当前目录下的 `nexus://` 协议：

```powershell
pwsh desktop/windows/.build/app/Nexus/register-nexus-protocol.ps1
```

安装器会注册开始菜单快捷方式、可选桌面快捷方式和当前用户的 `nexus://` 协议；本地 build 目录仍可手动运行上面的注册脚本。

## 当前边界

- 目前只在仓库内落了骨架；非 Windows 环境无法本地运行 WPF/WebView2。
- 桌面运行数据统一写入 `~/.nexus`，数据库位于 `~/.nexus/app/data/nexus.db`，日志位于 `~/.nexus/app/logs`。
- 设置页可以迁移完整状态根；确认后 shell 退出 sidecar、离线复制并直接重启。新实例健康后才删除旧根，启动失败会通过当前用户注册表中的根指针自动回滚。
- sidecar 凭据加密 key 优先使用 DPAPI current user 保护后保存到 `~/.nexus/app/config/connector-credentials.dpapi`，DPAPI 不可用时才降级到本地文件。
- 桥接接口覆盖版本读取、状态根目录选择与完整迁移、外链打开、日志导出、主窗口路由打开和全局快捷键状态占位；日志导出会带 `diagnostics.json`，启动失败会写 `startup-failure-*.json`。
- 应用启动后会检测一次 GitHub Release 中的 Windows metadata，并每 4 小时在后台复查；仅桌面侧栏会在宿主确认有新版本时显示更新入口，点击后通过桌面桥直接下载 `NexusSetup-*.exe` 与对应 `.sha256` 到 `~/.nexus/app/cache/updates`，校验通过后提示是否退出 Nexus 并启动安装器。新版本首次启动成功后会清理旧的更新缓存目录；用户选择“稍后”时，当前版本的已下载包会保留。可设置 `NEXUS_DESKTOP_DISABLE_UPDATE_CHECK=1` 禁用检测。
- GitHub `Publish Release` workflow 会在 `windows-latest` 上构建、烟测并上传 Windows installer exe、sha256 与 metadata；未配置 Windows 签名证书时产物会明确标记为 unsigned。托盘在后续阶段补齐。
