# macOS 可信引导组件验收

2026-09-28，Bridge `861158e5c7f5c6ece527d2c20970e1516409bd05`，macOS 27.0 arm64。`productionIntegrated=false`，`releaseAccepted=false`。本批次不修改 Nexus 的 Bridge 固定依赖或发布包。

## 原生场景

[原生日志](native.jsonl)来自 `TestMacOSBootstrapExec`，构建真实 `cmd/nexus-runtime-bootstrap` 二进制并在独立 launchd job 中运行：

- helper 先按启动参数中的固定 audit identity 验证宿主连接。错误宿主身份及放行前断连都使真实 helper 以固定脱敏提示退出，不能执行任务。
- fixture 从本次唯一 job 的 `launchctl print` 获取 PID，再用连接内核 token 登记集合；测试登记落盘并同步文件/目录后才发送启动输入和三条标准管道。
- 任务使用 `/bin/sh`，验证原地 exec 保持 PID、stdin/stdout/stderr 分离、显式任务环境及宿主环境不继承、launchd 保留退出码 7。任务仍等待 stdin 时就确认控制连接 EOF，避免把进程退出冒充 CLOEXEC 证据。
- 任务参数和测试凭据不进入 job plist。收尾撤销 job 并等待原集合回收；不触碰已有 App 或会话。

[竞态检查](race.txt)覆盖身份编码、对端匹配、输入校验与真实 Unix socket/fd 传输。错误标记、额外 fd、方向错误、非管道 fd、输入截断/超限及未知字段均拒绝。批量传入 200 个 fd 后 stdout/stderr 管道仍可取得 EOF，证明被拒绝的写端未被保留。[无 cgo 构建](no-cgo.txt)通过；目标包 vet 通过。未做 Windows 验证或交叉编译。

## 发现并修复的失败

[最初失败](initial-descriptor-leak.txt)记录小 ancillary 缓冲区导致的描述符泄漏。macOS 在 `MSG_CTRUNC` 下可能保留原 `cmsg_len`；Go 通用解析器因此无法返回已安装的部分 fd。核对[固定 XNU 来源](source-index.json)后，接收缓冲扩大到覆盖 `UIPC_MAX_CMSG_FD=512`，再严格拒绝非三个 fd。内核控制 mbuf 另受 `MCLBYTES` 限制。若未来系统仍发生未知控制截断，helper 必须退出，不能复用该进程。最终失败路径用真实 EOF 检查替代仅检查错误值。

## 重放及边界

在 Bridge worktree：

```sh
GOWORK=off go vet ./internal/processscope ./internal/processbootstrap ./cmd/nexus-runtime-bootstrap
GOWORK=off go test -race -count=1 ./internal/processscope ./internal/processbootstrap ./cmd/nexus-runtime-bootstrap
CGO_ENABLED=0 GOWORK=off go test -count=1 ./internal/processscope ./internal/processbootstrap ./cmd/nexus-runtime-bootstrap
NEXUS_NATIVE_SCOPE_TEST=1 GOWORK=off go test -json -count=1 -timeout=60s ./internal/processbootstrap -run '^TestMacOSBootstrapExec$'
```

这里只完成引导接收端，不是产品端到端验收。fixture 的 job PID 解析和登记文件尚不是生产 launcher 或 owner/session/generation 持久层。宿主仍须实现可信 job/可执行文件核验、受启动 context 约束的发送、退出观察、撤销与清理回执、unknown 恢复及 scratch 回收。helper 尚未进入默认 transport 或正式 App 打包；macOS 14.0 精确信号缺口、Intel、签名、公证与 clean-host 验收继续待办。控制连接的双向身份比对不提供抵御任意未受限同 UID 程序的保证。
