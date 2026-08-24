# session-navigator/

L4 | 父级: web/src/features/conversation/shared

## 职责

- `conversation-session-navigator.tsx`: 会话刻度与非模态轻量预览卡渲染；预览不得复用 Dialog Header 或制造装饰图标层级
- `session-navigator-model.ts`: 先把用户、Assistant 和结果摘要解析为轮次快照，再将已加载快照或索引记录按当前语言投影为导航项；固定状态、回退标题和摘要不得写死在视图中
- `session-navigator-ruler-model.ts`: 刻度尺寸、波形、颜色分段和讲者文案的纯视觉模型
- `navigation-dom.ts`: 将可见轮元素投影为带焦点距离的候选，再按包含焦点、最近距离顺序选择，不持有 React 状态
- `use-active-round.ts`: 当前可见轮同步和用户滚动中断
- `jump/`: 分离作用域化目标、缺失窗口加载队列、逐帧落点确认和跳转入口
- `use-conversation-session-navigation.ts`: 只组合展示状态和两个控制器

导航目标以滚动容器的 `data-conversation-round-navigation-target` 为唯一真相，
不要再增加平行 ref 或局部状态镜像。

跳转事务必须绑定 `scopeKey`、索引中的 exact round identity 和请求代次。旧会话请求不得释放新会话锁、
清除新导航目标或修改当前活动轮；加载与落点修正共用一次 `auto` 滚动事务，不得让浏览器 smooth 动画跨越异步提交。只有目标正文已驻留且 DOM 确认可见后才能提交活动轮并释放导航所有权，失效和失败的导航目标也必须显式释放。

导航项中的 Agent ID 在数据投影阶段完成去空、去重；视觉模型只消费该不变量，
不得再次清洗数据或读取原始消息。
