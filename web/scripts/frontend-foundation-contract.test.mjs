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
];

const REQUIRED_SHARED_UI_BEHAVIOR_SUITES = [
  "src/shared/ui/button/button.test.tsx",
  "src/shared/ui/dialog/decision/decision-dialog.test.tsx",
  "src/shared/ui/dialog/dialog.test.tsx",
  "src/shared/ui/display/display.test.tsx",
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
  assert.match(tokens, /--dialog-compact-height:\s*min\(620px, calc\(100dvh - 72px\)\)/);
  assert.match(tokens, /--dialog-adaptive-height:\s*min\(82dvh, 760px\)/);
  assert.match(tokens, /--dialog-visual-preview-height:\s*min\(72dvh, 600px\)/);
  assert.match(tokens, /--dialog-document-preview-height:\s*min\(64dvh, 520px\)/);
  assert.match(tokens, /--dialog-workbench-height:\s*min\(820px, calc\(100dvh - 56px\)\)/);
  assert.match(recipes, /\.ui-layer-dialog\s*\{/);
  assert.match(recipes, /\.ui-dialog-viewport-compact\s*\{/);
  assert.match(recipes, /\.ui-dialog-viewport-compact-max\s*\{/);
  assert.match(recipes, /\.ui-dialog-viewport-adaptive\s*\{/);
  assert.match(recipes, /\.ui-dialog-viewport-adaptive-max\s*\{/);
  assert.match(recipes, /\.ui-dialog-viewport-visual-preview\s*\{/);
  assert.match(recipes, /\.ui-dialog-viewport-document-preview\s*\{/);
  assert.match(recipes, /\.ui-dialog-viewport-workbench\s*\{/);
  assert.match(recipes, /\.ui-dialog-size-workbench\s*\{/);
  assert.match(recipes, /\.ui-dialog-backdrop-compact\s*\{/);
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
    "Buttons & actions",
    "Forms & selection",
    "Identity & navigation",
    "Resource states",
    "Overlay & responsive checks",
  ]) {
    assert.match(gallery, new RegExp(section));
  }
});
