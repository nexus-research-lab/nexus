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

function parsePathPoints(pathData) {
  return [...pathData.matchAll(/[ML](-?\d+\.\d{2}) (-?\d+\.\d{2})/g)]
    .map((match) => ({ x: Number(match[1]), y: Number(match[2]) }));
}

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
    "documents",
    "spreadsheets",
    "presentations",
    "browser-control",
    "computer-use",
    "pdf-reader",
    "template-creator",
    "data-analytics",
    "calendar",
    "email",
    "notion",
    "github",
    "cloudflare",
    "product-design",
    "frontend-design",
    "zotero",
  ].map((name) => getSeededAvatarAppearance(name).pathData);

  assert.equal(new Set(appearances).size, appearances.length);
  assert.doesNotMatch(source, /Math\.random/);
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

test("Agent 与全局技能卡共用名称种子头像", async () => {
  const [agentCard, catalogCard, sharedCard] = await Promise.all([
    "src/features/agents/options/components/skills/agent-skill-card.tsx",
    "src/features/capability/skills/catalog/skills-card.tsx",
    "src/features/capability/skills/shared/skill-directory-card.tsx",
  ].map((file) => readFile(path.join(webRoot, file), "utf8")));

  assert.match(agentCard, /UiSeededAvatar/);
  assert.match(agentCard, /seed=\{skill\.name\}/);
  assert.match(catalogCard, /SkillDirectoryCard/);
  assert.match(catalogCard, /seed=\{skill\.name\}/);
  assert.match(sharedCard, /UiSeededAvatar seed=\{seed\}/);
  assert.doesNotMatch(catalogCard, /SKILL_CARD_ICON/);
});

test("工作循环目录按稳定 slug 复用数学曲线头像", async () => {
  const source = await readFile(
    path.join(webRoot, "src/features/capability/loops/loops-directory.tsx"),
    "utf8",
  );

  assert.match(source, /<UiSeededAvatar seed=\{loop\.slug\} size="sm" \/>/);
  assert.doesNotMatch(source, /<CapabilityItemIcon>/);
});

test("头像组件静态绘制数学曲线且不引入图标字典或动画", async () => {
  const [source, generator] = await Promise.all([
    "src/shared/ui/display/seeded-avatar.tsx",
    "src/lib/seeded-avatar.ts",
  ].map((file) => readFile(path.join(webRoot, file), "utf8")));

  assert.match(source, /<svg/);
  assert.match(source, /d=\{appearance\.pathData\}/);
  assert.match(source, /SEEDED_AVATAR_RADIUS_CLASS_NAME\[size\]/);
  assert.match(source, /rounded-\[10px\]/);
  assert.doesNotMatch(source, /rounded-full/);
  assert.doesNotMatch(source, /lucide-react|requestAnimationFrame|animate-/);
  assert.match(generator, /CURVE_POINT_BUILDERS/);
  assert.match(generator, /50 \+ point\.x \* scale/);
  assert.match(generator, /50 \+ point\.y \* scale/);
  assert.doesNotMatch(generator, /centerX|centerY|requestAnimationFrame/);
});

test("能力页目录、更新和社区结果共用数学曲线身份卡", async () => {
  const [
    sharedCard,
    catalogCard,
    updateCards,
    externalCard,
    detailView,
    previewDialog,
  ] = await Promise.all([
    "src/features/capability/skills/shared/skill-directory-card.tsx",
    "src/features/capability/skills/catalog/skills-card.tsx",
    "src/features/capability/skills/catalog/skills-update-highlight.tsx",
    "src/features/capability/skills/external/external-result-card.tsx",
    "src/features/capability/skills/detail/skill-detail-view.tsx",
    "src/features/capability/skills/external/external-skill-preview-dialog.tsx",
  ].map((file) => readFile(path.join(webRoot, file), "utf8")));

  assert.match(sharedCard, /WorkspaceCatalogCard/);
  assert.match(sharedCard, /UiSeededAvatar seed=\{seed\}/);
  assert.match(sharedCard, /grid-cols-\[40px_minmax\(0,1fr\)_auto\]/);
  assert.match(sharedCard, /flex min-h-10 min-w-0 items-center/);
  assert.doesNotMatch(sharedCard, /min-w-0 pt-0\.5/);
  assert.match(sharedCard, /line-clamp-2/);
  [catalogCard, updateCards, externalCard].forEach((source) => {
    assert.match(source, /SkillDirectoryCard/);
    assert.doesNotMatch(source, /UiListRow|Puzzle/);
  });
  assert.match(detailView, /UiSeededAvatar seed=\{model\.avatarSeed\} size="lg"/);
  assert.match(previewDialog, /UiSeededAvatar seed=\{model\.avatarSeed\} size="xs"/);
});

test("已安装 Skill 卡只保留名称、说明和真实动作", async () => {
  const [sharedCard, catalogCard, catalogModel, detailView, detailModel] =
    await Promise.all([
      "src/features/capability/skills/shared/skill-directory-card.tsx",
      "src/features/capability/skills/catalog/skills-card.tsx",
      "src/features/capability/skills/catalog/skills-catalog-model.ts",
      "src/features/capability/skills/detail/skill-detail-view.tsx",
      "src/features/capability/skills/detail/skill-detail-model.ts",
    ].map((file) => readFile(path.join(webRoot, file), "utf8")));

  assert.match(sharedCard, /!meta && "min-h-\[116px\]"/);
  assert.doesNotMatch(catalogCard, /meta=\{|model\.(?:state|source|usage|visibleTags)/);
  assert.match(catalogCard, /model\.showUpdate/);
  assert.match(catalogCard, /model\.showDelete/);
  assert.doesNotMatch(
    catalogModel,
    /全局可用|系统托管|Nexus 平台库|在 Room 设置中启用|尚未启用/,
  );
  assert.doesNotMatch(
    catalogModel,
    /stateLabel|stateTone|sourceLabel|usageLabel|visibleTags/,
  );
  assert.match(detailView, /buildSkillAgentBindingPresentation/);
  assert.match(detailModel, /capability\.skills_source\.nexus_library/);
});
