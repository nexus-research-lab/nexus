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

function parsePathPoints(pathData) {
  return [...pathData.matchAll(/[ML](-?\d+\.\d{2}) (-?\d+\.\d{2})/g)]
    .map((match) => ({ x: Number(match[1]), y: Number(match[2]) }));
}

test("种子头像对同一 Skill 标识保持稳定", async () => {
  const {
    getSeededAvatarAppearance,
    getSeededAvatarDataUrl,
  } = await server.ssrLoadModule(
    "/src/lib/seeded-avatar.ts",
  );

  const first = getSeededAvatarAppearance("Image Generation");
  const second = getSeededAvatarAppearance(" image generation ");
  assert.deepEqual(first, second);
  assert.match(first.backgroundColor, /^#[0-9A-F]{6}$/);
  assert.match(first.foregroundColor, /^#[0-9A-F]{6}$/);
  assert.match(first.pathData, /^M\d+\.\d{2} \d+\.\d{2}( L\d+\.\d{2} \d+\.\d{2})+$/);
  assert.equal(
    getSeededAvatarDataUrl("Image Generation"),
    getSeededAvatarDataUrl(" image generation "),
  );
  assert.match(
    getSeededAvatarDataUrl("Image Generation"),
    /^data:image\/svg\+xml,/,
  );
});

test("全部曲线闭合并围绕头像圆心生成", async () => {
  const { getSeededAvatarAppearance } = await server.ssrLoadModule(
    "/src/lib/seeded-avatar.ts",
  );
  const seeds = Array.from({ length: 64 }, (_, index) => `skill-${index}`);

  seeds.forEach((seed) => {
    const points = parsePathPoints(getSeededAvatarAppearance(seed).pathData);
    const openPoints = points.slice(0, -1);
    const centroid = openPoints.reduce(
      (result, point) => ({
        x: result.x + point.x / openPoints.length,
        y: result.y + point.y / openPoints.length,
      }),
      { x: 0, y: 0 },
    );
    const first = points[0];
    const last = points.at(-1);

    assert.equal(points.length, 193);
    assert.ok(Math.abs(centroid.x - 50) <= 0.12, `${seed} x 偏离圆心`);
    assert.ok(Math.abs(centroid.y - 50) <= 0.12, `${seed} y 偏离圆心`);
    assert.ok(Math.abs(first.x - last.x) <= 0.02, `${seed} x 未闭合`);
    assert.ok(Math.abs(first.y - last.y) <= 0.02, `${seed} y 未闭合`);
    points.forEach((point) => {
      assert.ok(point.x >= 15.99 && point.x <= 84.01, `${seed} x 越界`);
      assert.ok(point.y >= 15.99 && point.y <= 84.01, `${seed} y 越界`);
    });
  });
});
