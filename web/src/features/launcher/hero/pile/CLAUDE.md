# Launcher Agent Pile

- `launcher-agent-pile-model.ts` 以描述表定义确定性物理参数和品牌变体。
- `launcher-agent-pile-physics.ts` 封装 Matter 世界、可见性、动画和释放生命周期。
- `use-launcher-agent-pile-physics.ts` 只把 React 引用绑定到物理对象。
- `launcher-agent-token.tsx` 渲染单个可交互 Agent 或装饰 Room Token。
- `launcher-agent-pile.tsx` 只组合容器、Token 与物理引用。

Matter 对象不得进入 React 状态。Room Token 没有选择命令，必须保持为非交互元素；新增视觉变体通过描述表扩展，不增加渲染分支。

- 减少动效时使用共享偏好hook，物理场景只计算一次静态落位，不安排掉落timer或动画帧；偏好切换释放旧实例并重建。普通模式延迟加入刚体时重新唤醒已休眠循环。
- Agent按钮用完整目录名称生成本地化动作名称并提供pressed/内嵌focus状态；缩写和品牌tag仅作视觉。Room类型即使误带Agent ID仍保持装饰，不得获得私聊导航。
