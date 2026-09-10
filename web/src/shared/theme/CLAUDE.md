# Theme

- ThemeProvider 应用主题与聊天排版；Context 定义主题身份及 CSS 投影，排版存储由 chat-typography 独立持有。
- ThemeOverlay 仅拥有主题专用装饰。系统减少动态效果时不挂载视频或 Canvas，雾层保持静态；Canvas context 不可用不能使应用失败，卸载清理 RAF、resize 与视频播放。
- 装饰图形的粒子、色彩和合成保持主题私有，不上升为交互组件。

- theme-storage 只保护可选本机视觉偏好；拒绝存储时仍应用实时主题/排版，不把它用作业务写入成功证据。装饰层使用 --layer-theme-decoration，低于菜单/反馈，按DOM顺序叠加雾与雨。
