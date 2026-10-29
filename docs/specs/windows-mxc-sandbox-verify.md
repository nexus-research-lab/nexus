# Windows Rust MXC 验证清单

当前 SDK 使用 `native/nxs-mxc` 的 Rust helper，微软库固定提交
`ad54c69452147bd4ca15d027388ad6074903dbff`。SDK commit 以 Nexus 两个 workflow
和 `scripts/desktop/check-windows-sandbox.mjs` 的一致 pin 为准。Bridge 保持现有
远程 pin：它使用显式 nxs 路径，不下载或管理 helper。

## 构建与原生验收

构建机安装 Rust/MSVC 工具链，在 SDK 根目录、普通用户 PowerShell 执行：

```powershell
$env:GOWORK='off'
./native/nxs-mxc/build.ps1 -Architecture amd64 -OutputDirectory dist
if ($LASTEXITCODE -ne 0) { throw 'Rust helper build failed' }
go build -o dist/nxs.exe ./cmd/nxs
if ($LASTEXITCODE -ne 0) { throw 'nxs build failed' }
./dist/nxs-mxc.exe --probe
$env:NEXUS_NXS_EXECUTABLE=(Resolve-Path dist/nxs.exe).Path
$env:NEXUS_REQUIRE_NATIVE_MXC='1'
go test -count=1 -v -timeout=3m -run '^(TestWindowsMXC|TestMXC)' ./internal/tool/builtin/bash/sandboxexec ./internal/tool/builtin/file/sandboxfs ./internal/tool/executor
```

不得将 skip 计为通过。必须覆盖 PowerShell、读写/拒绝授权、域名代理和网络拒绝、
文件 helper、MCP stdio、宿主 ACL 不变、取消和前台退出后的后代清理。
文件测试直接操作文件并检查实际结果；树测试等待子进程就绪后检查进程句柄，
不通过 ping 或脚本标记猜测。失败时保留完整日志和 SDK commit。

## Nexus 打包与退出

本地 source build 的 `NEXUS_DESKTOP_NXS_RUNTIME_PATH` 指向上面 dist/nxs.exe，
同目录必须有 nxs-mxc.exe、nxs-mxc-LICENSE.txt 和 rg.exe。发布 workflow 会构建
固定 SDK 中的 Rust helper，签名与安装器元数据覆盖它；下载 runtime 的 archive
包含这些组件，校验 SHA256 后提取。`--skip-mxc` 仅用于 workflow 下载旧 runtime
以取得 rg、随后立即用源码构建替换 runtime 的中间步骤，不能用于最终包。

原生 SDK 全部通过后，再验证窗口 × 退出、立即重开、重新打包后打开；检查无残留
后代、无端口/单实例锁冲突。运行 Windows shell lifecycle 测试和安装器合约检查。
交叉编译、probe、文件存在或签名均不替代 enforcement/关闭重开验收。

## 当前证据边界

用户报告 Go SDK 5b49bd45 的 PSEC Create/Close 全部通过，之后执行矩阵失败。
因此不能继续称系统 PSEC API 不可用；此前 GitHub #53650 的 0x80070005 也不能
解释该机器旧的 0x80070032。Rust 替换减少自研底层代码；原生 SDK、完整应用启动和真实会话初始化必须分别验收。
