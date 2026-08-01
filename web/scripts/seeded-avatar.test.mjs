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

test("种子头像对同一 Skill 标识保持稳定", async () => {
  const { getSeededAvatarAppearance } = await server.ssrLoadModule(
    "/src/lib/seeded-avatar.ts",
  );

  const first = getSeededAvatarAppearance("Image Generation");
  const second = getSeededAvatarAppearance(" image generation ");
  assert.deepEqual(first, second);
  assert.match(first.backgroundColor, /^#[0-9A-F]{6}$/);
  assert.match(first.foregroundColor, /^#[0-9A-F]{6}$/);
  assert.match(first.pathData, /^M\d+\.\d{2} \d+\.\d{2}( L\d+\.\d{2} \d+\.\d{2})+$/);
});

test("常见 Skill 名称能生成各自独立且非运行时随机的曲线", async () => {
  const { getSeededAvatarAppearance } = await server.ssrLoadModule(
    "/src/lib/seeded-avatar.ts",
  );
  const source = await readFile(
    path.join(webRoot, "src/lib/seeded-avatar.ts"),
    "utf8",
  );
  const appearances = [
    "image-generation",
    "goal-manager",
    "slide-maker",
    "ima-skill",
    "web-tools-guide",
    "room-collaboration",
    "linear",
    "skill-creator",
  ].map((name) => getSeededAvatarAppearance(name).pathData);

  assert.equal(new Set(appearances).size, appearances.length);
  assert.doesNotMatch(source, /Math\.random/);
});

test("Agent 与全局技能卡共用名称种子头像", async () => {
  const [agentCard, catalogCard] = await Promise.all([
    "src/features/agents/options/components/skills/agent-skill-card.tsx",
    "src/features/capability/skills/catalog/skills-card.tsx",
  ].map((file) => readFile(path.join(webRoot, file), "utf8")));

  [agentCard, catalogCard].forEach((source) => {
    assert.match(source, /UiSeededAvatar/);
    assert.match(source, /seed=\{skill\.name\}/);
  });
  assert.doesNotMatch(catalogCard, /SKILL_CARD_ICON/);
});

test("头像组件静态绘制数学曲线且不引入图标字典或动画", async () => {
  const source = await readFile(
    path.join(webRoot, "src/shared/ui/display/seeded-avatar.tsx"),
    "utf8",
  );

  assert.match(source, /<svg/);
  assert.match(source, /d=\{appearance\.pathData\}/);
  assert.doesNotMatch(source, /lucide-react|requestAnimationFrame|animate-/);
});
