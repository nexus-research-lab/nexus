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

2. 使用 `list_applications`，再用 `select_target` 以返回的精确应用名称或稳定 application id 选择目标，明确请求所需 allowed_actions、allow_foreground_input 和 ttl_seconds，由宿主确认后生效。参数上限读取 get_computer.action_limits/action_schema。

   调用格式为 `{"domain":"computer","action":"invoke","operation":"list_applications","input":{}}`。
   `operation` 只放顶层，`input` 只包含该操作 contract 声明的参数，不再嵌套 operation。

3. 多窗口应用先调用 `list_windows`，再把返回的 `window_ref` 或精确标题传给 `observe`；单窗口应用可直接 `observe`。
4. 每次动作必须使用最新观察；首次调用 `observe`，后续可使用动作返回的新 `observation`。截图或无障碍树过期后重新观察。需要同应用菜单与弹窗时，在能力支持下使用 `application_scene=true`。
   点击、移动、拖拽和滚动目标点均使用截图左上角为原点的 `0..1000` 归一化坐标；不要传图片像素。滚动必须给出目标 x、y，正向 delta_y 向下滚动。快捷键优先使用 `mod`（macOS Command，Windows Control）。
5. 动作使用 `perform_action`，把业务输入放在 contract 允许的 closed object 中；顶层生成新的 `request_id`。如果结果未知，复用同一个 request_id 重试以完成对账，不能换 ID 重放。
6. 动作返回 `observation` 时直接读取它；未返回时重新 `observe`。`observation_error` 只代表后续读取失败，不能据此重做动作。缓存结果标记 `fresh_observation_required` 时重新观察。需要确认业务状态时调用 `verify_state`。
   普通 `observe` 的 `observation_id` 和 `elements` 在结果顶层；`perform_action` 的新观察在 `observation` 内。截图是单独的图片内容，不要把它当作文本分页读取。
7. 任务结束调用 `close_target`，丢弃本轮的 session、observation、element 和 pending mutation 状态。

## 安全边界

- 只使用宿主返回的 opaque refs；不能提交 PID、窗口句柄、AX/UIA 对象、路径、socket、token 或任意 native handle。
- 输入、文本和截图遵守 contract 返回的长度与数量上限。不要把敏感文本写入普通回复或日志。
- 常规动作使用 select_target 已批准的会话范围；退出应用及宿主标记的敏感操作仍需单独审批。被拒绝或权限不足时停止并说明原因。审批后 stale 必须重新观察，不自动重放。
- 最小化窗口先用 `include_screenshot=false, accessibility="disabled"` 观察，再执行 `restore_window`，恢复后截图。
- `session_active=true` 仅表示应用授权仍在，不保证已有可用于动作的 observation。观察失败或尚未观察时，先修复观察，不要反复 `select_target`。
- macOS 后台窗口（包括台前调度缩略窗口）若返回 `target_unavailable` 且 `recovery_action="focus_window"`，对同一 `window_ref` 先做一次 `include_screenshot=false, accessibility="disabled"` 的元数据观察，再在已批准的范围内执行一次 `focus_window`，然后重新截图和读取无障碍树。禁止用未成功的观察直接执行动作；一次恢复仍失败就报告具体错误，不能反复聚焦或重新授权。此流程不解除 `user_active`。
- 应用未运行时先 list_launchable_applications，再 launch_application；启动许可不包含后续输入权限。
- `user_active` 表示用户接管：停止自动动作并说明，不因等待或重新观察而恢复，不自动换 session。用户明确要求继续后再走新的 select_target 授权。
- session 过期重新 select_target；回合授权上限到达后开始新回合，不静默续期。
- 每个 mutation 只对应一个稳定 request_id；目标、动作或输入改变时必须换 ID。
- 不要自动点击系统权限弹窗。Screen Capture、Accessibility 或 Input Control 未授予时，说明用户需要在系统设置中授权后再试。
