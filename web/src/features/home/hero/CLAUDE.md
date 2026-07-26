# Home Hero

## 职责边界

- `home-ascii-hero.tsx` 只负责 React 视图、主题和减少动态效果偏好。
- Light/Sunny 的 ASCII Hero 使用局部 `--home-ascii-*` token 保留原有蓝色粒子、时钟和浅蓝白画布，不跟随主应用壳层的灰白黑重塑；Dark/Rain 继续消费各自主题 token。
- `use-home-ascii-scene.ts` 只负责把 React 生命周期绑定到 Canvas 场景。
- `home-ascii-scene.ts` 负责 Canvas、ResizeObserver、指针事件、时钟和动画资源。
- `home-ascii-particle-model.ts` 负责粒子字段创建和逐帧状态更新，不访问 DOM。

## 约定

- Canvas 场景状态不进入 React state，避免逐帧触发组件渲染。
- 异步重建必须绑定递增代次；过期的字体等待或尺寸重建不得启动动画。
- 场景销毁必须同时释放动画帧、定时器、观察器和输入事件。
- 桌面 `/app` 的侧栏 Header 与主内容区顶部透明拖动面共同承担窗口拖动；Hero Canvas 仍直接贴合工作区顶部且始终保留内容交互，不参与窗口移动。
