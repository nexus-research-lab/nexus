# macOS 脱离后代监督候选：launchd coalition

2026-09-28。本机原生实验，不是生产接入或发布验收。三个场景均通过；`productionIntegrated=false`，`releaseAccepted=false`。Nexus 源码基线为 `44f043d6e`，没有修改 SDK、Bridge 或产品最低系统版本。

## 实测

| 场景 | 证据 |
| --- | --- |
| fork 后另建 session，父进程退出 | [plain.json](plain.json) |
| 两次 fork、另建 session、重新 exec，父与中间进程退出 | [double-fork-exec.json](double-fork-exec.json) |
| 在独立 Seatbelt 测试 profile 内重复两次 fork/exec | [sandbox-double-fork-exec.json](sandbox-double-fork-exec.json) |

各场景都以普通用户在自己的 GUI launchd domain 登记唯一、一次性测试 job。内核分配的 resource coalition 与观察者和对照进程不同；后代改变 Unix session、父进程退出及 exec 后仍属于该集合。测试使用固定 XNU 结构读取集合归属，通过 task name port 读取 audit token，并在归属读取前后核对身份。独立观察程序发现了脱离后代，不依靠 PPID/session 或后代主动上报作为选择依据；夹具的自报身份仅用于断言观察结果正确。

修改 audit token 的 PID version 后，发信号被内核拒绝（返回 `ESRCH`），目标仍存活；按观察到的精确身份发信号成功。随后对已经观察到的原 coalition 查询返回 `ESRCH`，独立对照进程的身份与存活状态保持。各次测试最终 job 均不存在，对照子进程已单独退出并被回收。

原生结果还确认：`LaunchOnlyOnce` 的 job 在根进程退出后已消失，`launchctl print`/`bootout` 返回未找到，但脱离后代仍存活。这是必须保留的反例，**job 消失不是完成清理的证明**。`tasks_started/tasks_exited` 只作诊断，不能用差值为零证明没有进行中的 fork；本实验等待的是原集合被回收后的 `ESRCH`。

## 重放

需要已登录的 macOS GUI 用户域、Command Line Tools 和当前原生 API。逐项运行：

```sh
python3 docs/testing/evidence/desktop-sandbox/2026-09-28-process-coalition/probe.py plain
python3 docs/testing/evidence/desktop-sandbox/2026-09-28-process-coalition/probe.py double-fork-exec
python3 docs/testing/evidence/desktop-sandbox/2026-09-28-process-coalition/probe.py sandbox-double-fork-exec
```

脚本在新临时目录编译 [probe.c](probe.c)，只注册自己的随机 job，并只向本次夹具的精确身份发信号；测试后检查 job 消失。后代另有自主退出期限。没有管理员权限请求、系统授权修改、其他 App 操作或 Windows 验证；不会读取 Provider 配置或发送模型请求。原始报告保存完整调用结果，缺 API、错误身份、未回收集合或对照进程受损都不能算通过。

## 部署限制

- 本机 OS、内核、架构与源码哈希在各报告内；这不是 macOS 14.0、Intel、签名 App 或干净机器结果。
- [固定来源索引](source-index.json)记录 Apple XNU 源码和哈希。`PROC_PIDCOALITIONINFO` 结构及 coalition usage 接口涉及私有 ABI；动态符号存在和一次实测不能替代跨支持版本验证。
- XNU `xnu-10002.1.13`（macOS 14.0 初始内核系列）没有 `PROC_INFO_CALL_SIGNAL_AUDITTOKEN`，`xnu-10002.61.3` 已包含它。产品当前 `.macOS(.v14)` 不变；不能据此将整条路线认定为所有受支持系统均可部署，也不能偷偷退回裸 PID 发信号。
- 原型没有持久化 boot identity、认证启动通道或执行前登记，没有接入 Nexus/Bridge 的启动/关闭、工具取消、崩溃恢复或 scratch 回收。它也未覆盖持续 fork、exec 竞争、拒绝观察、权限变化、控制通道丢失或恶意同 UID 进程。
- Seatbelt 场景使用的是有限目的的实验 profile，不是生产 nxs profile；不据此宣称产品沙箱行为已经通过。

后续实施约束见[监督接入方案](../../../../explorations/desktop-sandbox/macos-process-supervision.md)。[manifest.json](manifest.json)记录原始程序、驱动、结果与源码索引的 SHA-256；不包含编译二进制、用户数据库或凭据。
