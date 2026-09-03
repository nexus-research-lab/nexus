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
  "src/shared/ui/button/button.test.tsx",
  "src/shared/ui/dialog/decision/decision-dialog.test.tsx",
  "src/shared/ui/dialog/dialog.test.tsx",
  "src/shared/ui/display/display.test.tsx",
  "src/shared/ui/feedback/feedback.test.tsx",
  "src/shared/ui/form/form-controls.test.tsx",
  "src/shared/ui/list/list.test.tsx",
  "src/shared/ui/menu/menu.test.tsx",
  "src/shared/ui/navigation/tabs.test.tsx",
  "src/shared/ui/overlay/tooltip.test.tsx",
  "src/shared/ui/panel.test.tsx",
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
