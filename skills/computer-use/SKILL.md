---
name: computer-use
description: 在用户明确要求时，通过 Nexus 内置 Computer Use 能力操作当前 macOS 或 Windows 桌面应用；遵守本轮权限、fresh observation 和 mutation 对账，不用于浏览器 DOM、普通文件或网络任务。
---

# Computer Use

当用户要求你查看或操作当前电脑上的原生桌面应用时使用本 Skill。浏览器网页优先使用 Browser 能力；普通文件读写使用文件或文档能力。不要为了绕过能力边界调用 shell、文件系统、浏览器调试协议或外部 MCP。

## 入口

Computer Use 只通过当前回合的 `nexus.command` 使用。先读取固定能力与权限：

```json
{"domain":"computer","action":"inspect"}
```

如果返回错误，说明能力未启用、宿主权限不足或 sidecar 不可用；把这个事实告诉用户，不要猜 endpoint、token、CLI 参数，也不要尝试重放动作。

## 标准流程

1. 需要新的操作时先读取精确 contract：

   ```json
   {"domain":"computer","action":"contract","operation":"<operation>"}
   ```

2. 使用 `list_applications`，再用 `select_target` 以返回的精确应用名称或稳定 application id 选择目标。
3. 每次动作前调用 `observe`，只使用该次返回的 window、element 和 observation ref。截图或无障碍树过期后重新观察。
4. 动作使用 `perform_action`，把业务输入放在 contract 允许的 closed object 中；顶层生成新的 `request_id`。如果结果未知，复用同一个 request_id 重试以完成对账，不能换 ID 重放。
5. 动作后重新 `observe`，需要确认状态时调用 `verify_state`。不要凭动作返回文本声称状态已改变。
6. 任务结束调用 `close_target`，丢弃本轮的 session、observation、element 和 pending mutation 状态。

## 安全边界

- 只使用宿主返回的 opaque refs；不能提交 PID、窗口句柄、AX/UIA 对象、路径、socket、token 或任意 native handle。
- 输入、文本和截图遵守 contract 返回的长度与数量上限。不要把敏感文本写入普通回复或日志。
- `perform_action` 属于真实桌面变更，等待当前回合的宿主审批；被拒绝或权限不足时停止并说明原因。
- 每个 mutation 只对应一个稳定 request_id；目标、动作或输入改变时必须换 ID。
- 不要自动点击系统权限弹窗。Screen Capture、Accessibility 或 Input Control 未授予时，说明用户需要在系统设置中授权后再试。
