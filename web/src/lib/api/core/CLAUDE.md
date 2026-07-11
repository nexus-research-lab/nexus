# API 核心

- `http.ts` 是唯一通用请求编排入口，只负责 fetch 生命周期和桌面鉴权恢复决策。
- `http-request.ts` 负责请求体归一化、超时与外部取消信号合并。
- `http-response.ts` 负责响应解析、数据读取和带 request id 的错误文案投影。
- `http-error.ts` 保存传输错误类型，`http-auth.ts` 保存 HTTP / WebSocket 共用的鉴权失效事件。
- `timestamp.ts` 只做 API 时间字符串到前端时间戳的容错转换。
- 消费者直接导入职责模块；`http.ts` 不转发错误、选项或鉴权事件。
- 该目录不得依赖任何业务 Feature 或领域 API。
