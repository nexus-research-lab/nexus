# Workspace Controller

Room Workspace 的状态与命令边界。

## 职责

- `workspace-path-model.ts` 只保存路径合并、父目录、显示标签和前缀替换规则。
- `interaction/` 分离菜单/Prompt 状态、路径导航和条目事务。
- `use-workspace-agent-scope.ts` 负责 Room Agent 选择与外部打开请求协议。
- `use-workspace-files-resource.ts` 负责当前 Agent 的文件快照和加载错误。
- `use-workspace-commands.ts` 负责上传、创建、重命名、删除、下载/定位，以及桌面文件打开、复制路径和加入聊天事务；`workspace-command-recovery.ts` 只用精确 Agent/path/command 和权威列表判断当前目标状态。
- `use-room-workspace-controller.ts` 只编排上述能力，并按 `agent/browser/dialogs/fileInput` 消费者返回具体控制面。

## 不变量

- 文件资源、写命令和界面结果都必须绑定 Agent 作用域。
- 同一 Agent 只允许一个 Workspace 写命令在途；切换 Agent 后旧完成不得改写当前界面。
- 修改响应成功与随后列表刷新是两个阶段：刷新失败不得把已完成修改误报为失败；结果未知时锁住同一 Agent/path/command，先读列表核对且绝不自动重放。
- 多文件上传逐项保留已完成、未确认、明确未完成与未开始状态；上传可能自动改名且列表没有内容摘要，因此仅凭列表不得猜测未知上传已经完成。
- 外部资产打开请求可以切换 Agent，但不得清空该请求已设置的文件路径。
- 视图不得接收原始 state setter；关闭、选择和打开目标必须使用具名命令。
