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

test("history pages request deferred indexing by default and preserve its retry state", async () => {
  const {
    buildConversationMessagesQuerySuffix,
    normalizeConversationMessagePage,
  } = await server.ssrLoadModule(
    "/src/lib/api/conversation/message-page-model.ts",
  );

  assert.equal(
    buildConversationMessagesQuerySuffix({limit: 3}),
    "?defer_index=true&limit=3",
  );
  assert.equal(
    buildConversationMessagesQuerySuffix({defer_index: false, limit: 3}),
    "?limit=3",
  );
  assert.deepEqual(
    normalizeConversationMessagePage({
      has_more: false,
      indexing: true,
      items: [],
      retry_after_ms: 250,
    }),
    {
      has_more: false,
      indexing: true,
      items: [],
      next_before_round_id: null,
      next_before_round_timestamp: null,
      retry_after_ms: 250,
    },
  );
});

test("indexing responses stay in the request loop instead of becoming empty history", async () => {
  const [historySource, roundIndexApi, roundIndexHook] = await Promise.all([
    readFile(
      path.join(webRoot, "src/hooks/agent/session/conversation-history.ts"),
      "utf8",
    ),
    readFile(
      path.join(webRoot, "src/lib/api/conversation/session-api.ts"),
      "utf8",
    ),
    readFile(
      path.join(webRoot, "src/hooks/conversation/use-session-round-index.ts"),
      "utf8",
    ),
  ]);
  assert.match(historySource, /if \(!page\.indexing\) \{\s*return page;/);
  assert.match(historySource, /await waitForHistoryIndex\(page\.retry_after_ms\)/);
  assert.match(roundIndexApi, /if \(!result\.indexing\)/);
  assert.match(roundIndexApi, /await waitForSessionRoundIndex/);
  assert.match(roundIndexHook, /new AbortController\(\)/);
  assert.match(roundIndexHook, /return \(\) => controller\.abort\(\)/);
});
