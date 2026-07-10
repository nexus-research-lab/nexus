# general/

L4 | 父级: web/src/features/settings

## 职责

- `settings-general-section.tsx`: General 设置页纯视图装配
- `use-general-settings-controller.ts`: 行为、运行时与权限动作编排
- `use-user-preferences.ts`: 用户偏好加载、乐观保存和失败回滚
- `use-default-model-preferences.ts`: Provider 模型目录与默认模型选择
- `settings-preferences-model.ts`: 偏好归一化、载荷和模型选择纯函数
- `sections/`: General、工作区和默认权限视图
- `components/`: 默认模型与引导复位行

用户偏好是默认模型值的唯一状态源，不维护选择值镜像。Provider 目录只随运行时类型变化加载，运行时动作不得主动触发第二次加载。
