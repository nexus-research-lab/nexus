# Nexus Windows Desktop

这是 Windows 原生壳的第一阶段骨架，目标是对齐 macOS dogfood app 的边界：原生 shell 负责窗口、WebView2、sidecar 生命周期和 runtime config 注入，业务 UI 继续复用 `web/dist`。

## 架构边界

- Native shell：C# + WPF，负责窗口、单实例、基础 `nexus://` 唤起和后续任务栏、系统菜单、通知、更新。
- WebView：WebView2，只作为 React/Vite UI 的渲染面。
- Sidecar：复用当前 Go `nexus-server`，由 shell 随机端口启动并注入 `NEXUS_DESKTOP_SESSION_TOKEN`。
- Web UI：复用 `web/dist/app.html`，默认路由为完整 launcher `/`。

第一阶段暂不做安装器、托盘、全局快捷键和自动更新。

## 构建

需要 Windows、.NET 8 SDK、WebView2 Runtime、Go、Node.js 和 pnpm。

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

生成 zip 与 sha256：

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

## 当前边界

- 目前只在仓库内落了骨架；非 Windows 环境无法本地运行 WPF/WebView2。
- sidecar 凭据加密 key 优先使用 DPAPI current user 保护后保存到 `%LOCALAPPDATA%\Nexus\config\connector-credentials.dpapi`，DPAPI 不可用时才降级到本地文件。
- 桥接接口先覆盖版本读取、外链打开、日志导出、主窗口路由打开和全局快捷键状态占位；安装器、托盘和自动更新在后续阶段补齐。
