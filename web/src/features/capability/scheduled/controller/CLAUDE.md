# 定时任务控制器

- `use-scheduled-tasks-resource.ts` 独占列表请求、请求代次和本地权威命令结果；主列表访问失效后保持敏感快照为空，普通刷新继续 fail closed，只有用户显式触发的同 owner 重新验证可以读取，且完整成功后才解除访问 fence。
- `use-scheduled-task-commands.ts` 独占目录写命令、权限决策与重试、当前页面防重/对账、durable 删除收尾/人工停止确认反馈和跨界摘要通知。跨页面并发与重载后的结果只服从服务端 configuration version、request receipt、run 和删除状态，不用浏览器 journal 或 Web Lock 伪造服务端正确性。
- `scheduled-task-create-intent.ts` 只保存非敏感创建 request ID，帮助重载后查询服务端 receipt；存储不可用不阻止用户创建，也不保存任务正文、鉴权信息或 HTTP 诊断号。
- `pending-command-model.ts` 只提供定时任务子域共用的在途命令集合，不解释具体业务动作。
- 资源接口由命令消费者定义，只暴露对账需要的刷新、访问失效、写入和移除能力。
- API 命令成功后先提交本地结果，再静默刷新；旧列表响应不得覆盖命令结果。
