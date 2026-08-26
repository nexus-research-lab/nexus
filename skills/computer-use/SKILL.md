---
name: computer-use
description: 通过 Nexus 宿主管理的独立 Computer Use Runtime 观察和操作当前 macOS 或 Windows 桌面应用。仅在用户已开启 Computer Use 且任务确实需要原生桌面交互时使用；网页 DOM、标签页、网络和后台页面继续使用 Browser。
---

# Nexus Computer Use

Computer Use 只负责原生桌面观察与动作。Nexus Agent 负责推理；宿主固定 owner、
physical round、sidecar、应用策略和动作权限。不要查找、输出或调用 `nexus-cua`
binary、endpoint、token file、artifact root 或 raw protocol。

## 入口

1. 先独立读取当前状态：

   ```bash
   "${NEXUS_COMMAND_PATH}" --json computer inspect
   ```

   PowerShell 使用 `& "${env:NEXUS_COMMAND_PATH}" ...`。如果返回 disabled、未安装、
   权限缺失或 runtime unavailable，向用户说明准确恢复动作；不要绕过开关、改用
   shell GUI 工具或自行启动 sidecar。
2. 每个新操作前读取 fresh exact contract：

   ```bash
   "${NEXUS_COMMAND_PATH}" --json computer contract --operation '<operation>'
   ```

   严格遵守返回的 `command_usage`、`contract` 和 `input_staging`。命令必须作为一个
   独立进程执行，不使用管道、重定向、`jq`、Python、正则或 shell 后处理。
3. 第一次 Write 前先 Read fresh `input_staging.path`，再覆盖为一个完整 closed JSON
   object。只使用 schema 暴露的业务字段；不要提交 owner、Agent、Session、round、
   endpoint、token、native handle、CUA session manifest 或宿主权限字段。
4. 同一语义重试复用 request ID；operation、target 或 input 改变时生成新 ID。

## 工作流

- 不知道目标应用时使用 `list_applications`，按返回的精确 name/application ID 选择；
  多个候选时不要猜。
- 使用 `select_target` 建立本轮唯一目标并取得第一份 observation。窗口选择不唯一、
  minimized、权限不足或 target unavailable 时按 typed recovery 处理。
- 在每个动作前确认使用的是最新 observation。element ref 和 screenshot point 只属于
  产生它们的 observation；动作后旧 ref 立即视为失效。
- `perform_action` 成功会区分 native dispatch 与 post-action observation。若结果为
  indeterminate，使用同一 request ID 重试同一 closed input；绝不能创建新动作来
  “再试一次”。
- 重要动作后使用 `verify_state` 或 fresh `observe` 验证真实桌面状态，不以工具返回
  success 代替结果验证。
- 完成后可调用 `close_target`；physical round 结束时宿主仍会自动清理。

## 路由边界

- Browser 已开启且目标是网页 DOM、标签页、下载、网络、控制台或后台页面时，使用
  Browser；不要用 Computer Use 模拟浏览器协议能力。
- Browser 未开启不影响 Computer Use 操作普通桌面应用。不要替用户自动开启 Browser。
- 文件内容优先用 Read/Write，终端任务优先用 Bash；只有真实 GUI 状态是必要输入或
  必须通过原生界面完成时才使用 Computer Use。
- 不操作登录/锁屏、系统安全界面、权限提示、UAC、安全桌面或未被 host policy
  明确允许的 system surface。

## 敏感数据

输入密码、token、验证码、私钥或其他秘密前必须确认当前操作确属用户请求且权限策略
允许。不要在回复、日志摘要、错误解释或后续命令中回显敏感文本。截图或可访问性树
包含敏感内容时，只描述完成任务所需的最小事实。
