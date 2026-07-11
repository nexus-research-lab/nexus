# Workspace Controller

Room Workspace 的状态与命令边界。

## 职责

- `workspace-path-model.ts` 只保存路径合并、父目录和前缀替换规则。
- `interaction/` 分离菜单/Prompt 状态、路径导航和条目事务。
- `use-workspace-agent-scope.ts` 负责 Room Agent 选择与外部打开请求协议。
- `use-workspace-files-resource.ts` 负责当前 Agent 的文件快照和加载错误。
- `use-workspace-commands.ts` 负责上传、创建、重命名、删除和下载/定位事务。
- `use-room-workspace-controller.ts` 只编排上述能力，并按 `agent/browser/dialogs/fileInput` 消费者返回具体控制面。

## 不变量

- 文件资源、写命令和界面结果都必须绑定 Agent 作用域。
- 同一 Agent 只允许一个 Workspace 写命令在途；切换 Agent 后旧完成不得改写当前界面。
- 外部资产打开请求可以切换 Agent，但不得清空该请求已设置的文件路径。
- 视图不得接收原始 state setter；关闭、选择和打开目标必须使用具名命令。
