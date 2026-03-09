# Agent UI Frontend Interface

> 🔧 Agent UI前端接口文档 - 包含API、类型定义、组件使用等前端开发指南

---

## 🏗️ 架构概览

### 技术栈
- **框架**: Next.js 14 (App Router)
- **状态管理**: Zustand + Persist
- **样式**: Tailwind CSS + Radix UI
- **组件**: React + TypeScript
- **实时通信**: WebSocket

### 项目结构
```
src/
├── app/                  # Next.js 页面路由
├── components/           # UI组件库
├── hooks/               # 自定义Hook
├── lib/                 # 工具函数
├── store/              # 状态管理
├── types/               # 类型定义
└── utils/               # 工具函数
```

---

## 🔌 API接口

### 基础配置

```typescript
// API 基础URL
const AGENT_API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8010/agent/v1';

// WebSocket配置
const WS_URL = process.env.NEXT_PUBLIC_WS_URL || 'ws://localhost:8010/agent/v1/chat/ws';
```

### API响应类型

```typescript
interface ApiResponse<T> {
  code: number;
  message: string;
  data: T;
  request_id?: string;
}
```

### Session API

```typescript
// 获取所有会话
const getSessions = async (): Promise<Session[]> => {
  const response = await fetch(`${AGENT_API_BASE_URL}/sessions`);
  return response.json().then(res => res.data.map(transformApiSession));
};

// 获取会话消息
const getSessionMessages = async (agentId: string): Promise<Message[]> => {
  const response = await fetch(`${AGENT_API_BASE_URL}/sessions/${agentId}/messages`);
  return response.json().then(res => res.data);
};

// 更新会话标题
const updateSessionTitle = async (agentId: string, title: string): Promise<{ success: boolean }> => {
  const response = await fetch(`${AGENT_API_BASE_URL}/sessions/${agentId}/title`, {
    method: 'POST',
    body: JSON.stringify({ title }),
  });
  return response.json().then(res => res.data);
};

// 删除会话
const deleteSession = async (agentId: string): Promise<{ success: boolean }> => {
  const response = await fetch(`${AGENT_API_BASE_URL}/sessions/${agentId}`, {
    method: 'DELETE',
  });
  return response.json().then(res => res.data);
};
```

---

## 🛠️ 开发规范


### 目录结构

```
src/
├── app/                 # 页面路由 (按功能分组)
│   ├── page.tsx
│   ├── dashboard/
│   └── settings/
├── components/          # UI组件 (按类型分组)
│   ├── ui/              # 基础UI组件
│   │   ├── button.tsx
│   │   ├── input.tsx
│   │   └── modal.tsx
│   ├── message/         # 消息相关组件
│   │   ├── message-item.tsx
│   │   ├── message-avatar.tsx
│   │   └── message-actions.tsx
│   └── chat/           # 聊天相关组件
├── hooks/              # 自定义Hook (按功能分组)
│   ├── agent/
│   └── websocket/
├── lib/                # 工具函数 (按用途分组)
│   ├── utils/
│   ├── api/
│   └── websocket/
├── store/             # 状态管理 (按模块分组)
│   ├── session/
│   ├── settings/
│   └── index.ts
└── types/             # 类型定义 (按模块分组)
    ├── message/
    ├── session/
    └── index.ts
```

*🎯 让AI更智能，让交互更自然*