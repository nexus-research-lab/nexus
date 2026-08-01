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

test("正文、Surface Header 与 Agent 表单共用响应式水平留白", async () => {
  const [surfaceHeader, surfaceView, agentOptions] = await Promise.all([
    "src/shared/ui/workspace/surface/workspace-surface-header.tsx",
    "src/shared/ui/workspace/surface/workspace-surface-view.tsx",
    "src/features/agents/options/agent-options-editor.tsx",
  ].map(readSource));

  [surfaceHeader, surfaceView, agentOptions].forEach((source) => {
    assert.match(source, /WORKSPACE_CONTENT_GUTTER_CLASS_NAME/);
  });
  assert.doesNotMatch(surfaceHeader, /px-5|xl:px-6/);
  assert.doesNotMatch(surfaceView, /px-4 py-4 sm:px-5 xl:px-6|px-5 py-2\.5 xl:px-6/);
  assert.doesNotMatch(agentOptions, /px-6 py-5|gap-2 px-6 py-3/);
});

test("管理目录在桌面统一使用三列", async () => {
  const [layout, capabilityLayout, connectors, loops, channels, ...catalogs] =
    await Promise.all([
      "src/shared/ui/layout/workspace-content-layout.ts",
      "src/features/capability/shared/capability-page-layout.tsx",
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
  assert.match(
    capabilityLayout,
    /CAPABILITY_DIRECTORY_GRID_CLASS_NAME =\s*`\$\{WORKSPACE_CATALOG_GRID_CLASS_NAME\} gap-2\.5`/,
  );
  [connectors, loops, channels].forEach((source) => {
    assert.match(source, /CAPABILITY_DIRECTORY_GRID_CLASS_NAME/);
  });
  catalogs.forEach((source) => {
    assert.match(source, /WORKSPACE_CATALOG_GRID_CLASS_NAME/);
  });
});

test("能力目录条目共用可见边框并补齐身份图标", async () => {
  const [shared, connector, connectors, loop, channel, channels, scheduled, pairing] =
    await Promise.all([
      "src/features/capability/shared/capability-page-layout.tsx",
      "src/features/capability/connectors/catalog/connector-card.tsx",
      "src/features/capability/connectors/catalog/connectors-grid.tsx",
      "src/features/capability/loops/loops-directory.tsx",
      "src/features/capability/channels/catalog/channel-card.tsx",
      "src/features/capability/channels/channels-directory.tsx",
      "src/features/capability/scheduled/board/scheduled-task-card.tsx",
      "src/features/capability/channels/pairings/pairing-list.tsx",
    ].map(readSource));

  assert.match(
    shared,
    /CAPABILITY_DIRECTORY_ROW_CLASS_NAME[\s\S]*border-\(--divider-subtle-color\)/,
  );
  assert.match(shared, /export function CapabilityItemIcon/);
  [connector, loop, channel].forEach((source) => {
    assert.match(source, /CAPABILITY_DIRECTORY_ROW_CLASS_NAME/);
  });
  [connectors, loop, channels].forEach((source) => {
    assert.match(source, /CAPABILITY_DIRECTORY_GRID_CLASS_NAME/);
    assert.doesNotMatch(source, /gap-x-8 gap-y-2/);
  });
  assert.match(loop, /<UiSeededAvatar seed=\{loop\.slug\} size="sm" \/>/);
  assert.doesNotMatch(loop, /<CapabilityItemIcon>/);
  assert.match(scheduled, /TASK_IDENTITY_ICONS/);
  assert.match(scheduled, /<CapabilityItemIcon/);
  assert.match(pairing, /<UiPanel/);
  assert.match(pairing, /<ChannelIcon type=\{item\.channel_type\}/);
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

test("记忆页使用紧凑目录、单一阅读轴和受保护删除", async () => {
  const [
    view,
    memoryApi,
    catalog,
    memoryController,
    model,
    presentation,
    header,
    documentModel,
    panel,
    indexEntries,
    styles,
    zhCapability,
    enCapability,
  ] = await Promise.all([
      "src/features/memory/agent-memory-view.tsx",
      "src/lib/api/agent/memory-api.ts",
      "src/features/memory/catalog/agent-memory-catalog.tsx",
      "src/features/memory/catalog/use-agent-memory.ts",
      "src/features/memory/catalog/memory-catalog-model.ts",
      "src/features/memory/catalog/memory-catalog-presentation.ts",
      "src/features/memory/document/memory-document-header.tsx",
      "src/features/memory/document/memory-document-model.ts",
      "src/features/memory/document/memory-document-panel.tsx",
      "src/features/memory/document/index/memory-index-entries.tsx",
      "src/features/memory/memory-view.css",
      "src/shared/i18n/catalog/zh/capability.ts",
      "src/shared/i18n/catalog/en/capability.ts",
    ].map(readSource));

  assert.doesNotMatch(view, /MemorySummary|MemoryMetrics|MemoryAgentIdentity/);
  assert.match(view, /onRefresh=\{\(\) => void memory\.resource\.refresh\(\)\}/);
  assert.match(view, /ConfirmDialog/);
  assert.match(view, /capability\.memory_delete_confirm/);
  assert.match(memoryApi, /workspace\/memory\?\$\{query\.toString\(\)\}/);
  assert.match(memoryApi, /method: "DELETE"/);
  assert.match(memoryController, /deleteAgentMemoryDocumentApi/);
  assert.match(memoryController, /deleteRequestSequenceRef/);
  assert.match(memoryController, /target\.kind === "index"/);
  assert.match(memoryController, /await refresh\(\)/);
  assert.match(catalog, /UiSearchInput/);
  assert.match(catalog, /UiSelectMenu/);
  assert.match(catalog, /UiIconButton/);
  assert.match(catalog, /SIDEBAR_SELECTION_CLASS_NAME/);
  assert.match(catalog, /action=\{\(/);
  assert.doesNotMatch(
    catalog,
    /role="tab"|formatMemoryModifiedTime|isIndexedMemoryTopic|line-clamp-2|absolute bottom-2 left-0|border-r|shrink-0 border-b/,
  );
  assert.doesNotMatch(model, /counts:|latestDocument|countMemoryDocuments|getLatestMemoryDocument/);
  assert.match(model, /snapshot\.documents\[0\]\?\.path \?\? snapshot\.index\?\.path/);
  assert.match(presentation, /ICON_BY_KEY/);
  assert.doesNotMatch(presentation, /tone:|--(?:accent|warning|primary)/);
  assert.match(header, /nexus-memory-document-content/);
  assert.match(header, /Trash2/);
  assert.doesNotMatch(header, /border-b|formatMemoryFileSize|FileText|Link2|MemoryHeaderBadge/);
  assert.doesNotMatch(documentModel, /HEADER_BADGE_RULES|badges:|memory_indexed/);
  assert.match(documentModel, /state\.documentKind !== "index"/);
  assert.match(panel, /nexus-memory-document-content/);
  assert.doesNotMatch(panel, /shrink-0 border-b/);
  assert.match(indexEntries, /nexus-memory-document-content/);
  assert.doesNotMatch(indexEntries, /Link2|divide-y|border-y|font-mono|memory_index_entries/);
  assert.match(styles, /grid-template-columns: minmax\(240px, 288px\)/);
  assert.match(styles, /column-gap: 8px/);
  assert.match(styles, /box-shadow: -8px 0 20px -18px/);
  assert.match(styles, /\.nexus-memory-document-content/);
  assert.doesNotMatch(styles, /nexus-memory-(?:summary|metrics|agent-switcher)/);
  [zhCapability, enCapability].forEach((catalogSource) => {
    assert.match(catalogSource, /capability\.memory_filter_aria/);
    assert.match(catalogSource, /capability\.memory_delete_confirm/);
    assert.match(catalogSource, /capability\.memory_delete_failed/);
    assert.doesNotMatch(
      catalogSource,
      /capability\.memory_(?:agent_aria|metric_index|metric_topics|metric_logs|metric_updated|ready|indexed|index_entries)/,
    );
  });
});

test("Agent 技能页使用紧凑响应式网格并收敛重复工具与状态", async () => {
  const [view, content, card, model, zhAgent, enAgent] = await Promise.all([
    "src/features/agents/options/components/skills/agent-options-skills-view.tsx",
    "src/features/agents/options/components/skills/agent-options-skills-content.tsx",
    "src/features/agents/options/components/skills/agent-skill-card.tsx",
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
  assert.match(
    content,
    /AGENT_SKILL_GRID_CLASS_NAME =\s*`\$\{WORKSPACE_CATALOG_GRID_CLASS_NAME\} gap-2\.5`/,
  );
  assert.equal(
    [...content.matchAll(/className=\{AGENT_SKILL_GRID_CLASS_NAME\}/g)].length,
    2,
  );
  assert.doesNotMatch(
    content,
    /count=\{`\$\{projection\.visibleAvailable\.length\}\/\$\{projection\.available\.length\}`\}/,
  );
  assert.match(card, /min-h-\[104px\]/);
  assert.match(card, /grid-cols-\[40px_minmax\(0,1fr\)_auto\]/);
  assert.match(card, /UiSeededAvatar seed=\{skill\.name\}/);
  assert.match(card, /flex min-h-10 min-w-0 items-center overflow-hidden/);
  assert.match(card, /flex min-h-10 shrink-0 items-center gap-2/);
  assert.doesNotMatch(card, /pt-0\.5/);
  assert.match(card, /px-3\.5 py-3/);
  assert.match(card, /rounded-\[10px\] border border-\(--divider-subtle-color\)/);
  assert.doesNotMatch(card, />\{actionLabel\}<\/span>/);
  assert.doesNotMatch(card, /agent_options\.skills\.enabled/);
  assert.match(card, /aria-label=\{`\$\{actionLabel\}/);
  assert.match(card, /getSkillDisplayDescription\(skill, t\)/);
  assert.doesNotMatch(model, /totalCount: number;/);
  assert.doesNotMatch(model, /totalCount: skills\.length/);
  assert.doesNotMatch(zhAgent, /agent_options\.skills\.(?:summary|total)/);
  assert.doesNotMatch(enAgent, /agent_options\.skills\.(?:summary|total)/);
  assert.doesNotMatch(zhAgent, /"agent_options\.skills\.enabled":/);
  assert.doesNotMatch(enAgent, /"agent_options\.skills\.enabled":/);
});

test("Agent 工具与联络页使用紧凑中性工作面", async () => {
  const [
    tools,
    privateView,
    privateStyles,
    privateToolbar,
    privateList,
    privateModel,
    privateTimeline,
    privateEvent,
    zhAgent,
    enAgent,
  ] = await Promise.all([
    "src/features/agents/options/components/agent-options-advanced-tab.tsx",
    "src/features/agents/private-domain/agent-private-domain-view.tsx",
    "src/features/agents/private-domain/agent-private-domain.css",
    "src/features/agents/private-domain/agent-private-domain-toolbar.tsx",
    "src/features/agents/private-domain/agent-private-domain-thread-list.tsx",
    "src/features/agents/private-domain/agent-private-domain-thread-model.ts",
    "src/features/agents/private-domain/timeline/agent-private-domain-timeline.tsx",
    "src/features/agents/private-domain/timeline/agent-private-domain-event.tsx",
    "src/shared/i18n/catalog/zh/agent.ts",
    "src/shared/i18n/catalog/en/agent.ts",
  ].map(readSource));

  assert.match(tools, /TOOL_ICONS/);
  assert.match(tools, /SIDEBAR_SELECTION_CLASS_NAME/);
  assert.match(tools, /repeat\(auto-fit,minmax\(180px,1fr\)\)/);
  assert.match(tools, /md:grid-cols-2 xl:grid-cols-3/);
  assert.match(tools, /min-h-\[64px\]/);
  assert.match(tools, /size="xs"/);
  assert.doesNotMatch(tools, /UiChoiceButton|advanced\.runtime_policy|advanced\.security_title|min-h-\[96px\]/);

  assert.match(privateView, /nexus-private-domain-layout/);
  assert.match(privateView, /title="记录"/);
  assert.doesNotMatch(privateView, /WORKSPACE_CONTENT_(?:GUTTER|MAX_WIDTH)_CLASS_NAME/);
  assert.match(privateStyles, /grid-template-columns: minmax\(240px, 288px\) minmax\(0, 1fr\)/);
  assert.match(privateStyles, /column-gap: 8px/);
  assert.match(privateStyles, /box-shadow: -8px 0 20px -18px/);
  assert.match(privateToolbar, /UiIconButton/);
  assert.match(privateToolbar, /min-h-\[48px\]/);
  assert.doesNotMatch(privateToolbar, /Handshake|border-b/);
  assert.match(privateModel, /SIDEBAR_SELECTION_CLASS_NAME/);
  assert.match(privateModel, /timestampLabel/);
  assert.doesNotMatch(privateModel, /message_count|metadataClassName|var\(--primary\)|inset_2px/);
  assert.doesNotMatch(privateList, /item\.metadata/);
  assert.match(privateTimeline, /max-w-\[920px\]/);
  assert.match(privateTimeline, /min-h-\[48px\]/);
  assert.match(privateTimeline, /nexus-private-domain-reader/);
  assert.doesNotMatch(privateEvent, />\s*私信\s*</);
  assert.doesNotMatch(privateEvent, /shadow-\[/);
  [zhAgent, enAgent].forEach((catalog) => {
    assert.doesNotMatch(
      catalog,
      /agent_options\.advanced\.(?:runtime_policy|security_title)/,
    );
  });
});
