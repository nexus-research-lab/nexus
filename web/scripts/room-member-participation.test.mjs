import assert from "node:assert/strict";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";

import { createServer } from "vite";

const webRoot = fileURLToPath(new URL("..", import.meta.url));
const server = await createServer({
  configFile: false,
  logLevel: "silent",
  resolve: { alias: { "@": path.join(webRoot, "src") } },
  root: webRoot,
  server: { middlewareMode: true },
});

test.after(async () => {
  await server.close();
});

async function renderWithI18n(element, locale = "zh") {
  const { I18N_CONTEXT } = await server.ssrLoadModule(
    "/src/shared/i18n/i18n-context.ts",
  );
  const { MESSAGES } = await server.ssrLoadModule(
    "/src/shared/i18n/messages.ts",
  );
  const value = {
    locale,
    setLocale: () => {},
    t: (key, params = {}) => Object.entries(params).reduce(
      (message, [name, replacement]) => message.replaceAll(
        `{${name}}`,
        String(replacement),
      ),
      MESSAGES[locale][key] ?? key,
    ),
  };
  return renderToStaticMarkup(
    React.createElement(I18N_CONTEXT.Provider, { value }, element),
  );
}

test("Room 成员参与计划只提交实际变化并保留新成员暂停草稿", async () => {
  const { buildRoomParticipationPlan } = await server.ssrLoadModule(
    "/src/pages/room/controller/commands/room-management-command-model.ts",
  );

  assert.deepEqual(
    buildRoomParticipationPlan(
      [
        { agent_id: "agent-a", room_participation_paused: false },
        { agent_id: "agent-b", room_participation_paused: true },
        { agent_id: "agent-removed", room_participation_paused: true },
      ],
      ["agent-a", "agent-b", "agent-new"],
      ["agent-a", "agent-b", "agent-new"],
    ),
    [
      { agentId: "agent-a", paused: true },
      { agentId: "agent-new", paused: true },
    ],
  );
});

test("Room 成员投影把持久 participation 状态附着到当前 Room 身份", async () => {
  const { resolveCurrentRoomMemberAgents } = await server.ssrLoadModule(
    "/src/pages/room/controller/model/room-member-model.ts",
  );
  const roomContext = {
    room: { id: "room-1", room_type: "group" },
    conversation: { id: "conversation-1" },
    sessions: [],
    members: [
      {
        id: "member-a",
        room_id: "room-1",
        member_type: "agent",
        member_agent_id: "agent-a",
        participation_paused: true,
      },
      {
        id: "member-b",
        room_id: "room-1",
        member_type: "agent",
        member_agent_id: "agent-b",
        participation_paused: false,
      },
    ],
    member_agents: [
      { agent_id: "agent-a", name: "旧 A" },
      { agent_id: "agent-b", name: "旧 B" },
    ],
  };
  const projected = resolveCurrentRoomMemberAgents(
    [roomContext],
    [
      { agent_id: "agent-a", name: "目录 A" },
      { agent_id: "agent-b", name: "目录 B" },
    ],
  );

  assert.deepEqual(
    projected.map((agent) => ({
      agentId: agent.agent_id,
      name: agent.name,
      paused: agent.room_participation_paused,
    })),
    [
      { agentId: "agent-a", name: "目录 A", paused: true },
      { agentId: "agent-b", name: "目录 B", paused: false },
    ],
  );
});

test("Room 管理弹窗为每个已选成员显示独立暂停或恢复动作", async () => {
  const { RoomMemberSelector } = await server.ssrLoadModule(
    "/src/features/conversation/room/members/room-member-selector.tsx",
  );
  const html = await renderWithI18n(React.createElement(RoomMemberSelector, {
    agents: [
      { agent_id: "agent-a", name: "研究员 A" },
      { agent_id: "agent-b", name: "研究员 B" },
    ],
    canManageParticipation: true,
    onQueryChange: () => {},
    onToggleAgent: () => {},
    onToggleParticipation: () => {},
    pausedAgentIds: new Set(["agent-a"]),
    query: "",
    selectedAgentIds: new Set(["agent-a", "agent-b"]),
  }));

  assert.match(html, /aria-label="恢复 研究员 A 参与"/);
  assert.match(html, /aria-label="暂停 研究员 B 参与"/);
  assert.match(html, />恢复参与</);
  assert.match(html, />暂停参与</);
});
