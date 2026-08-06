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
  assert.match(
    header,
    /sm:h-\[var\(--workspace-header-height,60px\)\] sm:pb-0/,
  );
  assert.match(header, /sm:h-full sm:min-h-0 sm:flex-row sm:items-center/);
  assert.match(header, /text-md font-semibold leading-5/);
  assert.match(header, /mt-0\.5 max-w-\[640px\] text-compact leading-4/);
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
  assert.match(
    recipes,
    /@media \(width >= 40rem\)[\s\S]*?\.workspace-content-header\s*\{[\s\S]*?margin-block-start: -20px/,
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

test("macOS 顶栏共用分隔线与原生红灯双轴中心", async () => {
  const [
    recipes,
    homeLayout,
    headerLayout,
    runtimeConfig,
    metrics,
    runtimeScript,
  ] =
    await Promise.all([
      readSource("src/app/styles/theme-recipes.css"),
      readSource("src/lib/layout/home-layout.ts"),
      readSource("src/shared/ui/workspace/surface/workspace-header-layout.ts"),
      readSource("src/config/desktop-runtime/runtime-config.ts"),
      readFile(
        path.join(
          webRoot,
          "../desktop/macos/Sources/NexusDesktop/Window/DesktopWindowMetrics.swift",
        ),
        "utf8",
      ),
      readFile(
        path.join(
          webRoot,
          "../desktop/macos/Sources/NexusDesktop/Bridge/DesktopRuntimeScript.swift",
        ),
        "utf8",
      ),
    ]);

  assert.doesNotMatch(
    recipes,
    /\.workspace-surface-header-inner\s*\{[^}]*padding-bottom:\s*8px/,
  );
  assert.match(recipes, /--desktop-window-close-button-center-x/);
  assert.match(recipes, /--desktop-window-close-button-center-y/);
  assert.match(recipes, /--workspace-header-height/);
  assert.match(homeLayout, /--sidebar-shell-leading-padding,4px/);
  assert.match(headerLayout, /--workspace-header-height,60px/);
  assert.match(runtimeConfig, /desktop_window_close_button_center_x/);
  assert.match(runtimeConfig, /desktop_window_close_button_center_y/);
  assert.match(runtimeConfig, /--desktop-window-close-button-center-x/);
  assert.match(runtimeConfig, /--desktop-window-close-button-center-y/);
  assert.match(metrics, /windowCloseButtonCenter/);
  assert.match(runtimeScript, /desktop_window_close_button_center_x/);
  assert.match(runtimeScript, /desktop_window_close_button_center_y/);
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

test("创建 Agent 弹窗切换栏目时保持稳定尺寸", async () => {
  const dialog = await readSource(
    "src/features/agents/options/dialog/agent-options-dialog.tsx",
  );

  assert.match(dialog, /h-\[min\(82dvh,760px\)\]/);
  assert.match(dialog, /max-sm:h-\[calc\(100dvh-16px\)\]/);
  assert.doesNotMatch(dialog, /max-h-\[min\(82dvh,760px\)\]/);
});

test("桌面更新入口在侧边栏底部直接启动原生更新", async () => {
  const [
    panel,
    footer,
    indicator,
    resource,
    bridge,
    macOSBridge,
    macOSUpdater,
    windowsBridge,
    windowsUpdater,
  ] = await Promise.all([
    ...[
      "src/features/navigation/sidebar/view/sidebar-panel.tsx",
      "src/features/navigation/sidebar/view/sidebar-utility-actions.tsx",
      "src/features/navigation/sidebar/view/sidebar-update-indicator.tsx",
      "src/features/navigation/sidebar/view/use-sidebar-update-version.ts",
      "src/lib/desktop-bridge/desktop-bridge.ts",
    ].map(readSource),
    readFile(
      path.join(
        webRoot,
        "../desktop/macos/Sources/NexusDesktop/Bridge/DesktopBridgeHandler.swift",
      ),
      "utf8",
    ),
    readFile(
      path.join(
        webRoot,
        "../desktop/macos/Sources/NexusDesktop/Update/DesktopUpdateChecker.swift",
      ),
      "utf8",
    ),
    readFile(
      path.join(
        webRoot,
        "../desktop/windows/Nexus.Desktop/Bridge/DesktopBridgeHandler.cs",
      ),
      "utf8",
    ),
    readFile(
      path.join(
        webRoot,
        "../desktop/windows/Nexus.Desktop/Update/DesktopUpdateChecker.cs",
      ),
      "utf8",
    ),
  ]);

  assert.doesNotMatch(panel, /SidebarUpdateIndicator/);
  assert.match(footer, /useSidebarUpdateVersion/);
  assert.match(footer, /<SidebarUpdateIndicator/);
  assert.match(footer, /expandedRight: props\.showLogout/);
  assert.match(indicator, /sidebar-update-indicator relative/);
  assert.match(indicator, /className=\{cn\(/);
  assert.match(indicator, /<button/);
  assert.match(indicator, /startDesktopUpdate\(\)/);
  assert.doesNotMatch(indicator, /href=|releases\/latest|target=/);
  assert.doesNotMatch(indicator, /bg-emerald|hover:bg-emerald/);
  assert.match(resource, /desktop\.update\.available/);
  assert.match(resource, /isDesktopRuntime\(\)/);
  assert.match(resource, /isDesktopBridgeAvailable\(\)/);
  assert.match(resource, /getDesktopPersistentState/);
  assert.match(bridge, /"app\.start_update"/);
  assert.match(macOSBridge, /case "app\.start_update"/);
  assert.match(macOSUpdater, /startAvailableUpdate\(\)/);
  assert.match(macOSUpdater, /await downloadAndInstallUpdate\(latest\)/);
  assert.match(windowsBridge, /"app\.start_update"/);
  assert.match(windowsUpdater, /StartAvailableUpdate\(System\.Windows\.Window owner\)/);
  assert.match(windowsUpdater, /await DownloadAndOfferInstallAsync\(owner, latest\)/);
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

test("共享搜索框使用应用本地化的清除控件", async () => {
  const searchInput = await readSource("src/shared/ui/form/form-control.tsx");

  assert.match(searchInput, /role="searchbox"/);
  assert.match(searchInput, /type="text"/);
  assert.match(searchInput, /aria-label=\{t\("common\.clear"\)\}/);
  assert.match(searchInput, /onChange\(""\)/);
  assert.doesNotMatch(searchInput, /type=\{type \?\? "search"\}/);
});

test("Liquid Glass 开关强制转发具体的可访问名称", async () => {
  const glassSwitch = await readSource(
    "src/shared/ui/liquid-glass/glass-switch.tsx",
  );

  assert.match(glassSwitch, /"aria-label": string/);
  assert.match(glassSwitch, /"aria-label": ariaLabel/);
  assert.match(glassSwitch, /aria-label=\{ariaLabel\}/);
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

test("CC Switch 入口只在桌面 Provider 设置和初始化向导中显示", async () => {
  const [panel, sidebar, controller, onboarding, dialog] = await Promise.all([
    "src/features/settings/provider-settings/provider-settings-panel.tsx",
    "src/features/settings/provider-settings/components/provider-settings-sidebar.tsx",
    "src/features/settings/provider-settings/use-provider-settings-controller.ts",
    "src/features/onboarding/provider-setup/provider-setup-dialog.tsx",
    "src/features/provider-imports/cc-switch/provider-ccswitch-dialog.tsx",
  ].map(readSource));

  assert.match(panel, /isDesktopRuntime/);
  assert.match(
    panel,
    /const canImportFromCCSwitch = visibilityScope === "private" && isDesktopRuntime\(\)/,
  );
  assert.match(panel, /showCCSwitchImport=\{canImportFromCCSwitch\}/);
  assert.doesNotMatch(panel, /ccswitch_action/);
  assert.match(sidebar, /onOpenCCSwitchImport/);
  assert.match(sidebar, /settings\.providers\.ccswitch_action/);
  assert.match(onboarding, /const canImportFromCCSwitch = isDesktopRuntime\(\)/);
  assert.match(onboarding, /supportsCCSwitch=\{canImportFromCCSwitch\}/);
  assert.match(onboarding, /onboarding\.provider_setup_ccswitch_action/);
  assert.match(onboarding, /onSynced=\{handleCCSwitchSynced\}/);
  assert.match(onboarding, /requireDefault/);
  assert.match(
    onboarding,
    /const handleCCSwitchSynced[\s\S]*?default_selection[\s\S]*?persistDefaultModelSelections[\s\S]*?setScene\("ready"\)/,
  );
  assert.match(onboarding, /default_background_model_selection: selection/);
  assert.match(controller, /setUserPreferences\(await getUserPreferencesApi\(\)\)/);
  assert.match(dialog, /const canSync = selectedSources\.size > 0 && \(!requireDefault \|\| canSetDefault\)/);
  assert.match(dialog, /requireDefault[\s\S]*?settings\.providers\.ccswitch_import_title/);
  assert.match(dialog, /selectedSources\.size > 1 \|\| selectedModelCount > 1/);
  assert.match(dialog, /!item\.can_sync \?/);
  assert.match(
    dialog,
    /className="h-\[500px\] max-h-\[calc\(100dvh-2rem\)\] !max-w-\[620px\]"/,
  );
  assert.match(dialog, /<UiDialogBody className="!min-h-0 !flex-1 p-0" scrollable>/);
});

test("联系人卡片明确展示默认模型继承状态", async () => {
  const card = await readSource("src/features/contacts/contacts-agent-card.tsx");

  assert.match(
    card,
    /agent\.options\.provider\?\.trim\(\)[\s\S]*?formatProviderLabel\(agent\.options\.provider\)[\s\S]*?agent_options\.identity\.follow_default_provider/,
  );
  assert.doesNotMatch(
    card,
    /const provider = formatProviderLabel\(agent\.options\.provider\)/,
  );
});

test("联系人卡片把权限协议值投影为本地化文案", async () => {
  const card = await readSource("src/features/contacts/contacts-agent-card.tsx");

  assert.match(card, /AGENT_PERMISSION_MODES\.find/);
  assert.match(card, /permissionMode=\{t\(permissionMode\.labelKey\)\}/);
  assert.doesNotMatch(
    card,
    /const permissionMode = agent\.options\.permission_mode \|\| "default"/,
  );
});

test("联系人卡片元数据标签跟随当前语言", async () => {
  const card = await readSource("src/features/contacts/contacts-agent-card.tsx");

  ["permission", "provider", "tools", "skills"].forEach((field) => {
    assert.match(card, new RegExp(`t\\("contacts\\.metadata\\.${field}"\\)`));
  });
  assert.doesNotMatch(card, />(?:权限|Provider|工具|Skill):?</);
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

test("Agent 技能页按自身宽度响应并收敛重复工具与状态", async () => {
  const [view, content, styles, card, model, zhAgent, enAgent] = await Promise.all([
    "src/features/agents/options/components/skills/agent-options-skills-view.tsx",
    "src/features/agents/options/components/skills/agent-options-skills-content.tsx",
    "src/features/agents/options/components/skills/agent-options-skills.css",
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
    /AGENT_SKILL_GRID_CLASS_NAME = "agent-options-skills-grid"/,
  );
  assert.doesNotMatch(content, /WORKSPACE_CATALOG_GRID_CLASS_NAME/);
  assert.equal(
    [...content.matchAll(/className=\{AGENT_SKILL_GRID_CLASS_NAME\}/g)].length,
    2,
  );
  assert.match(view, /agent-options-skills-container/);
  assert.match(styles, /container-name: agent-options-skills/);
  assert.match(styles, /container-type: inline-size/);
  assert.match(styles, /grid-template-columns: minmax\(0, 1fr\)/);
  assert.match(styles, /@container agent-options-skills \(min-width: 560px\)/);
  assert.match(styles, /grid-template-columns: repeat\(2, minmax\(0, 1fr\)\)/);
  assert.match(styles, /@container agent-options-skills \(min-width: 800px\)/);
  assert.match(styles, /grid-template-columns: repeat\(3, minmax\(0, 1fr\)\)/);
  assert.doesNotMatch(
    content,
    /count=\{`\$\{projection\.visibleAvailable\.length\}\/\$\{projection\.available\.length\}`\}/,
  );
  assert.match(card, /min-h-\[104px\]/);
  assert.match(card, /grid-cols-\[40px_minmax\(0,1fr\)_auto\]/);
  assert.match(card, /UiSeededAvatar seed=\{skill\.name\}/);
  assert.match(card, /flex min-h-10 min-w-0 items-center overflow-hidden/);
  assert.match(card, /line-clamp-2 min-w-0 text-sm font-semibold/);
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
    privateTimelineModel,
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
    "src/features/agents/private-domain/timeline/agent-private-domain-timeline-model.ts",
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
  assert.match(privateView, /title=\{t\("agent_options\.contact\.records_title"\)\}/);
  assert.match(privateView, /localization=\{localization\}/);
  assert.doesNotMatch(privateView, /WORKSPACE_CONTENT_(?:GUTTER|MAX_WIDTH)_CLASS_NAME/);
  assert.match(privateStyles, /grid-template-columns: minmax\(240px, 288px\) minmax\(0, 1fr\)/);
  assert.match(privateStyles, /column-gap: 8px/);
  assert.match(privateStyles, /box-shadow: -8px 0 20px -18px/);
  assert.match(privateToolbar, /UiIconButton/);
  assert.match(privateToolbar, /min-h-\[48px\]/);
  assert.match(privateToolbar, /aria-label=\{refreshLabel\}/);
  assert.doesNotMatch(privateToolbar, /Handshake|border-b/);
  assert.match(privateModel, /SIDEBAR_SELECTION_CLASS_NAME/);
  assert.match(privateModel, /timestampLabel/);
  assert.match(privateModel, /formatRelativeTime\(thread\.last_timestamp, localization\.locale\)/);
  assert.doesNotMatch(privateModel, /message_count|metadataClassName|var\(--primary\)|inset_2px/);
  assert.doesNotMatch(privateList, /item\.metadata/);
  assert.match(privateList, /agent_options\.contact\.empty_records/);
  assert.match(privateTimeline, /max-w-\[920px\]/);
  assert.match(privateTimeline, /min-h-\[48px\]/);
  assert.match(privateTimeline, /nexus-private-domain-reader/);
  assert.doesNotMatch(privateEvent, />\s*私信\s*</);
  assert.doesNotMatch(privateEvent, /shadow-\[/);
  [
    privateView,
    privateToolbar,
    privateList,
    privateModel,
    privateTimeline,
    privateTimelineModel,
  ].forEach((source) => {
    assert.doesNotMatch(source, /刷新联络|暂无联络记录|联络消息|选择一条联络记录|私有笔记/);
  });
  [zhAgent, enAgent].forEach((catalog) => {
    assert.match(catalog, /agent_options\.contact\.messages_title/);
    assert.doesNotMatch(
      catalog,
      /agent_options\.advanced\.(?:runtime_policy|security_title)/,
    );
    assert.doesNotMatch(
      catalog,
      /bypassPermissions|allowed_tools|disallowed_tools|\bhooks\b/,
    );
  });
});

test("共享弹窗关闭按钮跟随界面语言", async () => {
  const dialog = await readSource("src/shared/ui/dialog/dialog.tsx");

  assert.match(dialog, /useI18n/);
  assert.match(dialog, /aria-label=\{ariaLabel \?\? t\("common\.close"\)\}/);
  assert.doesNotMatch(dialog, /ariaLabel = "关闭"/);
});
