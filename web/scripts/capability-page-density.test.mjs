import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const repositoryRoot = fileURLToPath(new URL("../..", import.meta.url));

async function source(relativePath) {
  return readFile(path.join(repositoryRoot, relativePath), "utf8");
}

test("capability directories keep purpose text without duplicating mobile page identity", async () => {
  const layout = await source("web/src/features/capability/shared/capability-page-layout.tsx");
  const mobileHeader = await source("web/src/app/layout/mobile-app-page-header.tsx");
  const mobileRoutes = await source("web/src/app/layout/mobile-app-route-model.ts");
  const directories = await Promise.all([
    "web/src/features/capability/skills/skills-directory.tsx",
    "web/src/features/capability/connectors/connectors-directory.tsx",
    "web/src/features/capability/channels/channels-directory.tsx",
    "web/src/features/capability/channels/pairings-directory.tsx",
    "web/src/features/capability/scheduled/scheduled-tasks-directory.tsx",
    "web/src/features/capability/loops/loops-directory.tsx",
    "web/src/features/capability/workgraph-distillations/workgraph-distillations-directory.tsx",
  ].map(source));
  const messages = `${await source("web/src/shared/i18n/catalog/zh/capability.ts")}\n${await source("web/src/shared/i18n/catalog/en/capability.ts")}`;

  assert.match(layout, /description\?: ReactNode/);
  assert.match(layout, /useMobileAppPageHeaderActionsTarget/);
  assert.match(layout, /createPortal\(/);
  assert.match(layout, /className="max-sm:hidden"/);
  assert.match(layout, /description \? \(/);
  assert.match(layout, /sm:hidden/);
  assert.match(mobileHeader, /actionsRef/);
  assert.match(mobileHeader, /ml-auto flex shrink-0 items-center/);
  assert.match(mobileRoutes, /APP_ROUTE_PATHS\.workGraphDistillations/);
  assert.match(mobileRoutes, /"capability\.workgraph_distillations"/);
  for (const directory of directories) {
    assert.match(directory, /intro_description/);
  }
  assert.match(messages, /"capability\.(?:skills|connectors|channels|pairings|scheduled|loops|workgraph)_intro_description"/);
  assert.match(messages, /"capability\.skills_intro_title": "管理工作技能"/);
  assert.match(messages, /"capability\.connectors_intro_title": "连接工作工具"/);
  assert.match(messages, /"capability\.channels_intro_title": "接入消息频道"/);
  assert.match(messages, /"capability\.pairings_intro_title": "管理频道授权"/);
  assert.match(messages, /"capability\.scheduled_intro_title": "安排自动工作"/);
  assert.match(messages, /"capability\.loops_intro_title": "复用工作循环"/);
});

test("capability cards keep decision summaries and defer only internal identity", async () => {
  const skillCard = await source("web/src/features/capability/skills/shared/skill-directory-card.tsx");
  const skillGrid = await source("web/src/features/capability/skills/catalog/skills-catalog-grid.tsx");
  const skillCopy = await source("web/src/lib/skill-description.ts");
  const channelCard = await source("web/src/features/capability/channels/catalog/channel-card.tsx");
  const connectorCard = await source("web/src/features/capability/connectors/catalog/connector-card.tsx");
  const pairingList = await source("web/src/features/capability/channels/pairings/pairing-list.tsx");
  const pairingModel = await source("web/src/features/capability/channels/pairings/pairing-model.ts");
  const taskCard = await source("web/src/features/capability/scheduled/board/scheduled-task-card.tsx");
  const board = await source("web/src/features/capability/scheduled/board/scheduled-task-board.tsx");
  const boardModel = await source("web/src/features/capability/scheduled/board/scheduled-task-board-model.ts");

  assert.match(skillCard, /line-clamp-2/);
  assert.doesNotMatch(skillGrid, /count=\{/);
  assert.match(skillCopy, /"execution-orchestrator": "capability\.skill_title\.execution_orchestrator"/);
  assert.match(skillCopy, /"nexus-configuration": "capability\.skill_description\.nexus_configuration"/);
  assert.match(channelCard, /\{model\.description\}/);
  assert.doesNotMatch(connectorCard, /getConnectorCategoryLabel/);
  assert.match(pairingList, /pairingTarget/);
  assert.match(pairingList, /技术详情/);
  assert.match(pairingModel, /name !== item\.external_ref\.trim\(\)/);
  assert.match(taskCard, /\{task\.instruction\}/);
  assert.match(board, /suggestion\.description/);
  assert.doesNotMatch(board, /column\.emptyDescription/);
  assert.doesNotMatch(boardModel, /emptyDescription/);
});

test("capability page inventory and density rules stay documented", async () => {
  const specification = await source("docs/specs/capability-page-design-spec.md");
  assert.match(specification, /\*\*7 个能力页面\*\*/);
  assert.match(specification, /技能、连接器、频道、频道配对、定时任务、工作循环和工作图/);
  assert.match(specification, /识别、比较和决策/);
  assert.match(specification, /原始 ID、Session key、绑定键/);
});
