# 定时任务控制器

- `use-scheduled-tasks-resource.ts` 独占列表请求、请求代次和本地权威命令结果。
- `use-scheduled-task-commands.ts` 独占目录写命令、并发状态、反馈和跨界摘要通知。
- `pending-command-model.ts` 只提供定时任务子域共用的在途命令集合，不解释具体业务动作。
- 资源接口由命令消费者定义，只暴露刷新、写入和移除三项能力。
- API 命令成功后先提交本地结果，再静默刷新；旧列表响应不得覆盖命令结果。
