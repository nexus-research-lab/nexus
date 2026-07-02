# Room Memory MVP Summary

## 背景

Nexus 已经有 `internal/workspace/memory` 记忆引擎，也已经在 DM 和 Room runtime 中接入了自动召回、自动提交逻辑。但在 Room 页面里，`room_shared` 这一层记忆原本不可见、不可手动管理。

本次 MVP 的目标是：先不改自动提取和召回算法，只把当前 Room conversation 的共享记忆暴露出来，并提供基础管理能力。

## 已完成

### 后端

新增 Room conversation 级 memory API：

```text
GET    /nexus/v1/rooms/{room_id}/conversations/{conversation_id}/memory/items
POST   /nexus/v1/rooms/{room_id}/conversations/{conversation_id}/memory/items
PATCH  /nexus/v1/rooms/{room_id}/conversations/{conversation_id}/memory/items/{entry_id}
DELETE /nexus/v1/rooms/{room_id}/conversations/{conversation_id}/memory/items/{entry_id}
GET    /nexus/v1/rooms/{room_id}/conversations/{conversation_id}/memory/stats
```

这些接口通过 `room_id + conversation_id` 校验 conversation 归属，并固定写入 `room_shared:{room_id}:{conversation_id}` scope。

### 前端

Room header 新增 `Memory` tab。打开后右侧显示 `RoomMemorySurface`：

```text
- 统计：总数、候选、已访问、检查点
- 列表：title/content/status/kind/priority/scope/source/access_count
- 操作：新增、编辑、删除、刷新
```

## 涉及文件

```text
docs/memory-room-flow.md
docs/room-memory-mvp-summary.md
internal/app/server/routes.go
internal/handler/room/handlers.go
internal/service/room/service_memory.go
web/src/lib/api/memory-api.ts
web/src/types/conversation/room-surface.ts
web/src/features/conversation/room/group/header/group-conversation-header.tsx
web/src/features/conversation/room/surface/room-surface-layout.tsx
web/src/features/conversation/room/surface/room-memory-surface.tsx
```

## 验证

已完成以下验证：

```text
gofmt
go test ./internal/service/room ./internal/handler/room ./internal/app/server
pnpm exec tsc --noEmit
pnpm run build
Nexus health check
nginx health check
Room memory API create/update/delete smoke test
```

API smoke test 结果：

```json
{
  "created": true,
  "updated": "Codex CRUD smoke updated",
  "deleted": true
}
```

## 后续方向

下一步可以继续做两件事：

```text
1. 在每轮 Agent 回复旁展示本轮召回了哪些 memory。
2. 把 member agent 的 room_agent_session memory 合并展示到 Room Memory 面板里。
```
