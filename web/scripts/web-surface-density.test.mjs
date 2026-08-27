import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const readSource = (relativePath) => readFile(
  new URL(`../${relativePath}`, import.meta.url),
  "utf8",
);

test("sidebar two-line identity stays within its 40px avatar", async () => {
  const [listRow, sidebarRow] = await Promise.all([
    readSource("src/shared/ui/list/list-row.tsx"),
    readSource("src/features/home/sidebar/sidebar-list-rows.tsx"),
  ]);

  assert.match(listRow, /text-base font-semibold leading-5/);
  assert.match(listRow, /mt-0\.5 flex min-w-0 items-center/);
  assert.match(listRow, /text-compact leading-\[1\.125rem\]/);
  assert.match(sidebarRow, /relative flex h-5 w-10/);
  assert.match(sidebarRow, /nexus-sidebar-conversation-summary[^"\n]*leading-\[1\.125rem\]/);
});

test("contact and Agent directories keep decision information with the familiar sidebar fallback", async () => {
  const [contactCard, contactDirectory, contactSidebar, skillCard, skillDirectoryCard] = await Promise.all([
    readSource("src/features/contacts/contacts-agent-card.tsx"),
    readSource("src/features/contacts/contacts-directory.tsx"),
    readSource("src/features/home/sidebar/sidebar-list-rows.tsx"),
    readSource("src/features/agents/options/components/skills/agent-skill-card.tsx"),
    readSource("src/features/capability/skills/shared/skill-directory-card.tsx"),
  ]);

  assert.match(contactCard, /ContactsAgentComfortCard/);
  assert.match(contactCard, /contacts\.metadata\.permission/);
  assert.match(contactCard, /contacts\.metadata\.provider/);
  assert.match(contactCard, /contacts\.metadata\.tools/);
  assert.match(contactCard, /contacts\.metadata\.skills/);
  assert.match(contactDirectory, /contacts\.description/);
  assert.match(contactDirectory, /contacts\.new_agent_description/);
  assert.match(contactSidebar, /contact_no_description/);
  assert.match(skillCard, /getSkillDisplayTitle/);
  assert.match(skillCard, /getSkillDisplayDescription/);
  assert.doesNotMatch(skillCard, /getSkillDescription/);
  assert.match(skillDirectoryCard, /\{description \? \(/);
  assert.doesNotMatch(skillDirectoryCard, /skills_no_description/);
});

test("Agent detail keeps an explicit desktop route back to the directory", async () => {
  const [detail, navigation] = await Promise.all([
    readSource("src/features/contacts/contacts-agent-detail.tsx"),
    readSource("src/pages/contacts/orchestration/use-contacts-page-navigation.ts"),
  ]);

  assert.match(detail, /const directoryNavigation = !isCompactLayout \? \(/);
  assert.match(detail, /contacts\.back_to_directory/);
  assert.match(detail, /onBackToAgentDirectory/);
  assert.match(navigation, /const openDirectory = useCallback\(\(\) => \{/);
  assert.match(navigation, /navigate\(AppRouteBuilders\.contacts\(\)\)/);
});

test("Agent configuration keeps choice explanations and the full behavior file", async () => {
  const [advancedTab, profileEditor] = await Promise.all([
    readSource("src/features/agents/options/components/agent-options-advanced-tab.tsx"),
    readSource("src/features/agents/options/components/identity/agent-profile-file-editor.tsx"),
  ]);

  assert.match(advancedTab, /permission_control_hint/);
  assert.match(advancedTab, /security_hint/);
  assert.match(advancedTab, /connector_access_hint/);
  assert.match(advancedTab, /t\(mode\.descriptionKey\)/);
  assert.match(advancedTab, /connector\.description/);
  assert.doesNotMatch(profileEditor, /const \[isExpanded, setIsExpanded\]/);
  assert.match(profileEditor, /<TextFileEditorBody/);
  assert.match(profileEditor, /fileName=\{AGENT_PROFILE_FILE_PATH\}/);
});

test("settings keep one mobile identity and preserve decision prose", async () => {
  const [
    mobileRouteModel,
    settingsUi,
    generalSection,
    providerHeader,
    providerPanel,
    providerSidebar,
    zhSettings,
  ] = await Promise.all([
    readSource("src/app/layout/mobile-app-route-model.ts"),
    readSource("src/features/settings/shared/settings-panel-ui.tsx"),
    readSource("src/features/settings/general/settings-general-section.tsx"),
    readSource("src/features/settings/provider-settings/components/provider-settings-detail-header.tsx"),
    readSource("src/features/settings/provider-settings/provider-settings-panel.tsx"),
    readSource("src/features/settings/provider-settings/components/provider-settings-sidebar.tsx"),
    readSource("src/shared/i18n/catalog/zh/settings.ts"),
  ]);

  assert.match(mobileRouteModel, /getSettingsSectionLabelKey\(parseSettingsSection\(searchParams\)\)/);
  assert.doesNotMatch(settingsUi, /SETTINGS_ITEM_DESCRIPTION_CLASS_NAME = ".*max-sm:hidden/);
  assert.doesNotMatch(settingsUi, /SETTINGS_CONTROL_LABEL_CLASS_NAME = ".*max-sm:hidden/);
  assert.match(generalSection, /className="max-sm:hidden"/);
  assert.match(generalSection, /description=\{t\(copy\.description\)\}/);
  assert.doesNotMatch(providerHeader, /presetDescription/);
  assert.match(providerPanel, /flex-col gap-3 overflow-visible sm:flex-row/);
  assert.match(providerSidebar, /max-h-\[180px\].*sm:max-h-none/);
  assert.doesNotMatch(providerSidebar, /style=\{\{ width: 190 \}\}/);
  assert.match(zhSettings, /"settings\.runtime\.tool_search_title": "工具发现"/);
  assert.match(zhSettings, /"settings\.general\.agent_sdk_diagnostics_title": "运行诊断"/);
  assert.match(zhSettings, /"settings\.providers\.custom_provider": "自定义服务"/);
  assert.match(zhSettings, /"settings\.providers\.models": "模型"/);
});

test("memory navigation keeps readable summaries and distinguishable file identity", async () => {
  const [catalog, documentHeader, presentation] = await Promise.all([
    readSource("src/features/memory/catalog/agent-memory-catalog.tsx"),
    readSource("src/features/memory/document/memory-document-header.tsx"),
    readSource("src/features/memory/catalog/memory-catalog-presentation.ts"),
  ]);

  assert.match(presentation, /getMemoryDocumentDisplayTitle/);
  assert.match(presentation, /description && description !== document\.path/);
  assert.match(catalog, /const displayTitle = getMemoryDocumentDisplayTitle\(document\)/);
  assert.match(catalog, /showDocumentTitle/);
  assert.match(catalog, /\{document\.title\}/);
  assert.match(documentHeader, /getMemoryDocumentDisplayTitle\(document\)/);
});

test("channel account management keeps identity and current errors visible", async () => {
  const accountsPanel = await readSource(
    "src/features/capability/channels/connection/channel-accounts-panel.tsx",
  );

  assert.match(accountsPanel, /account\.user_id \|\| account\.account_id/);
  assert.match(accountsPanel, /account\.last_error/);
  assert.doesNotMatch(accountsPanel, /channelAccountDisplayName/);
});

test("personal settings omit unavailable forms and preserve usage composition", async () => {
  const [passwordSection, tokenUsageSection] = await Promise.all([
    readSource("src/features/settings/personal/personal-password-section.tsx"),
    readSource("src/features/settings/personal/personal-token-usage-section.tsx"),
  ]);

  assert.match(passwordSection, /if \(!canChange\) \{/);
  assert.match(passwordSection, /<PasswordSectionHeader canChange=\{false\}/);
  assert.match(tokenUsageSection, /TokenUsageChart|TOKEN_CHART_DEFINITIONS/);
});

test("App typography uses 14px UI tokens without shrinking reading surfaces or hit areas", async () => {
  const [
    tokens,
    base,
    recipes,
    densitySpec,
  ] = await Promise.all([
    readSource("src/app/styles/theme-tokens.css"),
    readSource("src/app/styles/theme-base.css"),
    readSource("src/app/styles/theme-recipes.css"),
    readSource("../docs/specs/web-surface-density-spec.md"),
  ]);

  assert.match(tokens, /--text-base: 14px;/);
  assert.match(tokens, /--text-md: 16px;/);
  assert.match(tokens, /--text-lg: 20px;/);
  assert.match(tokens, /--text-xl: 24px;/);
  assert.match(tokens, /--text-2xl: 36px;/);
  assert.doesNotMatch(base, /body \{[^}]*font-size: var\(--text-base\);/s);
  assert.match(recipes, /\.dialog-title \{\s+font-size: var\(--text-md\);/);
  assert.match(densitySpec, /根节点不覆盖浏览器继承字号/);
  assert.match(densitySpec, /阅读正文不得通过根字号随界面密度一起缩小/);
  assert.match(densitySpec, /触控热区仍按可达性合同保留高度/);
});
