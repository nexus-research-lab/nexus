# Room 页面协调

- 浏览器 URL 是当前 Room、Conversation 和外部 Session 路由的唯一真相源。
- `initial` 查询参数只消费一次，进入页面状态后立即从地址栏移除。
- 页面事件只处理路由失效和资源重同步；消息、Goal 等领域事件由各自控制器消费。
