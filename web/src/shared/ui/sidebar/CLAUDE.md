# Sidebar 空状态原语

本目录只保留可被不同业务侧栏复用的空状态。

- 展示原语不得读取应用路由、Sidebar Store、业务 API 或 Feature Tour。
- `sidebar-selection.ts` 只导出跨导航轨和目录行共用的中性浅灰选中态；侧栏消费者不得自行叠加蓝色、徽标、边框或浮起阴影，hover 也不得把活动项降回更弱层级。
- `sidebar-empty-guide.tsx` 只组合侧栏空状态与读取失败说明：所有文字使用 `caption` 的 tone/weight 层级，卡片使用共享 Surface 圆角，动作使用 `UiButton`；消费者不得通过 `className` 重写内部字号、形状或按钮状态。
- 业务导航项归所属 Feature；没有第二个真实消费者时不得在这里预留重命名、删除或外观变体。
- 应用宽侧栏归 `features/navigation/sidebar/`，引导定义归 `features/onboarding/`。
