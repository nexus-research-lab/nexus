# Desktop App QA Checklist

本文档用于记录 macOS app 包的人工验收步骤。自动化 smoke 负责确认 `.app` 能启动、sidecar 能监听、默认 launcher 和显式主窗口都能 reveal；这里覆盖更接近真实桌面使用的输入、导航、OAuth 和诊断行为。

## 1. 前置条件

1. 从 GitHub workflow artifact 或 Release asset 下载 `Nexus-macos-<version>-<build>.dmg` 与对应 `.sha256`。
2. 在同一目录执行：

```bash
shasum -a 256 -c Nexus-macos-<version>-<build>.dmg.sha256
```

3. 打开 dmg 后把 `Nexus.app` 拖到 `/Applications`，首次启动用 Finder 右键 Open。
4. 记录测试机器信息：macOS 版本、芯片、输入法、是否外接屏、包版本、build number。

## 2. 桌面交互

| 场景 | 操作 | 期望 |
| --- | --- | --- |
| 默认入口 | 打开 `Nexus.app` | 首屏不出现长期白屏或黑屏；默认显示 launcher，不直接进入 `/app` 工作台。 |
| Dock reopen | 关闭窗口后点击 Dock 图标 | launcher 重新出现，sidecar 不重启。 |
| Cmd+W | 主窗口按 `Command+W` | 只隐藏窗口，不退出应用。 |
| Cmd+Q | 按 `Command+Q` | App 退出，`nexus-server` 无残留。 |
| Launcher | 窗口菜单选择“显示启动器”，或执行 `open nexus://launcher` | 主窗口显示完整 launcher 首页，不出现紧凑浮层。 |
| 检查更新 | 应用菜单选择“检查更新...” | 弹出原生检查结果；若有新版本，展示当前版本、最新版本和打开下载页按钮。 |
| 全局快捷键 | 首次安装后按 `Option+Space` | 不应唤起 Nexus；设置页不展示启动器快捷键配置。 |
| URL 唤起 | 执行 `open nexus://open` | 主窗口显示完整 launcher 首页。 |
| 进入工作台 | 在 launcher 选择进入工作台 | 主窗口导航到 `/app` 工作台。 |
| 设置页返回 | 从菜单或 `nexus://settings` 进入设置后点击“返回工作台” | 回到主工作区，视觉上有明确返回路径。 |
| 文本输入 | 在设置页、搜索框、会话输入框输入英文、中文拼音、标点和换行 | IME 候选窗位置正常，提交文字不丢字符。 |
| 键盘导航 | 使用 `Tab` / `Shift+Tab` 在表单和按钮之间移动 | 焦点顺序可解释，焦点不会丢到不可见区域。 |
| 编辑快捷键 | `Command+A/C/X/V/Z/Shift+Command+Z` | 文本选择、复制、剪切、粘贴、撤销和重做符合 macOS 预期。 |
| 右键菜单 | 在页面空白、输入框、链接、代码块上右键 | 不出现浏览器默认菜单；输入场景不影响系统输入。 |
| 滚动 | 触控板惯性滚动、鼠标滚轮、长列表滚动 | 滚动方向、速度和边界回弹正常。 |
| 外链 | 点击 `https` / `mailto` 外链 | 交给系统默认应用打开，不在 WKWebView 内跳走。 |
| 未知 scheme | 触发非允许 scheme | 被阻断，应用不崩溃，日志有 `webview.navigation_blocked`。 |

## 3. OAuth 回调

1. 在 provider 后台登记 `nexus://connectors/oauth/callback`。
2. 在 Nexus Connectors 页面发起授权。
3. 系统浏览器打开 provider 授权页。
4. 授权后回到 Nexus App 的 OAuth callback 页面。
5. 连接状态变为已连接。
6. 日志中只允许出现 callback path 和 query key，不应记录 OAuth `code`、`state`、token 或 secret value。
7. 如果 provider 不支持 custom scheme，记录 provider 名称和限制，不把该 provider 标记为桌面 OAuth Green。

## 4. 诊断与反馈

| 场景 | 操作 | 期望 |
| --- | --- | --- |
| 手动导出日志 | 设置页触发日志导出 | zip 内包含 `diagnostics.json` 和 `Logs/`。 |
| 启动失败 | 临时破坏 bundle 资源后启动 | `~/.nexus/logs/startup-failure-*.json` 存在，错误弹窗提示路径。 |
| WebContent 终止 | 若出现 WebContent crash 或系统杀进程 | 自动 reload 当前 route，并写入 `webcontent-terminated-*.json`。 |
| 外链/阻断排查 | 重测外链和未知 scheme | `Nexus Startup` 时间线包含 external open / blocked navigation 事件。 |
| 更新检测排查 | 重测启动后自动检测和菜单手动检测 | `Nexus Startup` 时间线包含 `update_check.started` 以及 `update_check.result` 或 `update_check.failed`。 |

## 5. 记录格式

每轮 macOS app QA 用以下格式记录到 issue、PR 或发布检查单：

```text
Nexus macOS app QA
Version:
Build:
Commit:
macOS:
Hardware:
Input methods:
External displays:

Passed:
- ...

Failed:
- [场景] 复现步骤 / 期望 / 实际 / 日志路径

Artifacts:
- app dmg:
- sha256:
- diagnostics zip:
```
