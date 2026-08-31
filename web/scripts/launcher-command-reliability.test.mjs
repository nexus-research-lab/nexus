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

test("Launcher projects reads and uncertain DM preparation into different recovery", async () => {
  const { projectLauncherOperationFailure } = await server.ssrLoadModule(
    "/src/features/launcher/console/launcher-operation-failure.ts",
  );
  const t = (key) => key;
  const recover = () => {};

  const queryFailure = projectLauncherOperationFailure(
    t,
    { kind: "query_read" },
    recover,
  );
  assert.equal(queryFailure.action.label, "launcher.failure.retry_query");
  assert.equal(queryFailure.impact, "launcher.failure.query_impact");
  assert.equal(queryFailure.nextStep, "launcher.failure.query_next_step");
  assert.equal(queryFailure.tone, "error");

  const unknownDm = projectLauncherOperationFailure(
    t,
    { effect: "unknown", kind: "direct_room" },
    recover,
  );
  assert.equal(unknownDm.action.label, "launcher.failure.open_workspace");
  assert.equal(
    unknownDm.impact,
    "launcher.failure.direct_room_unknown_impact",
  );
  assert.equal(
    unknownDm.nextStep,
    "launcher.failure.direct_room_check_next_step",
  );
  assert.equal(unknownDm.tone, "warning");

  const notAppliedDm = projectLauncherOperationFailure(
    t,
    { effect: "not_applied", kind: "direct_room" },
    recover,
  );
  assert.equal(notAppliedDm.action.label, "launcher.failure.retry_dm");
  assert.equal(
    notAppliedDm.nextStep,
    "launcher.failure.direct_room_retry_next_step",
  );
  assert.equal(notAppliedDm.tone, "error");
});

test("Launcher user actions no longer fail only in the developer console", async () => {
  const [controller, page, consoleView] = await Promise.all([
    readFile(path.join(
      webRoot,
      "src/features/launcher/console/use-launcher-console-controller.ts",
    ), "utf8"),
    readFile(path.join(webRoot, "src/pages/launcher/launcher-page.tsx"), "utf8"),
    readFile(path.join(
      webRoot,
      "src/features/launcher/console/launcher-console.tsx",
    ), "utf8"),
  ]);

  assert.doesNotMatch(controller, /console\.error/);
  assert.doesNotMatch(page, /console\.error/);
  assert.match(controller, /kind: "query_read"/);
  assert.match(controller, /kind: "room_read"/);
  assert.match(controller, /projectMutationFailure/);
  assert.match(controller, /failure\.effect === "not_applied"/);
  assert.match(controller, /Launcher Query 只解析目录，不写入聊天；同一查询可以安全重试/);
  assert.match(page, /projectLauncherOperationFailure/);
  assert.match(page, /navigationFailure\.effect === "not_applied"/);
  assert.match(consoleView, /FeedbackBannerViewport/);
  assert.match(consoleView, /controller\.state\.feedback \?\? feedback/);
});
