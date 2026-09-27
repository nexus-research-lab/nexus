# Windows P3：本机组合结果与下一步决策

2026-09-28，Windows 11 Home 10.0.26200 amd64，普通宿主账号，独立临时普通执行账号。
状态：架构验证中，P3/P4 尚未验收，Windows 后端和分项 capability 继续 fail closed。
本文补充 development-plan 的固定实验记录，不替代当前规范。

## 同一套断言的实测结果

原完整限制 token（A）保留为负面基线：PowerShell 为 `0xc0000135`，普通受限后代
CreateProcess 拒绝访问；同账号普通 PowerShell 正向对照通过。

LogonW 每个场景创建全新 runner，内部命令在 Job 属性中原子创建且初始悬挂，完成
runner 保护后恢复。任务不继承临时账号密码。以下各列执行完全相同的十项断言：

| 场景 | reference：capability/User/logon/World | capability | capability＋logon |
| --- | --- | --- | --- |
| PowerShell 固定退出 37 | 通过 | CLR 失败 | CLR 失败 |
| 普通 Go 后代、Job 继承、breakaway 拒绝 | 通过 | 通过 | 通过 |
| Local 命名事件创建、重开、修改 | 通过 | 拒绝访问 | 通过 |
| runner process/token 控制权限逐项拒绝 | 通过 | 通过 | 通过 |
| runner 既有线程控制权限拒绝 | 通过 | 通过 | 通过 |
| runner 未来线程控制权限拒绝 | 通过 | 通过 | 通过 |
| host process/token 控制权限逐项拒绝 | 通过 | 通过 | 通过 |
| host 活线程控制权限拒绝 | 通过 | 通过 | 通过 |
| 排除继承句柄，不改变父进程哨兵 | 通过 | 通过 | 通过 |
| scratch 可写、同账号未授权目录不可写 | **失败：获得越界写句柄** | 通过 | 通过 |

没有一列完整通过。reference 的九项兼容/控制成功不能掩盖最后一项文件边界失败。
哨兵只尝试打开写句柄并立即关闭，未修改根外用户文件。外层测试现在要求指定内层
测试确实 PASS，空筛选的退出零、skip、缺少结果均不能作为证明。

## CLR 失败的具体对象

普通对照得到 PowerShell `0xffff0000`，消息为 CLR 初始化 HRESULT `80070005`。
只对固定系统 PowerShell 的 `exit 37` 进程做本机定向调试，观察到唯一的拒绝调用：

```text
NtCreateSection -> STATUS_ACCESS_DENIED (0xc0000022)
name: Global\Cor_Private_IPCBlock_v4_<process-id>
requested access: 0x000f0007
explicit DACL: Administrators + execution-account SID
```

该显式 DACL 未包含 capability 或 logon SID。调试器只观察固定子进程的 NT 调用，
保持返回值；没有改系统 ACL、扩大 token 或把调试模式当作通过。非调试十项矩阵仍
重现同一 CLR 失败。原始定向日志 SHA-256：
`519dc3ea59082060b1a72f922846108e1016e087a2c4b4ff8573eb028e510581`。

因此，本机证据指向 CLR 自建 IPC 对象与当前 restricting SID 组合的冲突。只改
PowerShell profile、工作目录或给整个系统目录增权并不解决这个已观察到的访问检查。
恢复 User SID 虽可兼容 CLR，却已由文件反例证明会丢失独立 capability 写范围。
这不是对完整 Codex 产品的漏洞判定；这里没有复现其完整文件/网络 provisioning。

## 决策和停止条件

1. 不把任何当前 reference token 变体接入生产；保留原拒绝断言与失败日志。
2. 不通过放开 User/World、关闭线程检查或修改整棵系统 ACL 获得“通过”。
3. 下一轮只验证一个新假设：Windows AppContainer 的独立对象命名空间与双主体访问
   检查，能否同时支持 CLR 和精确文件授权。先限于临时账号、fixture 目录和固定命令，
   保留上述十项断言，并增加实际 AppContainer SID、网络默认拒绝、子孙继承检查。
   不以支持 API 或成功启动代替整个候选验收。
4. 仅当新候选同时通过上述检查，才选择为 P4 后端并设计独立资源租约、provisioning
   与安装信任；稳定失败先记录具体对象/权限，再形成 ADR，不增加无假设的组合。
5. 外层 LogonW runner 的创建和 Job 绑定仍是两步；创建间隙崩溃、完整网络策略、真实
   文件/搜索/媒体等执行接线、取消与 unknown 恢复、安装/修复/升级均未被本轮替代。

新假设依据为微软的 [AppContainer 启动与访问模型](https://learn.microsoft.com/en-us/windows/win32/secauthz/implementing-an-appcontainer)。
它是待本机验证的候选，不代表 Nexus 已采用 AppContainer 或已经获得完整隔离。

## 可复现测试入口

`scripts/desktop/prepare-windows-sandbox-fixture.ps1` 仅用于测试，由同一原 owner 提升
权限创建 GUID 专属普通账号、受限目录 ACL 和 owner DPAPI 加密凭据。它不是产品
安装器，不支持另一管理员代输作为原 owner。Remove 核验账号 SID/描述与存活进程，
再清理已卸载的该账号 profile、账号和精确 fixture；不会触及用户工作区。

普通宿主通过 `scripts/desktop/check-windows-sandbox-logon.ps1` 从本地 SDK 编译测试，
核验 fixture 二进制摘要并跑三列十项断言，记录源码 SHA、dirty 状态、二进制/log
摘要和真实退出码。报告始终不授予发布验收；任一失败、skip 或结果缺失返回非零。

当前十项比较会返回 1，原因如上表，不能把脚本成功生成报告当作沙箱通过。
