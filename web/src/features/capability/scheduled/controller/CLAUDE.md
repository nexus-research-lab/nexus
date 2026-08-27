# 定时任务控制器

- `use-scheduled-tasks-resource.ts` 独占列表请求、请求代次和本地权威命令结果；主列表访问失效后保持敏感快照为空，普通刷新继续 fail closed，只有用户显式触发的同 owner 重新验证可以读取，且完整成功后才解除访问 fence。
- `use-scheduled-task-commands.ts` 独占目录写命令、权限决策与重试、exact journal/对账、durable 删除收尾/人工停止确认反馈和跨界摘要通知。同 owner 的多个页面通过 storage event 合并保护，并用每个 Job 一把 Web Lock 串行化实际写操作；不同 Job 不互相阻塞，锁外不轮询。缺少 Web Locks 的非支持环境必须在发送前 fail closed，不能用带超时的 localStorage lease 伪造安全互斥。`deleting` 或 `review_required` 不能因 configuration version 已增加而解除原删除保护；`confirm-stopped` 使用独立 exact command/journal，只有权威任务定义消失才算完成。
- `scheduled-task-mutation-journal.ts` 为每个 owner/command/target 与每个创建 request 单独使用一个 localStorage key，避免多页面 read-modify-write 覆盖；记录不含任务正文、鉴权信息或 HTTP trace ID，也不按时间猜测结果并淘汰。
- `pending-command-model.ts` 只提供定时任务子域共用的在途命令集合，不解释具体业务动作。
- 资源接口由命令消费者定义，只暴露对账需要的刷新、访问失效、写入和移除能力。
- API 命令成功后先提交本地结果，再静默刷新；旧列表响应不得覆盖命令结果。
