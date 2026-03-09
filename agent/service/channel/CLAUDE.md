# channel/

L2 | 父级: agent/service/CLAUDE.md (如果存在)

## 消息通道抽象层

将 nexus-core 从 "WebSocket-only" 演进为多通道消息平台。

## 成员清单

- `__init__.py`: 模块导出，暴露核心协议和管理器
- `channel.py`: 核心协议定义 — `MessageSender`/`MessageChannel`/`PermissionStrategy`
- `channel_manager.py`: 通道注册与生命周期管理，在 app.py lifespan 中使用
- `websocket_channel.py`: WebSocket 通道实现 — `WebSocketSender` + `InteractivePermissionStrategy`
- `discord_channel.py`: Discord 通道实现 — `DiscordSender` + `DiscordChannel` + `AutoAllowPermissionStrategy`
- `telegram_channel.py`: Telegram 通道实现 — `TelegramSender` + `TelegramChannel`

## 架构

```
channel.py (协议层)
├── websocket_channel.py (WebSocket 实现)
├── discord_channel.py   (Discord 实现 + AutoAllowPermissionStrategy)
└── telegram_channel.py  (Telegram 实现，复用 AutoAllowPermissionStrategy)

channel_manager.py (编排层，管理所有通道生命周期)
```

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
