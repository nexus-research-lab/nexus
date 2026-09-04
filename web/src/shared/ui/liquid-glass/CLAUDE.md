# Liquid Glass 原语

- `liquid-glass-engine.ts` 只判断浏览器能力，不依赖 React。
- `use-liquid-glass-support.ts` 负责挂载后的能力启用和稳定 filter id。
- `glass-switch.tsx` 只投影并组合开关几何；键盘、指针和过渡结束协议归交互 Hook，SVG 资源链归 Filter 视图。
- `GlassSwitch` 强制消费并转发具体的 `aria-label`，并由同一个 native button/`role=switch` 持有点击、键盘、`aria-checked` 与真实 disabled；每个调用方必须用当前界面语言描述实际切换目标，禁止匿名开关，也不得在外层增加 `span role=button` 伪造第二命中区。业务需要解释受保护状态时，保持 switch 可操作并在 `onChange` 中进入确认/说明流程。
- `glass-magnifier.tsx` 只组合放大镜表面；Web Animation 生命周期和 SVG 资源链分别独立维护。
- 消费者直接导入具体组件，不恢复目录级聚合出口。
- 动画资源必须在卸载时从当前 ref 取消；禁止在 render 阶段通过状态写入同步 Props。
