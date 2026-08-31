import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const webRoot = fileURLToPath(new URL("..", import.meta.url));

test("WorkGraph history never carries a snapshot or late response across sessions", async () => {
  const resource = await read(
    "src/features/conversation/shared/execution/use-workgraph-history-resource.ts",
  );
  const surface = await read(
    "src/features/conversation/shared/execution/execution-workgraph-surface.tsx",
  );

  assert.match(resource, /useResettableState<ExecutionView\[\]>\(\[\], sessionKey\)/);
  assert.match(resource, /activeSessionRef\.current === requestSessionKey/);
  assert.match(surface, /useResettableState<WorkGraphSurfaceMode>[\s\S]*sessionScopeKey/);
  assert.match(surface, /selectedHistoryId[\s\S]*sessionScopeKey/);
  assert.match(surface, /sketchScopeRef\.current === requestedScope/);
});

test("WorkGraph history read failures state result, impact, and recovery", async () => {
  const surface = await read(
    "src/features/conversation/shared/execution/execution-workgraph-surface.tsx",
  );

  assert.match(surface, /ReadResourceReliabilityNotice/);
  assert.match(surface, /surface_history_load_failed/);
  assert.match(surface, /surface_history_(?:stale|unavailable)_impact/);
  assert.match(surface, /surface_history_failure_next_step/);
  assert.match(surface, /onRefresh=\{historyResource\.refresh\}/);
});

function read(relativePath) {
  return readFile(path.join(webRoot, relativePath), "utf8");
}
