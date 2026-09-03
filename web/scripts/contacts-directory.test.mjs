import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

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

const agents = [
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

test("Agent 目录视图切换由共享分段控件统一管理", async () => {
  const source = await readFile(
    path.join(webRoot, "src/features/contacts/contacts-directory.tsx"),
    "utf8",
  );

  assert.match(source, /<UiSegmentedControl/);
  assert.match(source, /contacts\.views\.title/);
  assert.doesNotMatch(source, /aria-pressed=\{view ===/);
});

test("Agent 目录搜索与筛选只使用业务标签、Provider 和权限数据", async () => {
  const {
    CONTACTS_DEFAULT_PROVIDER_FILTER,
    filterContactsAgents,
    getContactsDirectoryBusinessTags,
    getContactsDirectoryPermissionModes,
    getContactsDirectoryProviders,
    matchesContactsSearch,
  } = await server.ssrLoadModule(
    "/src/features/contacts/contacts-directory-helpers.ts",
  );

  assert.equal(matchesContactsSearch(agents[0], "research"), true);
  assert.equal(matchesContactsSearch(agents[0], "friendly"), false);
  assert.deepEqual(
    filterContactsAgents(agents, {
      permissionMode: "",
      provider: "",
      query: "",
      tag: "Research",
    }).map((agent) => agent.agent_id),
    ["amy", "bob"],
  );
  assert.deepEqual(
    filterContactsAgents(agents, {
      permissionMode: "plan",
      provider: "openai",
      query: "bob",
      tag: "Code",
    }).map((agent) => agent.agent_id),
    ["bob"],
  );
  assert.deepEqual(
    filterContactsAgents(agents, {
      permissionMode: "",
      provider: CONTACTS_DEFAULT_PROVIDER_FILTER,
      query: "",
      tag: "",
    }).map((agent) => agent.agent_id),
    ["amy"],
  );
  assert.deepEqual(new Set(getContactsDirectoryBusinessTags(agents)), new Set([
    "Code",
    "Long primary tag",
    "Research",
  ]));
  assert.deepEqual(new Set(getContactsDirectoryProviders(agents)), new Set([
    CONTACTS_DEFAULT_PROVIDER_FILTER,
    "openai",
  ]));
  assert.deepEqual(new Set(getContactsDirectoryPermissionModes(agents)), new Set([
    "acceptEdits",
    "plan",
  ]));
});
