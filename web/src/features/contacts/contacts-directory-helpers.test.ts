// INPUT: Agent 目录的身份、业务/风格标签、Provider 与权限过滤条件。
// OUTPUT: 证明搜索字段边界、组合过滤、默认 Provider 和筛选候选去重行为。
// POS: Contacts 目录纯模型测试；直接导入生产模型，不启动短生命周期 Vite Server。

import { expect, it } from "vitest";

import type { Agent } from "@/types/agent/agent";

import {
  CONTACTS_DEFAULT_PROVIDER_FILTER,
  filterContactsAgents,
  getContactsDirectoryBusinessTags,
  getContactsDirectoryPermissionModes,
  getContactsDirectoryProviders,
  matchesContactsSearch,
} from "./contacts-directory-helpers";

const agents: Agent[] = [
  {
    agent_id: "amy",
    name: "Amy",
    workspace_path: "/amy",
    options: { permission_mode: "acceptEdits" },
    created_at: 1,
    status: "idle",
    business_tags: ["Research", "Long primary tag"],
    vibe_tags: ["Friendly"],
  },
  {
    agent_id: "bob",
    name: "Bob",
    workspace_path: "/bob",
    options: { permission_mode: "plan", provider: "openai" },
    created_at: 2,
    status: "idle",
    business_tags: ["Code", "research"],
    vibe_tags: ["Direct"],
  },
];

it("Agent 目录搜索与筛选只使用业务标签、Provider 和权限数据", () => {
  expect(matchesContactsSearch(agents[0], "research")).toBe(true);
  expect(matchesContactsSearch(agents[0], "friendly")).toBe(false);
  expect(filterContactsAgents(agents, {
    permissionMode: "",
    provider: "",
    query: "",
    tag: "Research",
  }).map((agent) => agent.agent_id)).toEqual(["amy", "bob"]);
  expect(filterContactsAgents(agents, {
    permissionMode: "plan",
    provider: "openai",
    query: "bob",
    tag: "Code",
  }).map((agent) => agent.agent_id)).toEqual(["bob"]);
  expect(filterContactsAgents(agents, {
    permissionMode: "",
    provider: CONTACTS_DEFAULT_PROVIDER_FILTER,
    query: "",
    tag: "",
  }).map((agent) => agent.agent_id)).toEqual(["amy"]);
  expect(new Set(getContactsDirectoryBusinessTags(agents))).toEqual(new Set([
    "Code",
    "Long primary tag",
    "Research",
  ]));
  expect(new Set(getContactsDirectoryProviders(agents))).toEqual(new Set([
    CONTACTS_DEFAULT_PROVIDER_FILTER,
    "openai",
  ]));
  expect(new Set(getContactsDirectoryPermissionModes(agents))).toEqual(new Set([
    "acceptEdits",
    "plan",
  ]));
});
