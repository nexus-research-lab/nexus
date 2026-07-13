# Launcher Agent Pile

- `launcher-agent-pile-model.ts` 以描述表定义确定性物理参数和品牌变体。
- `launcher-agent-pile-physics.ts` 封装 Matter 世界、可见性、动画和释放生命周期。
- `use-launcher-agent-pile-physics.ts` 只把 React 引用绑定到物理对象。
- `launcher-agent-token.tsx` 渲染单个可交互 Agent 或装饰 Room Token。
- `launcher-agent-pile.tsx` 只组合容器、Token 与物理引用。

Matter 对象不得进入 React 状态。Room Token 没有选择命令，必须保持为非交互元素；新增视觉变体通过描述表扩展，不增加渲染分支。
