# Nexus Core Web

React + Vite 前端。

## 技术栈

- React 19
- Vite 7
- React Router 7
- TypeScript 5
- Zustand
- Tailwind CSS 4

## 目录

```text
src/
├── app/            # 样式、路由常量
├── features/       # 业务模块
├── hooks/          # 页面与会话 hooks
├── lib/            # API、WebSocket、工具函数
├── pages/          # 路由页面
├── routes/         # Router 定义
├── shared/         # 共享 UI
├── store/          # Zustand 状态
└── types/          # 类型定义
```

## 环境变量

```bash
cp env.example .env.local
```

本地开发常用配置：

```bash
VITE_WS_URL=ws://localhost:8010/agent/v1/chat/ws
VITE_API_URL=http://localhost:8010/agent/v1
VITE_DEFAULT_MODEL=glm-5
```

## 启动

```bash
npm install
npm run dev
```

默认地址：`http://localhost:3000`

## 校验

```bash
npm run build
npm run start
npm run lint
npx tsc --noEmit
```

## 路由

```text
/                                   Launcher
/dms                                私聊目录
/rooms                              房间目录
/rooms/:room_id                     房间入口
/rooms/:room_id/conversations/:conversation_id
/contacts                           联系人目录
/contacts/:agent_id                 联系人详情
```

## 后端通信

- HTTP：`VITE_API_URL`
- WebSocket：`VITE_WS_URL`
