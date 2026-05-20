# Nexus Windows Desktop

这是 Windows 原生壳的第一阶段骨架，目标是对齐 macOS dogfood app 的边界：原生 shell 负责窗口、WebView2、sidecar 生命周期和 runtime config 注入，业务 UI 继续复用 `web/dist`。

## 架构边界

- Native shell：C# + WPF，负责窗口、单实例、基础 `nexus://` 唤起和后续任务栏、系统菜单、通知、更新。
- WebView：WebView2，只作为 React/Vite UI 的渲染面。
- Sidecar：复用当前 Go `nexus-server`，由 shell 随机端口启动并注入 `NEXUS_DESKTOP_SESSION_TOKEN`。
- Web UI：复用 `web/dist/app.html`，默认路由为完整 launcher `/`。
- GitHub OAuth：桌面包只注入公开 `NEXUS_DESKTOP_GITHUB_CLIENT_ID`，WebView 内使用 Device Flow 授权码完成连接，不打包 Client Secret。

第一阶段已支持 Inno Setup 安装器、WebView2 Evergreen Runtime bootstrapper、可选 Authenticode 签名、portable zip 和启动后更新检测/下载校验；托盘和全局快捷键在后续阶段补齐。

## 构建

构建需要 Windows、.NET 8 SDK、Go、Node.js 和 pnpm。生成安装器还需要 Inno Setup 6.3+；安装器会内置 Microsoft Edge WebView2 Evergreen bootstrapper，便携 zip 仍要求目标机器已安装 WebView2 Runtime：

```powershell
winget install --id JRSoftware.InnoSetup -e
```

```powershell
pwsh scripts/desktop/build-windows-app.ps1
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

构建、烟测并生成 zip、安装器 exe、sha256 与 metadata：

```powershell
pwsh scripts/desktop/package-windows-app.ps1
```

package 脚本默认用 self-contained .NET 发布 shell，当前只构建 `win-x64`；安装器允许在 x64-compatible Windows 上运行，也就是 x64 Windows 和支持 x64 仿真的 Windows 11 ARM64。只需要 zip 便携包时可加 `-SkipInstaller`。

如需签名，配置以下环境变量后再运行 package 脚本；脚本会签 `Nexus.exe`、`Nexus.dll`、`Resources\nexus-server.exe` 和安装器：

```powershell
$env:NEXUS_WINDOWS_SIGNING_CERT_PFX_BASE64 = "<base64 pfx>"
$env:NEXUS_WINDOWS_SIGNING_CERT_PASSWORD = "<pfx password>"
$env:NEXUS_WINDOWS_TIMESTAMP_SERVER = "http://timestamp.digicert.com"
```

默认输出：

```text
desktop/windows/.build/package/Nexus-windows-<version>-<build>.zip
desktop/windows/.build/package/Nexus-windows-<version>-<build>.zip.sha256
desktop/windows/.build/package/Nexus-windows-<version>-<build>.zip.metadata.json
desktop/windows/.build/package/NexusSetup-<version>-<build>.exe
desktop/windows/.build/package/NexusSetup-<version>-<build>.exe.sha256
```

低层构建脚本也可以直接生成 zip 与 sha256：

```powershell
pwsh scripts/desktop/build-windows-app.ps1 -CreateArchive
```

默认输出：

```text
desktop/windows/.build/package/Nexus-windows-<version>-<build>.zip
desktop/windows/.build/package/Nexus-windows-<version>-<build>.zip.sha256
```

注册当前目录下的 `nexus://` 协议：

```powershell
pwsh desktop/windows/.build/app/Nexus/register-nexus-protocol.ps1
```

安装器会注册开始菜单快捷方式、可选桌面快捷方式和当前用户的 `nexus://` 协议；zip 便携包仍可手动运行上面的注册脚本。

## 当前边界

- 目前只在仓库内落了骨架；非 Windows 环境无法本地运行 WPF/WebView2。
- 桌面运行数据统一写入 `~/.nexus`，数据库位于 `~/.nexus/data/nexus.db`，日志位于 `~/.nexus/logs`。
- sidecar 凭据加密 key 优先使用 DPAPI current user 保护后保存到 `~/.nexus/config/connector-credentials.dpapi`，DPAPI 不可用时才降级到本地文件。
- 桥接接口先覆盖版本读取、外链打开、日志导出、主窗口路由打开和全局快捷键状态占位；日志导出会带 `diagnostics.json`，启动失败会写 `startup-failure-*.json`。
- 应用启动后会按 24 小时节流检测 GitHub Release 中的 Windows metadata；发现新版本时可下载 `NexusSetup-*.exe` 与对应 `.sha256` 到 `~/.nexus/cache/updates`，校验通过后提示是否退出 Nexus 并启动安装器。可设置 `NEXUS_DESKTOP_DISABLE_UPDATE_CHECK=1` 禁用检测。
- GitHub `Publish Release` workflow 会在 `windows-latest` 上构建、烟测并上传 Windows app zip、installer exe、sha256 与 metadata；未配置 Windows 签名证书时产物会明确标记为 unsigned。托盘在后续阶段补齐。
