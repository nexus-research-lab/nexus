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

test("Home ASCII 跟随显示刷新并归一化运动步长", async () => {
  const {
    HOME_ASCII_REFERENCE_FRAME_MS,
    projectHomeAsciiFrame,
  } = await server.ssrLoadModule(
    "/src/features/home/hero/home-ascii-frame-model.ts",
  );

  const previousFrameAt = 100;
  const halfFrame = projectHomeAsciiFrame({
    activeUntil: 1000,
    now: previousFrameAt + HOME_ASCII_REFERENCE_FRAME_MS / 2,
    previousFrameAt,
  }).motionScale;
  assert.ok(Math.abs(halfFrame - 0.5) < Number.EPSILON * 4);
  assert.equal(
    projectHomeAsciiFrame({
      activeUntil: 1000,
      now: previousFrameAt + HOME_ASCII_REFERENCE_FRAME_MS * 4,
      previousFrameAt,
    }).motionScale,
    2,
  );
});

test("Home ASCII 只在有限唤醒时段继续请求动画帧", async () => {
  const { projectHomeAsciiFrame } = await server.ssrLoadModule(
    "/src/features/home/hero/home-ascii-frame-model.ts",
  );

  assert.deepEqual(
    projectHomeAsciiFrame({ activeUntil: 500, now: 100, previousFrameAt: 0 }),
    { motionScale: 1, shouldContinue: true },
  );
  assert.deepEqual(
    projectHomeAsciiFrame({ activeUntil: 500, now: 600, previousFrameAt: 600 }),
    { motionScale: 0, shouldContinue: false },
  );
});
