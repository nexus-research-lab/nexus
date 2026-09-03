# Shared UI foundation

本目录只保存不认识 Agent、Room、Goal 等业务名词的 UI 基础设施。业务语义留在 `features/`，页面组合留在 `pages/`；共享层不得向上导入它们。

| 基础合同 | 唯一所有者 | 行为测试 |
| --- | --- | --- |
| 文字/图标动作 | `button/`、`list/` | 默认 type、disabled、键盘、冒泡 |
| 输入与选择 | `form/` | 校验、ARIA、布尔与互斥选择 |
| 模态与决策 | `dialog/` | 栈、焦点圈、Escape、遮罩、滚动锁 |
| 锚定浮层 | `overlay/`、`menu/` | Portal、定位生命周期、焦点、键盘、关闭 |
| 页面内切换 | `navigation/` | pressed/selection 与独立关闭动作 |
| 内容表面 | `panel.tsx` | 有限 padding/radius/variant；默认无阴影 |
| 状态与标记 | `display/` | live region、忙碌、动作、计数边界 |

- 公共视觉 API 只暴露有限的 `size / tone / variant / density / elevation / layer / viewport`；`className` 只用于外部布局，不用于覆盖内部颜色、圆角、阴影、层级、hover 或 focus。
- 消费者直接导入职责文件；本目录不提供聚合导出，根目录只保留无法归入具体交互职责的基础原语。
- `UiPanel` 只有 `card / dashed / plain` 三种真实差异。不要用新名字复制同一组 class；需要业务状态表面时组合 `UiResourceState`，需要交互时使用 Button/List/Menu 等对应 primitive。
- `dialog/` 统一模态栈、滚动锁、键盘与焦点协议，业务弹窗只组合结构和内容；通用决策框不得自动补风险套话、装饰警告图标或消息卡片。连接、授权与紧凑表单统一使用 `UiDialogHeader/Footer appearance="plain"`，只保留标题、必要上下文和动作。
- `icon-picker/` 的锚定浮层只显示可访问名称和图标网格；`maxIcons` 是数据边界，不作为孤立数字显示在标题旁。
- `liquid-glass/` 分离浏览器能力、交互/动画生命周期、滤镜资源链与组件装配；视图渲染阶段不得修正自身状态。
- 新增 primitive 前先搜索本表所有者；DOM、键盘、焦点或 ARIA 相同就扩展既有组件。只有跨三个独立消费者且语义稳定的组合才晋升为共享 pattern。
- `frontend-foundation-contract.test.mjs` 固定关键行为测试清单；新增基础行为必须共置 `*.test.tsx`，源码正则不能代替 DOM 测试。
