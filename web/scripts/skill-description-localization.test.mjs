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
  ["room-playbook", "builtin", "nexus_platform"],
  ["werewolf-6p", "builtin", "nexus_platform"],
];

function createSkill(name, sourceType, sourceKind) {
  return {
    category_key: "test",
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

test("Nexus 分类按界面语言投影且不覆盖用户分类", async () => {
  const [{ getSkillCategoryLabel }, { MESSAGES }] = await Promise.all([
    server.ssrLoadModule("/src/lib/skill-category.ts"),
    server.ssrLoadModule("/src/shared/i18n/messages.ts"),
  ]);
  const known = {
    category_key: "content-docs",
    category_name: "内容与文档",
  };
  const custom = {
    category_key: "customer-playbooks",
    category_name: "Customer Playbooks",
  };

  assert.equal(
    getSkillCategoryLabel(known, (key) => MESSAGES.en[key]),
    "Content & docs",
  );
  assert.equal(
    getSkillCategoryLabel(known, (key) => MESSAGES.zh[key]),
    "内容与文档",
  );
  assert.equal(
    getSkillCategoryLabel(custom, (key) => MESSAGES.en[key]),
    "Customer Playbooks",
  );
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
  const skill = createSkill("wechat-article-search", "builtin", "nexus_platform");
  const projection = projectAgentSkills(
    [skill],
    "公众号",
    (item) => getSkillDisplayDescription(
      item,
      (key) => MESSAGES.zh[key],
    ),
  );

  assert.deepEqual(
    projection.visibleAvailable.map((item) => item.name),
    ["wechat-article-search"],
  );
});

test("同名非平台 Skill 保留自己的真实说明", async () => {
  const { getSkillDisplayDescription } = await server.ssrLoadModule(
    "/src/lib/skill-description.ts",
  );
  const userSkill = createSkill("ima-skill", "builtin", "user_global");
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
  const [detailModel, messagesModule] = await Promise.all([
    server.ssrLoadModule(
      "/src/features/capability/skills/detail/skill-detail-model.ts",
    ),
    server.ssrLoadModule("/src/shared/i18n/messages.ts"),
  ]);
  const { buildSkillDetailPresentation } = detailModel;
  const t = (key, params) => messagesModule.MESSAGES.en[key].replace(
    /\{(\w+)\}/g,
    (match, name) => params?.[name] ?? match,
  );
  const systemSkill = {
    ...createSkill("imagegen", "system"),
    category_key: "system-builtins",
    category_name: "系统内置",
    deletable: false,
    has_update: false,
    readme_markdown: "",
    scope: "any",
    source_ref: "",
    version: "system",
  };

  const systemBadges = buildSkillDetailPresentation(
    systemSkill,
    systemSkill.description,
    { t },
  ).badges;
  assert.deepEqual(
    systemBadges.map((badge) => badge.label),
    ["System built-ins", "Version system"],
  );

  const platformSkill = {
    ...systemSkill,
    category_key: "design-frontend",
    category_name: "设计与前端",
    source_kind: "nexus_platform",
    source_type: "builtin",
  };
  const platformBadges = buildSkillDetailPresentation(
    platformSkill,
    platformSkill.description,
    { t },
  ).badges;
  assert.deepEqual(
    platformBadges.map((badge) => badge.label),
    ["Design & frontend", "Nexus library", "Version system"],
  );
});

test("Skill Agent 使用状态跟随界面语言", async () => {
  const [detailModel, messagesModule] = await Promise.all([
    server.ssrLoadModule(
      "/src/features/capability/skills/detail/skill-detail-model.ts",
    ),
    server.ssrLoadModule("/src/shared/i18n/messages.ts"),
  ]);
  const t = (key, params) => messagesModule.MESSAGES.en[key].replace(
    /\{(\w+)\}/g,
    (match, name) => params?.[name] ?? match,
  );
  const binding = {
    agent_id: "agent-1",
    agent_name: "Amy",
    available: true,
    enabled: false,
    is_main: false,
  };

  assert.deepEqual(
    detailModel.buildSkillAgentBindingPresentation(binding, false, t),
    {
      description: "Independently configurable",
      status: "Enable",
      switchLabel: "Toggle Amy Skill",
    },
  );
  assert.deepEqual(
    detailModel.buildSkillAgentBindingPresentation(binding, true, t),
    {
      description: "System managed",
      status: "Disabled",
      switchLabel: "Toggle Amy Skill",
    },
  );
});
