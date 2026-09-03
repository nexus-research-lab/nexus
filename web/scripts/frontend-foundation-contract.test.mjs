import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

import { importLeafTypeScriptModule } from "./import-leaf-typescript-module.mjs";

const webRoot = fileURLToPath(new URL("..", import.meta.url));
const srcRoot = path.join(webRoot, "src");
const productUiRoots = [
  path.join(srcRoot, "features"),
  path.join(srcRoot, "pages"),
];

const PROHIBITED_PRODUCT_STYLE_PATTERNS = [
  {
    label: "arbitrary shadow",
    pattern: /(?:drop-)?shadow-\[[^\]]+\]/,
  },
  {
    label: "arbitrary numeric z-index",
    pattern: /\bz-\[\d+\]/,
  },
  {
    label: "arbitrary app typography scale",
    pattern: /\btext-\[(?:10|11|12|13|14|15|16|17|19|20|22|24|28|36)px\]/,
  },
];

const REQUIRED_SHARED_UI_BEHAVIOR_SUITES = [
  "src/features/conversation/shared/execution/execution-process-panel.test.tsx",
  "src/shared/ui/button/button.test.tsx",
  "src/shared/ui/dialog/decision/decision-dialog.test.tsx",
  "src/shared/ui/dialog/dialog.test.tsx",
  "src/shared/ui/display/display.test.tsx",
  "src/shared/ui/feedback/feedback.test.tsx",
  "src/shared/ui/form/form-controls.test.tsx",
  "src/shared/ui/list/list.test.tsx",
  "src/shared/ui/markdown/mermaid/mermaid-view-parts.test.tsx",
  "src/shared/ui/menu/menu.test.tsx",
  "src/shared/ui/navigation/tabs.test.tsx",
  "src/shared/ui/onboarding/overlay/tour-overlay-card.test.tsx",
  "src/shared/ui/overlay/tooltip.test.tsx",
  "src/shared/ui/panel.test.tsx",
  "src/shared/ui/sidebar/sidebar-empty-guide.test.tsx",
  "src/shared/ui/workspace/controls/workspace-conversation-tabs.test.tsx",
  "src/shared/ui/workspace/surface/workspace-task-strip.test.tsx",
];

async function readSource(relativePath) {
  return readFile(path.join(webRoot, relativePath), "utf8");
}

async function collectSourceFiles(directory) {
  const entries = await readdir(directory, { withFileTypes: true });
  const nested = await Promise.all(entries.map(async (entry) => {
    const target = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      return collectSourceFiles(target);
    }
    return /\.(?:css|ts|tsx)$/.test(entry.name) ? [target] : [];
  }));
  return nested.flat();
}

test("semantic overlay layers preserve the current visual stack without exposing integers", async () => {
  const { getUiOverlayLayerClassName } = await importLeafTypeScriptModule(
    webRoot,
    "src/shared/ui/overlay/layer-styles.ts",
  );
  const [dialog, recipes] = await Promise.all([
    readSource("src/shared/ui/dialog/dialog.tsx"),
    readSource("src/app/styles/theme-recipes.css"),
  ]);

  assert.deepEqual(
    [
      "selectMenu",
      "actionMenu",
      "popover",
      "feedback",
      "dialogUnderlay",
      "dialog",
      "dialogNested",
      "dialogInteraction",
      "tooltip",
      "tour",
      "tourDialog",
      "systemDialog",
    ].map((layer) => getUiOverlayLayerClassName(layer)),
    [
      "ui-layer-select-menu",
      "ui-layer-action-menu",
      "ui-layer-popover",
      "ui-layer-feedback",
      "ui-layer-dialog-underlay",
      "ui-layer-dialog",
      "ui-layer-dialog-nested",
      "ui-layer-dialog-interaction",
      "ui-layer-tooltip",
      "ui-layer-tour",
      "ui-layer-tour-dialog",
      "ui-layer-system-dialog",
    ],
  );
  assert.match(dialog, /layer = "dialog"/);
  assert.doesNotMatch(
    recipes,
    /\.dialog-backdrop\s*\{[^}]*\bz-index\s*:/s,
  );
});

test("dialog viewport modes expose one shared responsive geometry contract", async () => {
  const { getUiDialogViewportClassName } = await importLeafTypeScriptModule(
    webRoot,
    "src/shared/ui/dialog/dialog-layout.ts",
  );

  assert.equal(getUiDialogViewportClassName("content"), "");
  assert.equal(
    getUiDialogViewportClassName("compact"),
    "ui-dialog-viewport-compact",
  );
  assert.equal(
    getUiDialogViewportClassName("compactMax"),
    "ui-dialog-viewport-compact-max",
  );
  assert.equal(
    getUiDialogViewportClassName("adaptive"),
    "ui-dialog-viewport-adaptive",
  );
  assert.equal(
    getUiDialogViewportClassName("adaptiveMax"),
    "ui-dialog-viewport-adaptive-max",
  );
  assert.equal(
    getUiDialogViewportClassName("visualPreview"),
    "ui-dialog-viewport-visual-preview",
  );
  assert.equal(
    getUiDialogViewportClassName("documentPreview"),
    "ui-dialog-viewport-document-preview",
  );
  assert.equal(
    getUiDialogViewportClassName("workbench"),
    "ui-dialog-viewport-workbench",
  );
});

test("product source does not reintroduce numeric high layers or shared dialog viewport formulas", async () => {
  const files = await collectSourceFiles(srcRoot);
  const violations = [];
  for (const file of files) {
    const source = await readFile(file, "utf8");
    const relativePath = path.relative(webRoot, file);
    if (/z-\[(?:120|130|140|9998|9999|10000|10020|10030|11000|11050|12000)\]/.test(source)) {
      violations.push(`${relativePath}: numeric high overlay layer`);
    }
    if (
      /\.(?:ts|tsx)$/.test(file)
      && /(?:min\((?:64dvh,\s*620px|68dvh,\s*560px|78dvh,\s*620px|84vh,\s*640px)\)|min\(620px,\s*calc\(100dvh\s*-\s*(?:72px|2rem)\)\)|min\(640px,\s*calc\(100vh\s*-\s*96px\)\)|min\(82dvh,\s*(?:680px|740px|760px)\)|(?:max-)?h-\[(?:82|84|86|88|92)d?vh\]|calc\(100dvh\s*-\s*(?:16px|2rem|32px)\)|min\(820px,\s*calc\(100dvh\s*-\s*56px\)\)|min\(94vw,\s*1440px\))/.test(source)
    ) {
      violations.push(`${relativePath}: duplicated dialog viewport formula`);
    }
    if (
      /\.(?:ts|tsx)$/.test(file)
      && /<UiDialog(?:Form)?Shell\b(?:(?!>)[\s\S]){0,500}(?:max-w|w)-\[/.test(source)
    ) {
      violations.push(`${relativePath}: feature-owned dialog width`);
    }
    if (
      relativePath !== "src/shared/ui/form/checkbox.tsx"
      && /type="checkbox"/.test(source)
    ) {
      violations.push(`${relativePath}: raw ordinary checkbox`);
    }
  }

  assert.deepEqual(violations, []);
});

test("theme recipes own the semantic layer and adaptive dialog geometry implementations", async () => {
  const [tokens, recipes] = await Promise.all([
    readSource("src/app/styles/theme-tokens.css"),
    readSource("src/app/styles/theme-recipes.css"),
  ]);

  assert.match(tokens, /--layer-dialog:\s*9999/);
  assert.match(tokens, /--layer-feedback:\s*150/);
  assert.match(tokens, /--dialog-compact-height:\s*min\(620px, calc\(100dvh - 72px\)\)/);
  assert.match(tokens, /--dialog-adaptive-height:\s*min\(82dvh, 760px\)/);
  assert.match(tokens, /--dialog-visual-preview-height:\s*min\(72dvh, 600px\)/);
  assert.match(tokens, /--dialog-document-preview-height:\s*min\(64dvh, 520px\)/);
  assert.match(tokens, /--dialog-workbench-height:\s*min\(820px, calc\(100dvh - 56px\)\)/);
  assert.match(recipes, /\.ui-layer-dialog\s*\{/);
  assert.match(recipes, /\.ui-layer-feedback\s*\{/);
  assert.match(recipes, /\.ui-dialog-viewport-compact\s*\{/);
  assert.match(recipes, /\.ui-dialog-viewport-compact-max\s*\{/);
  assert.match(recipes, /\.ui-dialog-viewport-adaptive\s*\{/);
  assert.match(recipes, /\.ui-dialog-viewport-adaptive-max\s*\{/);
  assert.match(recipes, /\.ui-dialog-viewport-visual-preview\s*\{/);
  assert.match(recipes, /\.ui-dialog-viewport-document-preview\s*\{/);
  assert.match(recipes, /\.ui-dialog-viewport-workbench\s*\{/);
  assert.match(recipes, /\.ui-dialog-size-workbench\s*\{/);
  assert.match(recipes, /\.ui-dialog-backdrop-compact\s*\{/);
  assert.match(recipes, /\.ui-type-display\s*\{/);
  assert.match(recipes, /\.ui-type-page-title\s*\{/);
  assert.match(recipes, /\.ui-type-body\s*\{/);
  assert.match(recipes, /\.ui-type-code\s*\{/);
});

test("narrow app and Room chrome share one platform-aware shell geometry", async () => {
  const [layout, appHeader, roomHeader, switcher, auxiliary, actions, recipes] = await Promise.all([
    readSource("src/shared/ui/layout/mobile-shell-header-layout.ts"),
    readSource("src/app/layout/mobile-app-page-header.tsx"),
    readSource("src/features/conversation/room/surface/mobile/room-mobile-header.tsx"),
    readSource("src/features/conversation/room/surface/mobile/room-mobile-conversation-switcher.tsx"),
    readSource("src/features/conversation/room/surface/mobile/room-mobile-auxiliary-overlay.tsx"),
    readSource("src/features/conversation/room/surface/mobile/room-mobile-actions-menu.tsx"),
    readSource("src/app/styles/theme-recipes.css"),
  ]);

  assert.match(layout, /--mobile-shell-header-height,52px/);
  assert.match(layout, /MOBILE_SHELL_HEADER_GUTTER_CLASS_NAME/);
  assert.match(layout, /MOBILE_SHELL_HEADER_OFFSET_CLASS_NAME/);
  for (const consumer of [appHeader, roomHeader]) {
    assert.match(consumer, /MOBILE_SHELL_HEADER_HEIGHT_CLASS_NAME/);
    assert.match(consumer, /MOBILE_SHELL_HEADER_GUTTER_CLASS_NAME/);
    assert.match(consumer, /<UiIconButton/);
    assert.match(consumer, /shape="round"/);
    assert.match(consumer, /getUiTypographyClassName/);
    assert.doesNotMatch(consumer, /h-\[52px\]|px-2 sm:px-3/);
  }
  assert.match(switcher, /MOBILE_SHELL_HEADER_OFFSET_CLASS_NAME/);
  assert.match(switcher, /getUiOverlayLayerClassName\("dialogUnderlay"\)/);
  assert.match(switcher, /getUiOverlayLayerClassName\("dialog"\)/);
  assert.match(switcher, /<UiListRow/);
  assert.match(switcher, /density="compact"/);
  assert.doesNotMatch(switcher, /top-\[52px\]|\bz-\d+/);
  assert.match(auxiliary, /MOBILE_SHELL_HEADER_HEIGHT_CLASS_NAME/);
  assert.match(auxiliary, /MOBILE_SHELL_HEADER_GUTTER_CLASS_NAME/);
  assert.match(auxiliary, /getUiOverlayLayerClassName\("dialog"\)/);
  assert.match(auxiliary, /data-desktop-window-drag-region/);
  assert.doesNotMatch(auxiliary, /h-\[52px\]|\bz-\d+/);
  assert.match(actions, /<UiIconButton/);
  assert.match(actions, /shape="round"/);
  assert.match(
    recipes,
    /:root\[data-desktop-platform="macos"\][\s\S]*--mobile-shell-header-height:\s*var\(--workspace-header-height\)/,
  );
});

test("Room Thread and subagent overlays reuse the narrow shell and semantic layer owners", async () => {
  const [threadOverlay, subagentOverlay, threadView, subagentList, workspaceView] = await Promise.all([
    readSource("src/features/conversation/room/surface/mobile/room-mobile-thread-overlay.tsx"),
    readSource("src/features/conversation/room/surface/mobile/room-mobile-subagent-overlay.tsx"),
    readSource("src/features/conversation/shared/thread/conversation-thread-view.tsx"),
    readSource("src/features/conversation/shared/subagent/subagent-task-list.tsx"),
    readSource("src/shared/ui/workspace/surface/workspace-surface-view.tsx"),
  ]);

  for (const overlay of [threadOverlay, subagentOverlay]) {
    assert.match(overlay, /getUiOverlayLayerClassName\("dialog"\)/);
    assert.match(overlay, /--surface-popover-background/);
    assert.doesNotMatch(overlay, /\bz-\d+/);
  }
  assert.match(subagentOverlay, /flex min-h-0 flex-col/);
  assert.match(threadView, /MOBILE_SHELL_HEADER_HEIGHT_CLASS_NAME/);
  assert.match(threadView, /MOBILE_SHELL_HEADER_GUTTER_CLASS_NAME/);
  assert.match(threadView, /<UiIconButton/);
  assert.doesNotMatch(threadView, /h-\[52px\]/);
  assert.match(subagentList, /kind: "mobile"/);
  assert.match(subagentList, /shape="round"/);
  assert.match(workspaceView, /MOBILE_SHELL_HEADER_HEIGHT_CLASS_NAME/);
  assert.match(workspaceView, /data-desktop-window-drag-region/);
});

test("Workspace Surface primitives own their semantic typography and identity shape", async () => {
  const [header, view, toolbarAction] = await Promise.all([
    readSource("src/shared/ui/workspace/surface/workspace-surface-header.tsx"),
    readSource("src/shared/ui/workspace/surface/workspace-surface-view.tsx"),
    readSource("src/shared/ui/workspace/surface/workspace-surface-toolbar-action.tsx"),
  ]);

  for (const primitive of [header, view, toolbarAction]) {
    assert.match(primitive, /getUiTypographyClassName/);
  }
  assert.match(header, /role: "pageTitle"/);
  assert.match(header, /role: "metadata"/);
  assert.match(header, /radius-control-md/);
  assert.doesNotMatch(header, /rounded-\[10px\]/);
  assert.match(view, /role: "pageTitle"/);
  assert.match(toolbarAction, /role: "caption"/);
});

test("Conversation activity chips share one semantic typography and icon-action owner", async () => {
  const [styles, tasks, execution, room, recipes] = await Promise.all([
    readSource("src/shared/ui/workspace/surface/conversation-activity-chip-styles.ts"),
    readSource("src/shared/ui/workspace/surface/workspace-task-strip.tsx"),
    readSource("src/features/conversation/shared/execution/execution-process-panel.tsx"),
    readSource("src/features/conversation/room/group/chat/panel/view/group-chat-panel-view.tsx"),
    readSource("src/app/styles/theme-recipes.css"),
  ]);

  assert.match(styles, /getUiTypographyClassName\(\{ role: "metadata" \}\)/);
  for (const consumer of [tasks, execution, room]) {
    assert.match(consumer, /getConversationActivityChipClassName/);
    assert.doesNotMatch(consumer, /className="conversation-activity-chip/);
  }
  assert.match(tasks, /role: "caption"/);
  assert.match(tasks, /<UiIconButton/);
  assert.match(execution, /<UiIconButton/);
  assert.doesNotMatch(tasks, /\btext-(?:xs|compact)\b|rounded-\[6px\]/);
  assert.doesNotMatch(execution, /rounded-\[8px\]/);
  assert.doesNotMatch(
    recipes.match(/\.conversation-activity-chip\s*\{[^}]*\}/s)?.[0] ?? "",
    /font-size|line-height/,
  );
});

test("Onboarding Tour card and target highlight consume shared typography and recipes", async () => {
  const [card, overlay, recipes] = await Promise.all([
    readSource("src/shared/ui/onboarding/overlay/tour-overlay-card.tsx"),
    readSource("src/shared/ui/onboarding/overlay/tour-overlay.tsx"),
    readSource("src/app/styles/theme-recipes.css"),
  ]);

  assert.match(card, /role: "pageTitle"/);
  assert.match(card, /role: "supporting"/);
  assert.match(card, /role: "metadata"/);
  assert.match(card, /role: "caption"/);
  assert.match(card, /<UiButton/);
  assert.doesNotMatch(
    card,
    /\btext-(?:xs|compact|md)\b|\bfont-(?:medium|semibold)\b|\bleading-(?:5|tight)\b/,
  );
  assert.match(overlay, /className="tour-target-highlight pointer-events-none absolute"/);
  assert.doesNotMatch(overlay, /shadow-\[0_0_0_9999px|rounded-\[10px\]/);
  assert.match(recipes, /\.tour-target-highlight\s*\{[^}]*box-shadow:/s);
});

test("Sidebar empty and recovery guidance consume shared typography, shape, and actions", async () => {
  const guide = await readSource("src/shared/ui/sidebar/sidebar-empty-guide.tsx");

  assert.match(guide, /getUiTypographyClassName/);
  assert.match(guide, /role: "caption"/);
  assert.match(guide, /surface-radius-md/);
  assert.match(guide, /<UiButton/);
  assert.doesNotMatch(
    guide,
    /<button\b|\btext-xs\b|\bfont-(?:medium|semibold)\b|\bleading-relaxed\b|rounded-\[12px\]/,
  );
});

test("loading indicators share one size, tone, and reduced-motion recipe", async () => {
  const [
    spinnerStyles,
    resourceState,
    decisionDialog,
    appRouter,
    desktopEntry,
    workspaceState,
    appLoading,
    mermaidParts,
    lazyMermaid,
    conversationTabs,
  ] = await Promise.all([
    readSource("src/shared/ui/display/spinner-styles.ts"),
    readSource("src/shared/ui/display/resource-state.tsx"),
    readSource("src/shared/ui/dialog/decision/decision-dialog-frame.tsx"),
    readSource("src/app/router/app-router.tsx"),
    readSource("src/app/router/desktop-entry-layout.tsx"),
    readSource("src/shared/ui/workspace/frame/workspace-loading-state.tsx"),
    readSource("src/shared/ui/layout/app-loading-screen.tsx"),
    readSource("src/shared/ui/markdown/mermaid/mermaid-view-parts.tsx"),
    readSource("src/shared/ui/markdown/mermaid/lazy-mermaid-view.tsx"),
    readSource("src/shared/ui/workspace/controls/workspace-conversation-tabs.tsx"),
  ]);

  assert.match(spinnerStyles, /SPINNER_SIZE_CLASS_MAP/);
  assert.match(spinnerStyles, /SPINNER_TONE_CLASS_MAP/);
  assert.match(spinnerStyles, /motion-reduce:animate-none/);
  for (const consumer of [
    resourceState,
    decisionDialog,
    appRouter,
    desktopEntry,
    workspaceState,
    mermaidParts,
    lazyMermaid,
    conversationTabs,
  ]) {
    assert.match(consumer, /getUiSpinnerClassName/);
    assert.doesNotMatch(consumer, /\banimate-spin\b|border-t-transparent/);
  }
  assert.match(appRouter, /useI18n/);
  assert.match(desktopEntry, /useI18n/);
  assert.match(workspaceState, /getUiTypographyClassName/);
  assert.match(workspaceState, /aria-busy="true"/);
  assert.match(appLoading, /getUiTypographyClassName/);
  assert.match(appLoading, /useI18n/);
  assert.match(appLoading, /cat-loading-static\.webp/);

  const sharedUiFiles = await collectSourceFiles(path.join(srcRoot, "shared", "ui"));
  const rawSpinnerViolations = [];
  for (const file of sharedUiFiles) {
    const relativePath = path.relative(webRoot, file);
    if (
      relativePath === "src/shared/ui/display/spinner-styles.ts"
      || /\.test\.[jt]sx?$/.test(relativePath)
    ) {
      continue;
    }
    const source = await readFile(file, "utf8");
    if (/\banimate-spin\b|border-t-transparent/.test(source)) {
      rawSpinnerViolations.push(relativePath);
    }
  }
  assert.deepEqual(rawSpinnerViolations, []);
});

test("List and Badge primitives expose semantic typography and shape instead of page overrides", async () => {
  const [listRow, badge, badgeStyles, providerModels, connectorCard, customMcpGrid] = await Promise.all([
    readSource("src/shared/ui/list/list-row.tsx"),
    readSource("src/shared/ui/display/badge.tsx"),
    readSource("src/shared/ui/display/badge-styles.ts"),
    readSource("src/features/settings/provider-settings/components/provider-settings-model-list.tsx"),
    readSource("src/features/capability/connectors/catalog/connector-card.tsx"),
    readSource("src/features/capability/connectors/custom/custom-mcp-grid.tsx"),
  ]);

  assert.match(listRow, /role: "sectionTitle"/);
  assert.match(listRow, /role: "metadata"/);
  assert.doesNotMatch(listRow, /text-base font-semibold|text-compact leading-/);
  assert.match(badge, /shape\?: UiBadgeShape/);
  assert.match(badgeStyles, /pill: "rounded-full"/);
  assert.match(badgeStyles, /rounded: "radius-control-xs"/);
  assert.match(providerModels, /<UiBadge shape="pill"/);
  assert.doesNotMatch(providerModels, /<UiBadge className="rounded-full"/);
  for (const connectorList of [connectorCard, customMcpGrid]) {
    assert.match(connectorList, /description=/);
    assert.match(connectorList, /title=/);
    assert.doesNotMatch(connectorList, /text-base font-(?:medium|semibold)/);
  }
  assert.match(connectorCard, /meta=\{<ConnectorCardBadge/);
  assert.match(customMcpGrid, /role: "code"/);
});

test("desktop hosts share viewport bounds but keep platform chrome ownership separate", async () => {
  const desktopRoot = path.join(webRoot, "..", "desktop");
  const [macWindow, windowsWindow, windowsXaml, windowsWebView, recipes] = await Promise.all([
    readFile(path.join(desktopRoot, "macos/Sources/NexusDesktop/Window/WindowManager.swift"), "utf8"),
    readFile(path.join(desktopRoot, "windows/Nexus.Desktop/Window/MainWindow.xaml.cs"), "utf8"),
    readFile(path.join(desktopRoot, "windows/Nexus.Desktop/Window/MainWindow.xaml"), "utf8"),
    readFile(path.join(desktopRoot, "windows/Nexus.Desktop/WebView/WebViewHost.cs"), "utf8"),
    readSource("src/app/styles/theme-recipes.css"),
  ]);

  for (const [macPattern, windowsPattern] of [
    [/preferredWindowSize = NSSize\(width: 1280, height: 820\)/, /PreferredWindowWidth = 1280;[\s\S]*PreferredWindowHeight = 820;/],
    [/preferredMinimumWindowSize = NSSize\(width: 360, height: 520\)/, /PreferredMinimumWindowWidth = 360;[\s\S]*PreferredMinimumWindowHeight = 520;/],
    [/compactMinimumWindowSize = NSSize\(width: 320, height: 480\)/, /CompactMinimumWindowWidth = 320;[\s\S]*CompactMinimumWindowHeight = 480;/],
    [/screenPadding: CGFloat = 48/, /ScreenPadding = 48;/],
  ]) {
    assert.match(macWindow, macPattern);
    assert.match(windowsWindow, windowsPattern);
  }

  assert.match(macWindow, /\.fullSizeContentView/);
  assert.match(macWindow, /titlebarAppearsTransparent = true/);
  assert.match(windowsXaml, /<RowDefinition Height="34"\s*\/>[\s\S]*<RowDefinition Height="\*"\s*\/>/);
  assert.match(windowsXaml, /x:Name="WebViewContainer"[\s\S]*Grid\.Row="1"/);
  assert.match(windowsWebView, /IsNonClientRegionSupportEnabled = false/);
  assert.match(recipes, /:root\[data-desktop-platform="macos"\][\s\S]*\[data-desktop-window-drag-region\]/);
  assert.doesNotMatch(recipes, /:root\[data-desktop-platform="windows"\][\s\S]*\[data-desktop-window-drag-region\]/);
});

test("floating feedback reuses shared surface, layer, and typography recipes", async () => {
  const [banner, viewport, recovery] = await Promise.all([
    readSource("src/shared/ui/feedback/feedback-banner.tsx"),
    readSource("src/shared/ui/feedback/feedback-banner-viewport.tsx"),
    readSource("src/shared/ui/feedback/recovery-summary.tsx"),
  ]);

  assert.match(banner, /surface-popover surface-radius-md/);
  assert.match(banner, /getUiTypographyClassName/);
  assert.doesNotMatch(banner, /shadow-\[/);
  assert.doesNotMatch(banner, /rounded-\[/);
  assert.match(viewport, /getUiOverlayLayerClassName\("feedback"\)/);
  assert.doesNotMatch(viewport, /\bz-(?:\d+|\[)/);
  assert.match(recovery, /getUiTypographyClassName/);
});

test("App typography exposes one typed semantic role map", async () => {
  const { getUiTypographyClassName } = await importLeafTypeScriptModule(
    webRoot,
    "src/shared/ui/typography/typography-styles.ts",
  );
  const [tokens, design] = await Promise.all([
    readSource("src/app/styles/theme-tokens.css"),
    readFile(path.join(webRoot, "..", "design.md"), "utf8"),
  ]);

  assert.deepEqual(
    [
      "display",
      "featureTitle",
      "objectTitle",
      "pageTitle",
      "sectionTitle",
      "body",
      "control",
      "supporting",
      "metadata",
      "caption",
      "overline",
      "code",
    ].map((role) => getUiTypographyClassName({ role })),
    [
      "ui-type-display",
      "ui-type-feature-title",
      "ui-type-object-title",
      "ui-type-page-title",
      "ui-type-section-title",
      "ui-type-body",
      "ui-type-control",
      "ui-type-supporting",
      "ui-type-metadata",
      "ui-type-caption",
      "ui-type-overline",
      "ui-type-code",
    ],
  );
  assert.equal(
    getUiTypographyClassName({ role: "supporting", tone: "muted", weight: "medium" }),
    "ui-type-supporting ui-type-tone-muted ui-type-weight-medium",
  );
  for (const [token, value] of [
    ["2xs", "10px"],
    ["xs", "11px"],
    ["compact", "12px"],
    ["sm", "13px"],
    ["base", "14px"],
    ["md", "16px"],
    ["lg", "20px"],
    ["xl", "24px"],
    ["2xl", "36px"],
  ]) {
    assert.match(tokens, new RegExp(`--text-${token}:\\s*${value}`));
  }
  assert.match(design, /10 \/ 11 \/ 12 \/ 13 \/ 14 \/ 16 \/ 20 \/ 24 \/ 36px/);
  assert.doesNotMatch(design, /字号阶梯：`10 \/ 11 \/ 12 \/ 13 \/ 15 \/ 17/);
});

test("settings reuse semantic typography and the shared segmented control", async () => {
  const [settingsStyles, segmentedControl, appearance, behavior, runtime] = await Promise.all([
    readSource("src/features/settings/shared/settings-panel-ui.tsx"),
    readSource("src/shared/ui/form/segmented-control.tsx"),
    readSource("src/features/settings/general/sections/settings-appearance-section.tsx"),
    readSource("src/features/settings/general/sections/settings-general-behavior-section.tsx"),
    readSource("src/features/settings/runtime/settings-runtime-section.tsx"),
  ]);

  assert.match(settingsStyles, /getUiTypographyClassName/);
  assert.doesNotMatch(settingsStyles, /SettingsSegmentedControl/);
  assert.doesNotMatch(settingsStyles, /(?:text|leading|tracking|font)-\[/);
  assert.doesNotMatch(settingsStyles, /rounded-\[/);
  assert.match(segmentedControl, /getUiTypographyClassName\(\{ role: "caption"/);
  assert.match(segmentedControl, /surface-radius-md/);
  assert.doesNotMatch(segmentedControl, /rounded-full|shadow-/);
  for (const consumer of [appearance, behavior, runtime]) {
    assert.match(consumer, /UiSegmentedControl/);
    assert.doesNotMatch(consumer, /SettingsSegmentedControl/);
  }
});

test("Settings feature code consumes shared form DOM owners", async () => {
  const files = await collectSourceFiles(path.join(srcRoot, "features", "settings"));
  const violations = [];

  for (const file of files) {
    if (!/\.(?:ts|tsx)$/.test(file)) continue;
    const source = await readFile(file, "utf8");
    if (/<(?:input|textarea|select)\b/.test(source)) {
      violations.push(path.relative(webRoot, file));
    }
  }

  assert.deepEqual(violations, []);
});

test("Settings navigation consumes shared Button and typography owners", async () => {
  const files = await collectSourceFiles(path.join(srcRoot, "features", "settings"));
  const violations = [];

  for (const file of files) {
    if (!/\.(?:ts|tsx)$/.test(file)) continue;
    const source = await readFile(file, "utf8");
    if (/<button\b/.test(source)) {
      violations.push(path.relative(webRoot, file));
    }
  }

  const [navigationPattern, buttonStyles] = await Promise.all([
    readSource("src/features/settings/shared/settings-panel-ui.tsx"),
    readSource("src/shared/ui/button/button-styles.ts"),
  ]);

  assert.deepEqual(violations, []);
  assert.match(navigationPattern, /SettingsNavigationButton/);
  assert.match(navigationPattern, /<UiButton/);
  assert.match(navigationPattern, /role: "overline"/);
  assert.match(buttonStyles, /md: "[^"]*ui-type-control"/);
  assert.match(buttonStyles, /lg: "[^"]*ui-type-control"/);
  assert.match(buttonStyles, /aria-\[current=page\]/);
});

test("Personal settings cannot redefine App typography or card shape", async () => {
  const personalRoot = path.join(srcRoot, "features", "settings", "personal");
  const files = await collectSourceFiles(personalRoot);
  const violations = [];
  const localTypographyPattern = /\b(?:text-(?:2xs|xs|compact|sm|base|md|lg|xl|2xl)|font-(?:normal|medium|semibold|bold|mono)|leading-(?:none|\d+|\[[^\]]+\])|tracking-(?:tight|wide|\[[^\]]+\])|rounded-\[[^\]]+\])/;

  for (const file of files) {
    if (!/\.(?:ts|tsx)$/.test(file)) continue;
    const source = await readFile(file, "utf8");
    if (localTypographyPattern.test(source)) {
      violations.push(path.relative(webRoot, file));
    }
  }

  const [profile, usage, password, avatar] = await Promise.all([
    readSource("src/features/settings/personal/personal-profile-section.tsx"),
    readSource("src/features/settings/personal/personal-token-usage-section.tsx"),
    readSource("src/features/settings/personal/personal-password-section.tsx"),
    readSource("src/features/settings/personal/personal-avatar-picker.tsx"),
  ]);

  assert.deepEqual(violations, []);
  for (const consumer of [profile, usage, password, avatar]) {
    assert.match(consumer, /getUiTypographyClassName/);
  }
  assert.match(profile, /<UiBadge/);
  for (const consumer of [profile, usage, password]) {
    assert.match(consumer, /SETTINGS_CARD_CLASS_NAME/);
  }
});

test("Provider settings cannot redefine App typography, badges, or shape", async () => {
  const providerRoot = path.join(srcRoot, "features", "settings", "provider-settings");
  const files = await collectSourceFiles(providerRoot);
  const violations = [];
  const localTypographyPattern = /\b(?:text-(?:2xs|xs|compact|sm|base|md|lg|xl|2xl)|font-(?:normal|medium|semibold|bold|mono)|leading-(?:none|\d+|\[[^\]]+\])|tracking-(?:tight|wide|\[[^\]]+\])|rounded-\[[^\]]+\])/;

  for (const file of files) {
    if (!/\.(?:ts|tsx)$/.test(file)) continue;
    const source = await readFile(file, "utf8");
    if (localTypographyPattern.test(source)) {
      violations.push(path.relative(webRoot, file));
    }
  }

  const consumers = await Promise.all([
    "components/provider-settings-capability-switch.tsx",
    "components/provider-settings-config-form.tsx",
    "components/provider-settings-detail-header.tsx",
    "components/provider-settings-icon.tsx",
    "components/provider-settings-model-list.tsx",
    "dialogs/provider-settings-add-model-dialog.tsx",
    "dialogs/provider-settings-delete-usage-dialog.tsx",
    "dialogs/provider-settings-model-options-dialog.tsx",
  ].map((file) => readSource(`src/features/settings/provider-settings/${file}`)));

  assert.deepEqual(violations, []);
  for (const consumer of consumers) {
    assert.match(consumer, /getUiTypographyClassName/);
  }
  for (const consumer of [consumers[1], consumers[2], consumers[4], consumers[6]]) {
    assert.match(consumer, /<UiBadge/);
  }
  assert.doesNotMatch(
    await readSource("src/features/settings/provider-settings/model/provider-settings-presentation.ts"),
    /CLASS_NAME|className/,
  );
});

test("Browser settings use shared typography, status, recovery, and shape owners", async () => {
  const browserRoot = path.join(srcRoot, "features", "settings", "browser");
  const files = await collectSourceFiles(browserRoot);
  const violations = [];
  const localTypographyPattern = /\b(?:text-(?:2xs|xs|compact|sm|base|md|lg|xl|2xl)|font-(?:normal|medium|semibold|bold|mono)|leading-(?:none|\d+|\[[^\]]+\])|tracking-(?:tight|wide|\[[^\]]+\])|rounded-\[[^\]]+\])/;

  for (const file of files) {
    if (!/\.(?:ts|tsx)$/.test(file)) continue;
    const source = await readFile(file, "utf8");
    if (localTypographyPattern.test(source)) {
      violations.push(path.relative(webRoot, file));
    }
  }

  const section = await readSource(
    "src/features/settings/browser/browser-settings-section.tsx",
  );
  assert.deepEqual(violations, []);
  assert.match(section, /getUiTypographyClassName/);
  assert.match(section, /<UiBadge showDot/);
  assert.match(section, /<UiResourceState/);
  assert.match(section, /SETTINGS_CARD_CLASS_NAME/);
  assert.match(section, /SETTINGS_SECTION_TITLE_CLASS_NAME/);
  assert.doesNotMatch(section, /statusColor|statusDot/);
});

test("standalone Settings collapse the panel to the shared rail on narrow windows", async () => {
  const [panel, navigation] = await Promise.all([
    readSource("src/features/settings/settings-panel.tsx"),
    readSource("src/features/settings/settings-sidebar-navigation.tsx"),
  ]);

  assert.match(panel, /data-settings-navigation="panel"/);
  assert.match(panel, /hidden h-full w-\[224px\][^\n]*sm:flex/);
  assert.match(panel, /data-settings-navigation="rail"/);
  assert.match(panel, /w-14[^\n]*sm:hidden/);
  assert.match(panel, /<SettingsSidebarNavigation variant="panel"/);
  assert.match(panel, /<SettingsSidebarNavigation variant="rail"/);
  assert.match(navigation, /aria-current=\{active \? "page" : undefined\}/);
});

test("Operations settings use shared typography, badges, resource states, and shapes", async () => {
  const operationsRoot = path.join(srcRoot, "features", "settings", "operations");
  const files = await collectSourceFiles(operationsRoot);
  const violations = [];
  const localTypographyPattern = /\b(?:text-(?:2xs|xs|compact|sm|base|md|lg|xl|2xl)|font-(?:normal|medium|semibold|bold|mono)|leading-(?:none|\d+|\[[^\]]+\])|tracking-(?:tight|wide|\[[^\]]+\])|rounded-\[[^\]]+\])/;

  for (const file of files) {
    if (!/\.(?:ts|tsx)$/.test(file) || file.endsWith(".test.tsx")) continue;
    const source = await readFile(file, "utf8");
    if (localTypographyPattern.test(source)) {
      violations.push(path.relative(webRoot, file));
    }
  }

  const [members, accounts, plans, projects] = await Promise.all([
    readSource("src/features/settings/operations/control-members-panel.tsx"),
    readSource("src/features/settings/operations/subscription-admin/subscription-account-view.tsx"),
    readSource("src/features/settings/operations/subscription-admin/subscription-plan-view.tsx"),
    readSource("src/features/settings/operations/project-admin/project-admin-panel.tsx"),
  ]);
  const operationsUi = [members, accounts, plans, projects].join("\n");

  assert.deepEqual(violations, []);
  assert.match(operationsUi, /getUiTypographyClassName/);
  assert.match(operationsUi, /<UiBadge/);
  assert.match(operationsUi, /<UiResourceState/);
  assert.match(operationsUi, /SETTINGS_CARD_CLASS_NAME/);
  assert.match(operationsUi, /SETTINGS_CONTROL_LABEL_CLASS_NAME/);
  assert.doesNotMatch(operationsUi, /Subscription(?:Loading|Empty)State/);
});

test("Settings app chrome does not redefine semantic typography or arbitrary radii", async () => {
  const settingsRoot = path.join(srcRoot, "features", "settings");
  const files = await collectSourceFiles(settingsRoot);
  const violations = [];
  const localTypographyPattern = /\b(?:text-(?:2xs|xs|compact|sm|base|md|lg|xl|2xl)|font-(?:normal|medium|semibold|bold|mono)|leading-(?:none|\d+|\[[^\]]+\])|tracking-(?:tight|wide|\[[^\]]+\])|rounded-\[[^\]]+\])/;

  for (const file of files) {
    if (!/\.(?:ts|tsx)$/.test(file) || file.includes(".test.")) continue;
    const source = await readFile(file, "utf8");
    if (localTypographyPattern.test(source)) {
      violations.push(path.relative(webRoot, file));
    }
  }

  assert.deepEqual(violations, []);
});

test("Capability detail chrome uses shared actions, typography, states, and shapes", async () => {
  const detailPaths = [
    "src/features/capability/connectors/detail/connector-detail-header.tsx",
    "src/features/capability/connectors/detail/connector-detail-content.tsx",
    "src/features/capability/connectors/custom/detail/custom-mcp-detail-view.tsx",
    "src/features/capability/connectors/mcp/mcp-tools-section.tsx",
    "src/features/capability/skills/detail/skill-detail-view.tsx",
  ];
  const sources = await Promise.all(detailPaths.map(readSource));
  const localTypographyPattern = /\b(?:text-(?:2xs|xs|compact|sm|base|md|lg|xl|2xl)|font-(?:normal|medium|semibold|bold|mono)|leading-(?:none|\d+|\[[^\]]+\])|tracking-(?:tight|wide|\[[^\]]+\])|rounded-\[[^\]]+\])/;

  assert.deepEqual(
    detailPaths.filter((_, index) => localTypographyPattern.test(sources[index])),
    [],
  );
  const detailChrome = sources.join("\n");
  assert.match(detailChrome, /getUiTypographyClassName/);
  assert.match(detailChrome, /<UiButton/);
  assert.match(detailChrome, /<UiLinkButton/);
  assert.match(detailChrome, /<UiBadge/);
  assert.match(detailChrome, /<UiResourceState/);
  assert.doesNotMatch(detailChrome, /<button/);
  assert.match(sources[3], /flex flex-col items-start gap-3 sm:flex-row/);
  assert.match(sources[3], /className="shrink-0"/);
});

test("Capability page chrome has one Header, typography, action, and shape owner", async () => {
  const [
    capabilityLayout,
    workspaceHeader,
    skillDetail,
    connectorDetail,
    connectorIdentity,
    customMcpDetail,
    loopDetail,
    workGraphDetail,
    workGraphDirectory,
  ] = await Promise.all([
    readSource("src/features/capability/shared/capability-page-layout.tsx"),
    readSource("src/shared/ui/layout/workspace-content-header.tsx"),
    readSource("src/features/capability/skills/detail/skill-detail-view.tsx"),
    readSource("src/features/capability/connectors/detail/connector-detail-view.tsx"),
    readSource("src/features/capability/connectors/detail/connector-detail-header.tsx"),
    readSource("src/features/capability/connectors/custom/detail/custom-mcp-detail-view.tsx"),
    readSource("src/features/capability/loops/loop-detail-view.tsx"),
    readSource("src/features/capability/workgraph-distillations/workgraph-distillation-detail.tsx"),
    readSource("src/features/capability/workgraph-distillations/workgraph-distillations-directory.tsx"),
  ]);
  const localTypographyPattern = /\b(?:text-(?:2xs|xs|compact|sm|base|md|lg|xl|2xl)|font-(?:normal|medium|semibold|bold|mono)|leading-(?:none|\d+|\[[^\]]+\])|tracking-(?:tight|wide|\[[^\]]+\])|rounded-\[[^\]]+\])/;

  assert.doesNotMatch(capabilityLayout, localTypographyPattern);
  assert.doesNotMatch(workspaceHeader, localTypographyPattern);
  assert.match(capabilityLayout, /<WorkspaceContentHeader/);
  assert.match(capabilityLayout, /createPortal/);
  assert.match(capabilityLayout, /getUiTypographyClassName/);
  assert.match(capabilityLayout, /radius-control-sm/);
  assert.match(capabilityLayout, /CapabilityDetailSplitLayout/);
  assert.match(capabilityLayout, /capability-detail-aside/);
  assert.match(capabilityLayout, /xl:grid-cols-\[minmax\(0,760px\)_minmax\(280px,360px\)\]/);
  assert.match(capabilityLayout, /CapabilityDetailSectionHeader/);
  assert.match(capabilityLayout, /CapabilityDetailPage/);
  assert.match(capabilityLayout, /CapabilityDetailIdentity/);
  assert.match(capabilityLayout, /data-slot="capability-detail-header"/);
  for (const consumer of [
    skillDetail,
    connectorDetail,
    customMcpDetail,
    loopDetail,
    workGraphDetail,
  ]) {
    assert.match(consumer, /<CapabilityDetailPage/);
    assert.doesNotMatch(consumer, /WorkspaceContentDetailHeader/);
  }
  for (const consumer of [
    skillDetail,
    connectorIdentity,
    customMcpDetail,
    loopDetail,
    workGraphDetail,
  ]) {
    assert.match(consumer, /<CapabilityDetailIdentity/);
    assert.doesNotMatch(consumer, /role: "objectTitle"/);
  }
  assert.doesNotMatch(loopDetail, /<WorkspaceContentHeader/);
  assert.match(workGraphDirectory, /detailRouteContent/);
  assert.doesNotMatch(workGraphDirectory, /className=\{selected \?/);
  assert.match(skillDetail, /<CapabilityDetailSplitLayout/);
  assert.match(skillDetail, /<CapabilityDetailSectionHeader/);
  assert.doesNotMatch(skillDetail, /max-w-\[760px\]/);
  assert.match(workspaceHeader, /getUiTypographyClassName/);
});

test("Capability auxiliary states reuse resource, list, typography, and spinner owners", async () => {
  const [
    pairings,
    pairingList,
    skillUpdates,
    channelAccounts,
    channelLogin,
    channelLoginQr,
    channelFooter,
  ] = await Promise.all([
    readSource("src/features/capability/channels/pairings-directory.tsx"),
    readSource("src/features/capability/channels/pairings/pairing-list.tsx"),
    readSource("src/features/capability/skills/catalog/skills-update-highlight.tsx"),
    readSource("src/features/capability/channels/connection/channel-accounts-panel.tsx"),
    readSource("src/features/capability/channels/connection/login/channel-login-panel.tsx"),
    readSource("src/features/capability/channels/connection/login/login-qr-code.tsx"),
    readSource("src/features/capability/channels/connection/view/channel-connect-dialog-footer.tsx"),
  ]);

  assert.match(pairings, /<UiResourceState/);
  assert.doesNotMatch(pairings, /text-base|font-(?:medium|semibold)/);
  assert.match(pairingList, /<UiPanel/);
  assert.match(pairingList, /getUiTypographyClassName/);
  assert.doesNotMatch(pairingList, /rounded-\[|text-(?:2xs|xs|sm|base|compact)|font-(?:medium|semibold)|font-mono/);
  assert.match(skillUpdates, /<UiPanel/);
  assert.match(skillUpdates, /<UiListRow/);
  assert.match(skillUpdates, /getUiTypographyClassName/);
  assert.match(skillUpdates, /getUiSpinnerClassName/);
  assert.doesNotMatch(skillUpdates, /<button\b|rounded-\[|animate-spin/);
  for (const source of [channelAccounts, channelLogin, channelLoginQr]) {
    assert.match(source, /getUiTypographyClassName/);
    assert.doesNotMatch(source, /rounded-\[|text-(?:2xs|xs|sm|base|compact)|font-(?:medium|semibold)|font-mono/);
  }
  assert.match(channelAccounts, /<UiPanel/);
  assert.match(channelAccounts, /getUiSpinnerClassName/);
  assert.match(channelLogin, /<UiPanel/);
  assert.match(channelFooter, /getUiSpinnerClassName/);
  assert.doesNotMatch(channelFooter, /animate-spin/);
});

test("Skill directory chrome reuses shared actions, states, typography, and filters", async () => {
  const [card, grid, search, header, externalCard, detail] = await Promise.all([
    readSource("src/features/capability/skills/shared/skill-directory-card.tsx"),
    readSource("src/features/capability/skills/catalog/skills-catalog-grid.tsx"),
    readSource("src/features/capability/skills/skills-search-bar.tsx"),
    readSource("src/features/capability/skills/skills-header-actions.tsx"),
    readSource("src/features/capability/skills/external/external-result-card.tsx"),
    readSource("src/features/capability/skills/detail/skill-detail-view.tsx"),
  ]);
  const localVisualPattern = /rounded-\[|text-(?:2xs|xs|sm|base|compact)|font-(?:medium|semibold)|font-mono|animate-spin/;

  assert.match(card, /<UiButton/);
  assert.match(card, /getUiTypographyClassName/);
  assert.doesNotMatch(card, /<button\b|rounded-\[|text-(?:2xs|xs|sm|base|compact)|font-(?:medium|semibold)|font-mono/);
  assert.match(grid, /<UiResourceState/);
  assert.doesNotMatch(grid, /Loader2/);
  assert.match(search, /<UiSegmentedControl/);
  assert.match(search, /<UiIconButton/);
  assert.doesNotMatch(search, /<button\b/);
  assert.match(header, /<UiIconButton/);
  assert.match(header, /getUiSpinnerClassName/);
  assert.doesNotMatch(header, /<button\b|animate-spin|rounded-\[/);
  assert.match(externalCard, /getUiSpinnerClassName/);
  assert.match(detail, /state="loading"/);
  assert.match(detail, /getUiSpinnerClassName/);
  assert.doesNotMatch(externalCard, localVisualPattern);
  assert.doesNotMatch(detail, /animate-spin/);
});

test("Channel catalog shares resource, typography, action, and brand icon owners", async () => {
  const [directory, card, channelIcon, connectorIcon, brandIcon] = await Promise.all([
    readSource("src/features/capability/channels/channels-directory.tsx"),
    readSource("src/features/capability/channels/catalog/channel-card.tsx"),
    readSource("src/features/capability/channels/channel-icon.tsx"),
    readSource("src/features/capability/connectors/connector-icon.tsx"),
    readSource("src/features/capability/shared/capability-brand-icon.tsx"),
  ]);
  const localTypographyPattern = /\b(?:text-(?:2xs|xs|compact|sm|base|md|lg|xl|2xl)|font-(?:normal|medium|semibold|bold|mono)|leading-(?:none|\d+|\[[^\]]+\])|tracking-(?:tight|wide|\[[^\]]+\])|rounded-\[[^\]]+\])/;

  assert.doesNotMatch(directory, localTypographyPattern);
  assert.doesNotMatch(card, localTypographyPattern);
  assert.match(directory, /state="loading"/);
  assert.doesNotMatch(directory, /ChannelLoadingGrid|Loader2/);
  assert.match(card, /getUiTypographyClassName/);
  assert.match(card, /<UiLinkButton/);
  assert.match(card, /<UiListActionButton/);
  assert.doesNotMatch(card, /<a\b|<button\b|list-action-styles/);
  assert.match(channelIcon, /<CapabilityBrandIcon/);
  assert.match(connectorIcon, /<CapabilityBrandIcon/);
  assert.doesNotMatch(channelIcon, /#[0-9a-f]{3,8}|bg-\[|text-white|lucide-react/i);
  assert.match(brandIcon, /var\(--text-strong\)/);
  assert.match(brandIcon, /radius-control-sm/);

  const channelSources = Array.from(
    channelIcon.matchAll(/src: "([^"]+)"/g),
    (match) => match[1],
  );
  assert.equal(channelSources.length, 6);
  assert.equal(new Set(channelSources).size, channelSources.length);
});

test("Connector catalog exposes only implemented products and derives real categories", async () => {
  const [serverCatalog, catalogHook, catalogModel, categoryModel, searchBar] = await Promise.all([
    readSource("../internal/service/connectors/catalog.go"),
    readSource("src/features/capability/connectors/controller/use-connector-catalog.ts"),
    readSource("src/features/capability/connectors/catalog/connector-catalog-model.ts"),
    readSource("src/features/capability/connectors/catalog/connectors-categories.ts"),
    readSource("src/features/capability/connectors/catalog/connectors-search-bar.tsx"),
  ]);

  assert.doesNotMatch(serverCatalog, /coming_soon|ConnectorID:\s*"outlook"|ConnectorID:\s*"gmail"/);
  assert.match(catalogHook, /getConnectorsApi\(\{ status: "available" \}\)/);
  assert.match(catalogModel, /connector\.status === "available"/);
  assert.match(catalogModel, /getAvailableConnectorCategoryKeys/);
  assert.doesNotMatch(catalogModel, /COMING_SOON|connector_section_featured/);
  assert.match(categoryModel, /getAvailableConnectorCategoryKeys/);
  assert.match(searchBar, /categoryKeys/);
  assert.doesNotMatch(searchBar, /CONNECTOR_CATEGORY_OPTIONS/);
});

test("Loop surfaces use semantic typography, badges, panels, and responsive actions", async () => {
  const loopPaths = [
    "src/features/capability/loops/loops-directory.tsx",
    "src/features/capability/loops/loop-detail-view.tsx",
  ];
  const sources = await Promise.all(loopPaths.map(readSource));
  const localTypographyPattern = /\b(?:text-(?:2xs|xs|compact|sm|base|md|lg|xl|2xl)|font-(?:normal|medium|semibold|bold|mono)|leading-(?:none|\d+|\[[^\]]+\])|tracking-(?:tight|wide|\[[^\]]+\])|rounded-\[[^\]]+\])/;

  assert.deepEqual(
    loopPaths.filter((_, index) => localTypographyPattern.test(sources[index])),
    [],
  );
  const loopChrome = sources.join("\n");
  assert.match(loopChrome, /getUiTypographyClassName/);
  assert.match(loopChrome, /<UiBadge/);
  assert.match(loopChrome, /<UiPanel/);
  assert.match(loopChrome, /<UiResourceState/);
  assert.match(sources[1], /<CapabilityDetailIdentity/);
});

test("WorkGraph capability directory and detail keep separate semantic owners", async () => {
  const workGraphPaths = [
    "src/features/capability/workgraph-distillations/workgraph-distillations-directory.tsx",
    "src/features/capability/workgraph-distillations/workgraph-distillation-detail.tsx",
  ];
  const sources = await Promise.all(workGraphPaths.map(readSource));
  const localTypographyPattern = /\b(?:text-(?:2xs|xs|compact|sm|base|md|lg|xl|2xl)|font-(?:normal|medium|semibold|bold|mono)|leading-(?:none|\d+|\[[^\]]+\])|tracking-(?:tight|wide|\[[^\]]+\])|rounded-\[[^\]]+\])/;

  assert.deepEqual(
    workGraphPaths.filter((_, index) => localTypographyPattern.test(sources[index])),
    [],
  );
  const workGraphChrome = sources.join("\n");
  assert.match(sources[0], /<WorkGraphDistillationDetail/);
  assert.match(sources[0], /getUiTypographyClassName/);
  assert.match(sources[1], /<UiButton/);
  assert.match(sources[1], /<UiPanel/);
  assert.match(sources[1], /<WorkGraphWorkflowCanvasPreview/);
  assert.doesNotMatch(workGraphChrome, /<button\b/);
  assert.match(sources[1], /<CapabilityDetailIdentity/);
});

test("product source contains no arbitrary shadows or numeric z-index values", async () => {
  const files = (await Promise.all(productUiRoots.map(collectSourceFiles))).flat();
  const violations = [];

  for (const rule of PROHIBITED_PRODUCT_STYLE_PATTERNS) {
    for (const file of files) {
      const source = await readFile(file, "utf8");
      const relativePath = path.relative(webRoot, file);
      if (rule.pattern.test(source)) {
        violations.push(`${relativePath}: ${rule.label}`);
      }
    }
  }

  assert.deepEqual(violations, []);
});

test("only shared primitive adapters consume the internal button style projection", async () => {
  const files = await collectSourceFiles(srcRoot);
  const adapters = new Set([
    "src/shared/ui/button/button.tsx",
    "src/shared/ui/dialog/dialog-styles.ts",
  ]);
  const violations = [];

  for (const file of files) {
    if (!/\.(?:ts|tsx)$/.test(file)) continue;
    const source = await readFile(file, "utf8");
    const relativePath = path.relative(webRoot, file);
    if (!adapters.has(relativePath) && /@\/shared\/ui\/button\/button-styles/.test(source)) {
      violations.push(relativePath);
    }
  }

  assert.deepEqual(violations, []);
});

test("form style projection and ordinary native selects keep explicit owners", async () => {
  const files = await collectSourceFiles(srcRoot);
  const embeddedSelectOwners = new Set([
    "src/features/conversation/room/group/chat/panel/view/room-goal-lead-control.tsx",
  ]);
  const violations = [];

  for (const file of files) {
    if (!/\.(?:ts|tsx)$/.test(file)) continue;
    const source = await readFile(file, "utf8");
    const relativePath = path.relative(webRoot, file);
    if (
      relativePath !== "src/shared/ui/form/form-control.tsx"
      && /@\/shared\/ui\/form\/form-control-styles/.test(source)
    ) {
      violations.push(`${relativePath}: internal form style import`);
    }
    if (
      !embeddedSelectOwners.has(relativePath)
      && relativePath !== "src/shared/ui/form/form-control.tsx"
      && /<select\b/.test(source)
    ) {
      violations.push(`${relativePath}: unowned native select`);
    }
  }

  assert.deepEqual(violations, []);
});

test("critical shared UI groups keep co-located DOM behavior suites", async () => {
  for (const suitePath of REQUIRED_SHARED_UI_BEHAVIOR_SUITES) {
    const source = await readSource(suitePath);
    assert.match(source, /@testing-library\/react/, suitePath);
    assert.match(source, /(?:userEvent|fireEvent)/, suitePath);
  }
});

test("the UI contract gallery stays reproducible and outside production entries", async () => {
  const [html, entry, gallery, viteConfig] = await Promise.all([
    readSource("ui-gallery.html"),
    readSource("src/entries/ui-gallery.tsx"),
    readSource("src/dev/ui-gallery/ui-contract-gallery.tsx"),
    readSource("vite.config.ts"),
  ]);

  assert.match(html, /src\/entries\/ui-gallery\.tsx/);
  assert.match(entry, /bootstrapPublicReactApp/);
  assert.match(entry, /UiContractGallery/);
  assert.doesNotMatch(viteConfig, /ui-gallery\.html/);
  for (const section of [
    "Typography hierarchy",
    "Buttons & actions",
    "Forms & selection",
    "Identity & navigation",
    "Resource states",
    "Overlay & responsive checks",
  ]) {
    assert.match(gallery, new RegExp(section));
  }
});
