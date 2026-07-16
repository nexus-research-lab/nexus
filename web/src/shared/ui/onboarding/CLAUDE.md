# onboarding/

L3 | 父级: web/src/shared/ui

## 职责

- `tour-contract.ts` 只声明 Tour、步骤和 Context 契约。
- `tour-context.ts` 只创建共享 Context 实例。
- `tour-provider.tsx` 持有注册表、完成状态、导航事务和 Overlay 装配。
- `tour-state.ts` 负责浏览器与桌面宿主的持久化边界。
- `use-onboarding-tour.ts` 与 `use-page-onboarding-tour.ts` 提供消费和页面注册生命周期。
- `overlay/` 负责目标观察、几何定位、卡片和目标高亮展示。
- 产品级引导目录与文案属于 `features/onboarding/guide-center`，Shared 只保留 Tour 基础设施。

契约、Context 与 Provider 单向依赖，消费者不得从 Provider 文件提取协议类型。
