# general/

L4 | 父级: web/src/features/settings

## 职责

- `settings-general-section.tsx`: 按设置导航分区装配 General、外观、工作区与权限视图
- `use-general-settings-controller.ts`: 行为、运行时与权限动作编排
- `use-user-preferences.ts`: 用户偏好加载、乐观保存和失败回滚
- `use-default-model-preferences.ts`: Provider 模型目录与默认模型选择
- `model/`: 偏好归一化、载荷、选项和模型选择纯函数
- `sections/`: 系统、外观、行为、工作区、权限和桌面设置视图
- `components/`: 默认模型与引导复位行

布局常量和分段控件来自 `../shared/settings-panel-ui.tsx`，General 不拥有跨域共享 UI。

用户偏好是默认模型值的唯一状态源，不维护选择值镜像。Provider 目录只随运行时类型变化加载，运行时动作不得主动触发第二次加载。
