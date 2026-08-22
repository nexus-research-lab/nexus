# Launcher

Launcher 只负责应用入口查询、最近会话导航和首屏视觉，不拥有 Room、Agent 或 Mention 的底层协议。

- `console/` 负责目录投影、查询命令、导航和页面装配。
- `hero/` 负责首屏视觉、输入交互和装饰动画。

Console 不持有视觉状态；Hero 不直接调用 API 或拼接 Room 路由。通用 Mention 规则归 `shared/ui/mention/`。

Launcher 只把顶部双层品牌 Header 声明为桌面窗口手势面：第一行承载系统窗口控制与横向灯组，第二行承载品牌图标和字标。Hero、输入和 Agent Pile 均保持内容交互语义，不得把页面根画布扩成拖动层。

Launcher 页面只消费 `home-directory-resource.ts` 的共享目录快照，不得单独请求 bootstrap 或重复订阅目录失效事件。

`/launcher?initial=...` 只预填 Launcher 输入，供桌面深链和 Browser 上下文交接使用；不得自动选择 Agent 或发送消息。
