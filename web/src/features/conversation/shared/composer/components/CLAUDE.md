# components/

L5 | 父级: web/src/features/conversation/shared/composer

## 职责

- `composer-input-row.tsx`: 装配后端权威 Slash/Mention 补全与 textarea，提交和其他动作留在底部工具行；Mention 直接使用真实 textarea ref，共享浮层负责实时位置、可访问候选关联和输入焦点保留，不在消费侧计算矩形或强制方向
- `slash-command-popover.tsx`: 复用 Shared Select Menu 展示后端静态命令目录、`/model` 模型子面板、`/skills` 技能子面板及异步/空状态
- `composer-submit-button.tsx`: 以单一投影选择停止、加载、Goal 或发送动作
- `composer-local-directories.tsx`: 以具名横向组展示当前 Session 的目录，消费共享 RemovableChip 与范围 Badge，保留完整路径 title 和精确路径移除；添加使用共享 IconButton。禁用、保存中或控制器明确 blocksMutation 时禁止目录变更，安全读取/重新加载仍可用；无目录时不渲染空组，不在视图重做授权或持久化逻辑
- `interaction/`: DM 等待用户确认、回答或批准计划时，原位替换输入壳的唯一交互 surface
- `footer/`: 动作菜单、Session 级模型/权限、Goal 标记、运行状态、输入元数据和提交动作
- `pending-queue/`: 待发送消息、拖拽重排和队列命令
- `loop-picker/`: Loop 目录资源、筛选、选择事务和 Dialog 展示
- `workgraph-distillation-picker/`: owner 工作图目录、搜索、键盘选项与只读预览；明确复用时只将原始 Slash 写回当前 Composer

组件只消费控制器或本子域模型的明确结果，不重新派生发送资格、运行时阶段或跨域协议状态。
Slash 浮层必须与 Composer 输入壳同宽并复用 `SelectMenuPanel`、`SelectMenuOptionRow`、`UiSearchInput`、共享菜单行状态和锚定浮层生命周期；一级命令与模型各占一行，其中一级命令为名称预留稳定列宽，让说明从同一纵向基线开始、参数提示保持右对齐，Skill 最多保留名称与说明两行，不再增加标题区、计数徽标或重复状态文案。浮层高度保持有界，搜索栏固定，只有条目列表承担纵向滚动。
