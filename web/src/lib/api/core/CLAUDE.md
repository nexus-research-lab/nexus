# API 核心

- `http.ts` 是唯一通用请求编排入口，只负责 fetch/响应体生命周期、本地传输失败投影和桌面鉴权恢复决策；它不自动重试。
- `http-request.ts` 负责请求体归一化、超时与外部取消信号合并，并只按安全 HTTP 方法区分读取与结果未知的写入传输失败。
- `http-response.ts` 负责新旧响应兼容解析、FailureCore 宽容解码、数据读取和带诊断 request id 的错误文案投影；未知恢复动作不能自动执行。
- `http-error.ts` 分开保存服务端 FailureCore 错误与没有完整响应的本地 `ApiTransportError`；外部主动 Abort 保持浏览器原始异常。`http-auth.ts` 保存 HTTP / WebSocket 共用的鉴权失效事件。
- `timestamp.ts` 只做 API 时间字符串到前端时间戳的容错转换。
- 消费者直接导入职责模块；`http.ts` 不转发错误、选项或鉴权事件。
- 该目录不得依赖任何业务 Feature 或领域 API。
