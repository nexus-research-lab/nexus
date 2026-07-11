# session-navigator/

L4 | 父级: web/src/features/conversation/shared

## 职责

- `conversation-session-navigator.tsx`: 会话刻度与预览面板渲染
- `session-navigator-model.ts`: 将已加载消息或索引记录解析为统一来源，再投影为导航项
- `session-navigator-ruler-model.ts`: 刻度尺寸、波形、颜色分段和讲者文案的纯视觉模型
- `navigation-dom.ts`: 可见轮定位和滚动目标解析，不持有 React 状态
- `use-active-round.ts`: 当前可见轮同步和用户滚动中断
- `jump/`: 分离作用域化目标、缺失窗口加载队列、逐帧落点确认和跳转入口
- `use-conversation-session-navigation.ts`: 只组合展示状态和两个控制器

导航目标以滚动容器的 `data-conversation-round-navigation-target` 为唯一真相，
不要再增加平行 ref 或局部状态镜像。

跳转事务必须绑定 `scopeKey` 和请求代次。旧会话请求不得释放新会话锁、
清除新导航目标或修改当前活动轮；失效和失败的导航目标必须显式释放。

导航项中的 Agent ID 在数据投影阶段完成去空、去重；视觉模型只消费该不变量，
不得再次清洗数据或读取原始消息。
