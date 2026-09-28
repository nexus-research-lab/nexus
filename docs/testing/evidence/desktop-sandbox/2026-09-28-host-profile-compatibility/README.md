# 宿主外层 Seatbelt 方案兼容性反例

状态：**non-normative / 候选已撤回，不能启用默认 App，2026-09-28**。

基线 Nexus `fe926790d`、Bridge `c994b197`；macOS 27.0 arm64。没有调用模型、App UI、Windows 或生产用户数据。

## 观测

1. 候选在可信 bootstrap exec 前安装外层 HostPolicy。独立原生子进程验证任务目录可写，宿主目录读写、符号链接别名读取、父目录重命名、包目录改写和宿主信号被拒绝；后代继承拒绝规则。
2. 同一测试中的不同内层 profile 启动失败：`sandbox-exec: sandbox_apply: Operation not permitted`，退出 71。见 [candidate-native.log](candidate-native.log)。该日志整体是失败结果，不能称为候选通过。
3. 独立最小复现排除了具体候选规则的影响：全允许外层/内层成功；有文件拒绝的外层阻止不同的全允许内层，也阻止额外增加拒绝规则的内层；相同 profile 的内层成功。见 [nesting.json](nesting.json)。不把这一观测泛化成所有版本或所有 profile 组合的保证。

## 对接入的影响

nxs 的命令执行在 `internal/tool/builtin/bash/sandboxexec/seatbelt.go` 生成具体工具 profile 并调用 sandbox-exec；Claude 原生命令沙箱也需要独立策略。不能把有意义的固定外层 Seatbelt 直接套在整个 SDK 进程外，并假设子进程仍能安装不同的命令策略。

候选生产改动已从 Bridge 撤回，Bridge 保持 `c994b197` 干净状态。候选源只以 `.go.txt` 保留在此目录用于审查，不参与构建。没有关闭既有后端沙箱、放宽策略或在 sandbox_apply 失败时回退裸执行。

## 复现

`python3 docs/testing/evidence/desktop-sandbox/2026-09-28-host-profile-compatibility/reproduce.py`

只使用自动回收的临时目录和系统 sandbox-exec/cat/true；实际状态码与当前记录不同时停止并要求重新评估。候选文件测试源码见 [host_policy_darwin_test.go.txt](host_policy_darwin_test.go.txt)，profile 生成与安装代码也保留为文本。

## 后续验收约束

宿主目录不可写仍是实际要求，不因候选失败而取消。下一方案须同时证明：

- nxs 与 Claude 各自的原生命令沙箱仍可运行；
- Full Access 不能改写宿主监督、数据库/恢复证据或可信 helper；
- 原生文件工具、命令、hook、外部 MCP/helper 及其他任务可执行入口不能绕过；
- 宿主 IPC/网络端点不能成为代写通道；
- 不凭同 UID 的 0700 权限、模型审批或路径字符串检查宣称 OS 隔离。

在这些条件得到真实双后端证据前，不启用依赖该目录保护合同的默认监督与自动恢复。按后端执行入口合并不可撤销宿主保护、或采用独立受信身份/服务，需要继续选择与验证；本记录不宣称其中任一路线已实现。

## 后续范围澄清（2026-09-28）

用户已明确选择 Full Access 不提供沙箱隔离保证。因此上文“Full Access 也不能改写宿主目录”是本次候选的历史前提，已被用户决定替代，不再作为当前交付门槛。外层嵌套失败的原生观测仍成立；后续按受限模式逐入口保护并验收，不通过额外身份改写 Full Access 语义。
