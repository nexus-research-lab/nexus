import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const webRoot = fileURLToPath(new URL("..", import.meta.url));

async function readSource(file) {
  return readFile(path.join(webRoot, file), "utf8");
}

test("能力、设置与联系人共用同一铺满管理内容面", async () => {
  const [tokens, recipes, layout, header, capability, contacts, contactDetail, ...settings] =
    await Promise.all([
      "src/app/styles/theme-tokens.css",
      "src/app/styles/theme-recipes.css",
      "src/shared/ui/layout/workspace-content-layout.ts",
      "src/shared/ui/layout/workspace-content-header.tsx",
      "src/features/capability/shared/capability-page-layout.tsx",
      "src/features/contacts/contacts-directory.tsx",
      "src/features/contacts/contacts-agent-detail.tsx",
      "src/features/settings/general/settings-general-section.tsx",
      "src/features/settings/personal/personal-settings-panel.tsx",
      "src/features/settings/provider-settings/provider-settings-panel.tsx",
      "src/features/settings/runtime/settings-runtime-section.tsx",
      "src/features/settings/operations/operations-panel.tsx",
    ].map(readSource));

  assert.doesNotMatch(tokens, /--workspace-(?:content|board)-max-width/);
  assert.doesNotMatch(tokens, /--workspace-wide-content-max-width/);
  assert.match(
    tokens,
    /--workspace-content-gutter: clamp\(20px, 2vw, 32px\)/,
  );
  assert.match(layout, /WORKSPACE_CONTENT_PAGE_CLASS_NAME/);
  assert.match(layout, /WORKSPACE_CONTENT_MAX_WIDTH_CLASS_NAME = "max-w-none"/);
  assert.match(layout, /WORKSPACE_CONTENT_GUTTER_CLASS_NAME/);
  assert.match(layout, /px-\[var\(--workspace-content-gutter\)\]/);
  assert.doesNotMatch(layout, /"px-5"|"xl:px-6"/);
  assert.match(header, /min-h-\[52px\]/);
  assert.match(header, /text-lg font-semibold/);
  assert.match(header, /workspace-content-header-inner/);
  assert.match(header, /data-desktop-window-drag-region/);
  assert.match(
    recipes,
    /\.sidebar-panel-shell\[data-sidebar-collapsed="true"\][\s\S]*?\+ \.desktop-app-stage,[\s\S]*?--workspace-content-header-leading-inset/,
  );
  assert.match(
    recipes,
    /\.workspace-content-header-inner[\s\S]*?padding-inline-start: var\(--workspace-content-header-leading-inset, 0px\)/,
  );
  assert.match(
    recipes,
    /\.workspace-content-header\s*\{[\s\S]*?margin-block-start: -12px/,
  );
  assert.match(capability, /WORKSPACE_CONTENT_PAGE_CLASS_NAME/);
  assert.match(capability, /WorkspaceContentHeader/);
  assert.doesNotMatch(capability, /max-w-\[1240px\]/);
  assert.match(contacts, /WORKSPACE_CONTENT_PAGE_CLASS_NAME/);
  assert.match(contacts, /WorkspaceContentHeader/);
  assert.doesNotMatch(contacts, /WorkspaceSurfaceHeader/);
  assert.doesNotMatch(contacts, /2xl:grid-cols-4/);
  assert.match(contactDetail, /WORKSPACE_CONTENT_MAX_WIDTH_CLASS_NAME/);
  settings.forEach((source) => {
    assert.match(source, /WORKSPACE_CONTENT_PAGE_CLASS_NAME/);
  });
});

test("定时任务在铺满内容面内保持四列横向看板", async () => {
  const [layout, directory, board] = await Promise.all([
    "src/shared/ui/layout/workspace-content-layout.ts",
    "src/features/capability/scheduled/scheduled-tasks-directory.tsx",
    "src/features/capability/scheduled/board/scheduled-task-board.tsx",
  ].map(readSource));

  assert.doesNotMatch(layout, /WORKSPACE_BOARD_MAX_WIDTH_CLASS_NAME/);
  assert.doesNotMatch(directory, /WORKSPACE_BOARD_MAX_WIDTH_CLASS_NAME/);
  assert.doesNotMatch(directory, /bodyScrollable/);
  assert.match(board, /WORKSPACE_CONTENT_BLEED_CLASS_NAME/);
  assert.doesNotMatch(board, /-mx-5|xl:-mx-6|xl:px-6/);
  assert.match(board, /min-w-\[1080px\]/);
  assert.match(board, /grid-cols-4/);
  assert.match(board, /overflow-x-auto/);
  assert.doesNotMatch(board, /grid-cols-2/);
});

test("正文、Surface Header 与 Agent 详情共用响应式水平留白", async () => {
  const [surfaceHeader, surfaceView, agentOptions, privateDomain] = await Promise.all([
    "src/shared/ui/workspace/surface/workspace-surface-header.tsx",
    "src/shared/ui/workspace/surface/workspace-surface-view.tsx",
    "src/features/agents/options/agent-options-editor.tsx",
    "src/features/agents/private-domain/agent-private-domain-view.tsx",
  ].map(readSource));

  [surfaceHeader, surfaceView, agentOptions, privateDomain].forEach((source) => {
    assert.match(source, /WORKSPACE_CONTENT_GUTTER_CLASS_NAME/);
  });
  assert.doesNotMatch(surfaceHeader, /px-5|xl:px-6/);
  assert.doesNotMatch(surfaceView, /px-4 py-4 sm:px-5 xl:px-6|px-5 py-2\.5 xl:px-6/);
  assert.doesNotMatch(agentOptions, /px-6 py-5|gap-2 px-6 py-3/);
  assert.doesNotMatch(privateDomain, /px-5 py-5 xl:px-6/);
});

test("管理目录在桌面统一使用三列", async () => {
  const [layout, ...catalogs] = await Promise.all([
    "src/shared/ui/layout/workspace-content-layout.ts",
    "src/features/capability/connectors/catalog/connectors-grid.tsx",
    "src/features/capability/loops/loops-directory.tsx",
    "src/features/capability/channels/channels-directory.tsx",
    "src/features/capability/skills/catalog/skills-catalog-grid.tsx",
    "src/features/capability/skills/catalog/skills-update-highlight.tsx",
    "src/features/capability/skills/external/skills-external-results.tsx",
    "src/features/capability/scheduled/board/scheduled-task-board.tsx",
    "src/features/contacts/contacts-directory.tsx",
  ].map(readSource));

  assert.match(
    layout,
    /WORKSPACE_CATALOG_GRID_CLASS_NAME[\s\S]*grid-cols-1 md:grid-cols-2 xl:grid-cols-3/,
  );
  catalogs.forEach((source) => {
    assert.match(source, /WORKSPACE_CATALOG_GRID_CLASS_NAME/);
  });
});

test("全部能力目录复用同一正文标题与内容轴", async () => {
  const directories = await Promise.all([
    "src/features/capability/skills/skills-directory.tsx",
    "src/features/capability/loops/loops-directory.tsx",
    "src/features/capability/connectors/connectors-directory.tsx",
    "src/features/capability/scheduled/scheduled-tasks-directory.tsx",
    "src/features/capability/channels/channels-directory.tsx",
    "src/features/capability/channels/pairings-directory.tsx",
  ].map(readSource));

  directories.forEach((source) => {
    assert.match(source, /CapabilityPageLayout/);
    assert.doesNotMatch(source, /WorkspaceSurfaceHeader/);
    assert.doesNotMatch(source, /max-w-\[(?:960|980)px\]/);
  });
});

test("设置分区和联系人目录复用同一 Header 几何", async () => {
  const pages = await Promise.all([
    "src/features/contacts/contacts-directory.tsx",
    "src/features/settings/general/settings-general-section.tsx",
    "src/features/settings/personal/personal-settings-panel.tsx",
    "src/features/settings/provider-settings/provider-settings-panel.tsx",
    "src/features/settings/runtime/settings-runtime-section.tsx",
    "src/features/settings/operations/operations-panel.tsx",
  ].map(readSource));

  pages.forEach((source) => {
    assert.match(source, /WorkspaceContentHeader/);
    assert.match(source, /WORKSPACE_CONTENT_PAGE_CLASS_NAME/);
  });
});

test("设置和能力二级页不再恢复重复标题或私有版心", async () => {
  const [settingsPanel, operations, subscription, projects, loopDetail] = await Promise.all([
    "src/features/settings/settings-panel.tsx",
    "src/features/settings/operations/operations-panel.tsx",
    "src/features/settings/operations/subscription-admin/subscription-admin-panel.tsx",
    "src/features/settings/operations/project-admin/project-admin-panel.tsx",
    "src/features/capability/loops/loop-detail-view.tsx",
  ].map(readSource));

  assert.doesNotMatch(settingsPanel, /WorkspaceSurfaceHeader/);
  assert.doesNotMatch(operations, /WorkspaceSurfaceHeader/);
  assert.match(operations, /WorkspaceContentHeader/);
  assert.ok(
    operations.indexOf("<WorkspaceContentHeader") < operations.indexOf("<UiTabs"),
    "运营固定标题必须位于切换按钮上方",
  );
  assert.match(operations, /UiTabs/);
  assert.match(operations, /WORKSPACE_CONTENT_PAGE_CLASS_NAME/);
  assert.doesNotMatch(subscription, /WorkspaceContentHeader/);
  assert.doesNotMatch(subscription, /WORKSPACE_CONTENT_PAGE_CLASS_NAME/);
  assert.doesNotMatch(projects, /WorkspaceContentHeader/);
  assert.doesNotMatch(projects, /WORKSPACE_CONTENT_PAGE_CLASS_NAME/);
  assert.match(loopDetail, /WorkspaceContentHeader/);
  assert.match(loopDetail, /WORKSPACE_CONTENT_PAGE_CLASS_NAME/);
  assert.doesNotMatch(loopDetail, /max-w-\[960px\]/);
});

test("Agent 技能页直接使用分组，不重复汇总与手动刷新", async () => {
  const [view, content, model, zhAgent, enAgent] = await Promise.all([
    "src/features/agents/options/components/skills/agent-options-skills-view.tsx",
    "src/features/agents/options/components/skills/agent-options-skills-content.tsx",
    "src/features/agents/options/components/skills/agent-skills-model.ts",
    "src/shared/i18n/catalog/zh/agent.ts",
    "src/shared/i18n/catalog/en/agent.ts",
  ].map(readSource));

  assert.doesNotMatch(view, /SkillsHeader|RefreshCw|UiIconButton/);
  assert.doesNotMatch(view, /agent_options\.skills\.(?:summary|total)/);
  assert.match(content, /EnabledSkillsSection/);
  assert.match(content, /AvailableSkillsSection/);
  assert.match(content, /sm:flex-row sm:items-center sm:justify-between/);
  assert.match(content, /className="min-w-0 flex-1 sm:w-\[288px\] sm:flex-none"/);
  assert.match(content, /const filteredCount = searchQuery\.trim\(\)/);
  assert.match(content, /filteredCount \? \(/);
  assert.doesNotMatch(
    content,
    /count=\{`\$\{projection\.visibleAvailable\.length\}\/\$\{projection\.available\.length\}`\}/,
  );
  assert.doesNotMatch(model, /totalCount: number;/);
  assert.doesNotMatch(model, /totalCount: skills\.length/);
  assert.doesNotMatch(zhAgent, /agent_options\.skills\.(?:summary|total)/);
  assert.doesNotMatch(enAgent, /agent_options\.skills\.(?:summary|total)/);
});
