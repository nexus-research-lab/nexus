# agent/service/handler/

L2 | 父级: [service/CLAUDE.md](file:///Users/leemysw/Projects/nexus-core/agent/service/CLAUDE.md)

## 成员清单

- `__init__.py`: 模块导出
- `base_handler.py`: WebSocket 处理器基类，消息发送抽象
- `chat_handler.py`: 聊天消息处理，SDK client 懒加载，`setting_sources` 参数透传
- `debug_handler.py`: 调试消息处理器，模拟发送各类消息用于前端调试
- `error_handler.py`: 错误处理与错误响应格式化
- `interrupt_handler.py`: 会话中断处理
- `permission_handler.py`: 工具权限请求与审批
- `ping_handler.py`: 心跳检测处理

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
