# provider-settings/

L4 | 父级: web/src/features/settings

## 职责

- `provider-settings-panel.tsx`: Provider 设置入口与视图装配
- `provider-settings-api.ts`: 私有与公共 Provider API 族选择
- `use-provider-workspace.ts`: Provider 列表、选择、模式和草稿的原子 workspace
- `use-provider-settings-controller.ts`: 配置、模型动作与展示投影装配
- `actions/`: 唯一命令互斥、配置字段/持久化/删除命令、模型变更与连通性测试
- `model/`: 预设、配置、模型列表和展示映射的纯模型
- `components/`: 侧栏、配置表单、详情头和模型列表
- `dialogs/`: 新增模型、模型参数和删除占用确认

Provider 列表、选中项、表单模式和草稿属于同一个 workspace，刷新时必须原子替换；
删除弹窗使用带类型的单一状态，不增加目标、确认框和占用框的平行布尔状态；
所有异步命令共享一个基于 ref 的互斥入口，不增加镜像 busy/submitting 状态。
模型与测试动作只依赖 `PersistProvider` 窄命令，不读取配置动作控制器的完整状态。
