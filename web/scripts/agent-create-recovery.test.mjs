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

test("Agent creation journal stores only exact request identity and status", async () => {
  const values = new Map();
  const localStorage = {
    getItem: (key) => values.get(key) ?? null,
    removeItem: (key) => values.delete(key),
    setItem: (key, value) => values.set(key, value),
  };
  Object.defineProperty(globalThis, "window", {
    configurable: true,
    value: { localStorage },
  });
  Object.defineProperty(globalThis, "navigator", {
    configurable: true,
    value: {
      locks: {
        request: async (_name, _options, execute) => execute(),
      },
    },
  });
  const journal = await server.ssrLoadModule(
    "/src/store/agent/agent-creation-journal.ts",
  );

  assert.equal(journal.writeAgentCreationJournal("owner:a", {
    requestId: "web-create:one",
    status: "pending",
  }), true);
  assert.deepEqual(journal.readAgentCreationJournal("owner:a"), {
    available: true,
    entry: { requestId: "web-create:one", status: "pending" },
  });
  assert.equal(journal.readAgentCreationJournal("owner:b").entry, null);
  const persisted = JSON.parse([...values.values()][0]);
  assert.deepEqual(Object.keys(persisted).sort(), ["requestId", "status", "version"]);
  assert.equal("name" in persisted, false);
  assert.equal("digest" in persisted, false);
  assert.equal("body" in persisted, false);
  assert.equal("transportRequestId" in persisted, false);
  assert.equal(
    await journal.withAgentCreationJournalLock("owner:a", async () => "locked"),
    "locked",
  );

  const originalSetItem = localStorage.setItem;
  localStorage.setItem = () => {};
  assert.equal(journal.writeAgentCreationJournal("owner:c", {
    requestId: "web-create:not-persisted",
    status: "pending",
  }), false);
  localStorage.setItem = originalSetItem;

  let sideEffectStarted = false;
  navigator.locks.request = async () => {
    throw new Error("lock service unavailable");
  };
  await assert.rejects(
    journal.withAgentCreationJournalLock("owner:a", async () => {
      sideEffectStarted = true;
    }),
    /无法安全保留 Agent 创建记录/,
  );
  assert.equal(sideEffectStarted, false);
});

test("Agent create reconciles exact receipt before reusing the same request ID", async () => {
  const [store, api, journal] = await Promise.all([
    read("src/store/agent/index.ts"),
    read("src/lib/api/agent/agent-api.ts"),
    read("src/store/agent/agent-creation-journal.ts"),
  ]);
  const createBody = store.slice(
    store.indexOf("create_agent: async"),
    store.indexOf("delete_agent: async"),
  );

  assert.ok(createBody.indexOf("getAgentCreationRequestApi") < createBody.indexOf("createAgentApi({"));
  assert.match(createBody, /creation_request_id: creationRequestId/);
  assert.match(createBody, /result\.status === "committed"/);
  assert.match(createBody, /result\.status === "deleted" \|\| result\.status === "failed"/);
  assert.match(createBody, /result\.status === "pending"[\s\S]*agent\.creation_in_progress[\s\S]*"accepted"/);
  assert.ok(
    createBody.indexOf('result.status === "pending"')
      < createBody.indexOf("createAgentApi({"),
  );
  assert.match(createBody, /status: "unconfirmed"/);
  assert.match(store, /agents: \[agent, \.\.\.state\.agents\.filter/);
  assert.doesNotMatch(store, /X-Request-ID|transportRequestId/);
  assert.match(api, /agents\/create-requests\/\$\{encodeURIComponent\(creationRequestId\)\}/);
  assert.match(journal, /navigator\.locks/);
  assert.match(journal, /throw new AgentCreationCoordinationUnavailableError/);
});

test("Agent create feedback answers problem, impact, and next step in both languages", async () => {
  const [{ zhAgentMessages }, { enAgentMessages }] = await Promise.all([
    server.ssrLoadModule("/src/shared/i18n/catalog/zh/agent.ts"),
    server.ssrLoadModule("/src/shared/i18n/catalog/en/agent.ts"),
  ]);
  for (const catalog of [zhAgentMessages, enAgentMessages]) {
    for (const effect of ["not_applied", "accepted", "committed", "unknown"]) {
      assert.ok(catalog[`agent_options.create_${effect}_message`]);
      assert.ok(catalog[`agent_options.create_${effect}_impact`]);
      assert.ok(catalog[`agent_options.create_${effect}_next_step`]);
    }
  }
});

function read(relativePath) {
  return readFile(path.join(webRoot, relativePath), "utf8");
}
