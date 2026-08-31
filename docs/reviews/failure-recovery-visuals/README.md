# Nexus 异常恢复样式审阅

这里保存本批次新增或统一后的异常恢复视觉模式。截图全部由真实生产组件、正式主题变量和中文文案渲染；数据均为合成内容，不连接后端，也不读取本机的 Agent、Session、任务、文件或密钥。

本轮参考 [Claude Desktop 的官方界面说明](https://academy.claude.com/tutorials/navigating-the-claude-desktop-app) 收紧了信息密度，但没有照搬其品牌。统一规则是：标题说明结果，一句自然语言同时说明数据状态和下一步，只有确实可执行时才显示动作。结果未知只表达一次“待确认”，正文只保留已知事实和重复操作的具体风险，不再罗列假设分支。明确失败使用红色；结果未知、旧内容或版本冲突使用低饱和提醒色。

## 快速查看

- [桌面端总览](./contact-sheet--desktop.png)
- [移动端总览](./contact-sheet--mobile.png)
- 单张原图：同一状态分别以 `desktop--*.png` 和 `mobile--*.png` 命名，共 22 张。

## 覆盖清单

| 状态 | 使用的生产视觉模式 | 重点检查 |
| --- | --- | --- |
| `feedback-not-applied` | `FeedbackBanner` 错误 | 明确未保存、已有数据未变、可安全修正后重试 |
| `feedback-accepted` | `FeedbackBanner` 信息 | 明确已接收但未完成，不重复启动 |
| `feedback-committed-refresh` | `FeedbackBanner` 警告 | 区分“已经保存”和“后续刷新失败” |
| `feedback-outcome-unknown` | `FeedbackBanner` 警告 | 不猜测结果，以只读核对替代普通重试 |
| `resource-load-failed` | `UiResourceState` 区域错误 | 无可靠内容时不伪装成空状态 |
| `resource-stale-snapshot` | `ReadResourceReliabilityNotice` | 明确正在显示上次成功快照，并提供重新检查 |
| `conversation-delivery-unknown` | `ConversationReliabilityNotice` | 固定在输入区附近，不写入消息历史 |
| `editor-conflict` | `TextFileEditorReliability` | 保留草稿，明确选择服务端版本或覆盖 |
| `editor-outcome-unknown` | `TextFileEditorReliability` | 只读核对保存结果，不自动再次写入 |
| `provider-persist-unknown` | `ProviderSetupFailureView` | 只停在保存阶段，不自动测试连接或修改默认模型 |
| `destructive-outcome-unknown` | `ConfirmDialog` 内联失败 | 保留删除上下文，阻止未知结果下重复删除 |

Agent、Automation、Channel、Provider、Workspace/Memory、Skill、Goal、Contacts、Auth、Connectors 与 Conversation 的具体文案会复用这些视觉模式；各领域的数据影响和安全动作仍由自己的状态机决定。macOS、Windows 与浏览器扩展本批次只统一了既有原生提示中“发生了什么、数据是否受影响、接下来做什么”的文案，没有新增另一套 Web 视觉组件，因此不重复伪造原生截图。

## 生成方式

审阅入口位于 `web/visual-review/failure-recovery/`，不在正式 App 路由或生产构建入口中。截图脚本位于 `web/scripts/capture-failure-recovery-visuals.py`，使用 Playwright 和 `webapp-testing` 的受控临时服务器流程：

```bash
cd web
python3 -m pip install playwright==1.55.0
python3 -m playwright install chromium
python3 /Users/berhand/.agents/skills/webapp-testing/scripts/with_server.py \
  --server "pnpm exec vite --host 127.0.0.1 --port 4173" --port 4173 -- \
  python3 scripts/capture-failure-recovery-visuals.py
```

生成时固定使用中文、浅色主题和减少动态效果。浏览器视口为桌面 `1180 × 820`、移动端 `390 × 844`；最终只截取真实验收区域，分别输出为 `780 × 520` 和 `366 × 720`，避免无关说明和大块空白干扰审阅。脚本会等待页面和字体完成，并把浏览器控制台错误视为失败。
