# general/

L4 | 父级: web/src/features/settings

## 职责

- `settings-general-section.tsx`: 按设置导航分区装配 General、外观、工作区与权限视图
- `use-general-settings-controller.ts`: 常规行为、默认模型与权限动作编排
- `use-user-preferences.ts`: 用户偏好加载、乐观保存和失败回滚
- `use-default-model-preferences.ts`: Provider 模型目录请求与默认模型保存事务
- `use-desktop-settings.ts`、`use-workspace-settings.ts`: 各自独占 Section 所需的资源和命令生命周期
- `model/`: 分别组装完整偏好、默认模型目录展示和工作区路径快照；跨 Config 的值清洗规则归 `lib/settings/`
- `sections/`: 桌面应用、外观、行为、工作区和权限纯视图，不直接调用 API 或 Desktop Bridge
- `components/`: 默认模型与引导复位行

布局常量和分段控件来自 `../shared/settings-panel-ui.tsx`，General 不拥有跨域共享 UI。

用户偏好是默认模型值的唯一状态源，不维护选择值镜像。Provider 目录只随运行时类型变化加载，运行时动作不得主动触发第二次加载；目录响应严格按当前协议投影，不保留旧字段兜底。

默认模型行在桌面端为 Provider / Model 控件保留 300–360px 列宽，窄屏回落为整行宽度；触发器保持单行并使用足以容纳英文下行字形的行盒，展开菜单通过锚点定位的 `menuMinWidth` 保证完整名称可读。

常规页只展示桌面宿主 Bridge 返回的应用版本与构建号，并在同一区域提供日志导出；Go sidecar 的服务版本不再投影成用户可见的应用版本。Web 模式没有 Desktop Bridge 时隐藏整个桌面应用区域。
