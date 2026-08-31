import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const webRoot = fileURLToPath(new URL("..", import.meta.url));

function read(relativePath) {
  return readFile(path.join(webRoot, relativePath), "utf8");
}

test("private-domain reads keep safe scoped snapshots and complete recovery copy", async () => {
  const [view, timeline, model] = await Promise.all([
    read("src/features/agents/private-domain/agent-private-domain-view.tsx"),
    read("src/features/agents/private-domain/timeline/agent-private-domain-timeline.tsx"),
    read("src/features/agents/private-domain/timeline/agent-private-domain-timeline-model.ts"),
  ]);

  assert.match(view, /getErrorMessage\(loadError, recordsLoadError\)/);
  assert.match(view, /getErrorMessage\(loadError, messagesLoadError\)/);
  assert.match(view, /activeQueryKeyRef\.current !== requestKey/);
  assert.match(view, /activeEventsKeyRef\.current !== requestKey/);
  assert.match(view, /stale: threadsRef\.current\.length > 0/);
  assert.match(view, /stale: eventsRef\.current\.length > 0/);
  assert.doesNotMatch(view, /loadError instanceof Error \? loadError\.message/);

  assert.match(timeline, /<UiResourceState/);
  assert.match(timeline, /impact=/);
  assert.match(timeline, /nextStep=/);
  assert.match(timeline, /failure && !failure\.stale && events\.length === 0/);
  assert.doesNotMatch(model, /kind: "error"/);
});

test("root failures never expose arbitrary exceptions and always state impact and recovery", async () => {
  const [bootstrap, failureView, renderer] = await Promise.all([
    read("src/bootstrap/root-bootstrap.tsx"),
    read("src/bootstrap/root-failure-view.tsx"),
    read("src/bootstrap/root-renderer.tsx"),
  ]);

  assert.match(bootstrap, /getErrorMessage\(error, "暂时无法加载运行时配置"\)/);
  assert.doesNotMatch(bootstrap, /error instanceof Error \? error\.message/);
  assert.match(failureView, /impact: ReactNode/);
  assert.match(failureView, /nextStep: ReactNode/);
  assert.match(renderer, /impact=/);
  assert.match(renderer, /nextStep=/);
});
