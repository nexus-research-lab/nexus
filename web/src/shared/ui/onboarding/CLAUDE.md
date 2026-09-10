# onboarding/

L3 | 父级: web/src/shared/ui

## 职责

- `tour-contract.ts` 只声明 Tour、步骤和 Context 契约。
- `tour-context.ts` 只创建共享 Context 实例。
- `tour-provider.tsx` 持有注册表、完成状态、导航事务和 Overlay 装配。
- `tour-state.ts` 负责浏览器与桌面宿主的持久化边界。
- `use-onboarding-tour.ts` 与 `use-page-onboarding-tour.ts` 提供消费和页面注册生命周期。
- `overlay/` 负责目标观察、几何定位、卡片和目标高亮展示。
- Overlay 只能提供非阻塞的说明层；用户点击页面时应退出当前导览，并让原始交互继续执行。
- 产品级引导目录与文案属于 `features/onboarding/guide-center`，Shared 只保留 Tour 基础设施。

契约、Context 与 Provider 单向依赖，消费者不得从 Provider 文件提取协议类型。

注销引导后在微任务中核对是否重新注册；仍缺失时只关闭同 ID 的活动引导，不记录完成，不关闭随后启动的其他引导。同轮定义更新可保留进度。

宿主持久状态的初始化读取必须以本地完成状态代次为栅栏；用户已完成或重置引导后，迟到的旧快照不得覆盖新状态。
