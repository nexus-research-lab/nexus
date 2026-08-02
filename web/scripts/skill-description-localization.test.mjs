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

const BUNDLED_SKILLS = [
  ["imagegen", "system", undefined],
  ["goal-manager", "system", undefined],
  ["ima-skill", "builtin", "nexus_platform"],
  ["wechat-article-search", "builtin", "nexus_platform"],
  ["slide-maker", "builtin", "nexus_platform"],
  ["room-playbook", "builtin", "nexus_platform"],
  ["werewolf-6p", "builtin", "nexus_platform"],
];

function createSkill(name, sourceType, sourceKind) {
  return {
    category_name: "测试",
    description: `raw:${name}`,
    enabled_for_agent: false,
    locked: false,
    name,
    source_kind: sourceKind,
    source_type: sourceType,
    tags: [],
    title: name,
  };
}

test("Nexus 全部内置 Skill 都按界面语言投影说明", async () => {
  const [{ getSkillDisplayDescription }, { MESSAGES }] = await Promise.all([
    server.ssrLoadModule("/src/lib/skill-description.ts"),
    server.ssrLoadModule("/src/shared/i18n/messages.ts"),
  ]);

  for (const [name, sourceType, sourceKind] of BUNDLED_SKILLS) {
    const skill = createSkill(name, sourceType, sourceKind);
    const before = structuredClone(skill);
    const zhDescription = getSkillDisplayDescription(
      skill,
      (key) => MESSAGES.zh[key],
    );
    const enDescription = getSkillDisplayDescription(
      skill,
      (key) => MESSAGES.en[key],
    );

    assert.notEqual(zhDescription, skill.description, `${name} 缺少中文说明`);
    assert.notEqual(enDescription, skill.description, `${name} 缺少英文说明`);
    assert.notEqual(zhDescription, enDescription, `${name} 双语说明未区分`);
    assert.deepEqual(skill, before, `${name} 的真实元数据被修改`);
  }
});

test("Agent 搜索使用当前语言的内置 Skill 说明", async () => {
  const [
    { getSkillDisplayDescription },
    { projectAgentSkills },
    { MESSAGES },
  ] = await Promise.all([
    server.ssrLoadModule("/src/lib/skill-description.ts"),
    server.ssrLoadModule(
      "/src/features/agents/options/components/skills/agent-skills-model.ts",
    ),
    server.ssrLoadModule("/src/shared/i18n/messages.ts"),
  ]);
  const skill = createSkill("slide-maker", "builtin", "nexus_platform");
  const projection = projectAgentSkills(
    [skill],
    "演示文稿",
    (item) => getSkillDisplayDescription(
      item,
      (key) => MESSAGES.zh[key],
    ),
  );

  assert.deepEqual(
    projection.visibleAvailable.map((item) => item.name),
    ["slide-maker"],
  );
});

test("同名非平台 Skill 保留自己的真实说明", async () => {
  const { getSkillDisplayDescription } = await server.ssrLoadModule(
    "/src/lib/skill-description.ts",
  );
  const userSkill = createSkill("slide-maker", "builtin", "user_global");
  const externalSkill = createSkill("imagegen", "external", "marketplace");
  const translate = () => "不应使用的本地化说明";

  assert.equal(
    getSkillDisplayDescription(userSkill, translate),
    userSkill.description,
  );
  assert.equal(
    getSkillDisplayDescription(externalSkill, translate),
    externalSkill.description,
  );
});

test("Skill 详情不会重复展示相同的分类和来源", async () => {
  const { buildSkillDetailPresentation } = await server.ssrLoadModule(
    "/src/features/capability/skills/detail/skill-detail-model.ts",
  );
  const systemSkill = {
    ...createSkill("imagegen", "system"),
    category_name: "系统内置",
    deletable: false,
    has_update: false,
    readme_markdown: "",
    scope: "any",
    source_ref: "",
    version: "system",
  };

  const systemBadges = buildSkillDetailPresentation(systemSkill).badges;
  assert.deepEqual(
    systemBadges.map((badge) => badge.label),
    ["系统内置", "版本 system"],
  );

  const platformBadges = buildSkillDetailPresentation({
    ...systemSkill,
    category_name: "图像与设计",
    source_kind: "nexus_platform",
    source_type: "builtin",
  }).badges;
  assert.deepEqual(
    platformBadges.map((badge) => badge.label),
    ["图像与设计", "Nexus 平台库", "版本 system"],
  );
});
