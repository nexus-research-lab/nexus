# Launcher

Launcher 只负责应用入口查询、最近会话导航和首屏视觉，不拥有 Room、Agent 或 Mention 的底层协议。

- `console/` 负责目录投影、查询命令、导航和页面装配。
- `hero/` 负责首屏视觉、输入交互和装饰动画。

Console 不持有视觉状态；Hero 不直接调用 API 或拼接 Room 路由。通用 Mention 规则归 `shared/ui/mention/`。
