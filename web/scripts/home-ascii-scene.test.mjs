import assert from "node:assert/strict";
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

test("Home ASCII 活跃动画限制为 30 FPS", async () => {
  const {
    HOME_ASCII_ACTIVE_FRAME_INTERVAL_MS,
    projectHomeAsciiFrame,
  } = await server.ssrLoadModule(
    "/src/features/home/hero/home-ascii-frame-model.ts",
  );

  const lastPaintAt = 100;
  assert.equal(
    projectHomeAsciiFrame({
      activeUntil: 1000,
      lastPaintAt,
      now: lastPaintAt + HOME_ASCII_ACTIVE_FRAME_INTERVAL_MS / 2,
    }).shouldPaint,
    false,
  );
  assert.equal(
    projectHomeAsciiFrame({
      activeUntil: 1000,
      lastPaintAt,
      now: lastPaintAt + HOME_ASCII_ACTIVE_FRAME_INTERVAL_MS,
    }).shouldPaint,
    true,
  );
});

test("Home ASCII 只在有限唤醒时段继续请求动画帧", async () => {
  const { projectHomeAsciiFrame } = await server.ssrLoadModule(
    "/src/features/home/hero/home-ascii-frame-model.ts",
  );

  assert.deepEqual(
    projectHomeAsciiFrame({ activeUntil: 500, lastPaintAt: 0, now: 100 }),
    { shouldContinue: true, shouldPaint: true },
  );
  assert.deepEqual(
    projectHomeAsciiFrame({ activeUntil: 500, lastPaintAt: 100, now: 600 }),
    { shouldContinue: false, shouldPaint: true },
  );
});
