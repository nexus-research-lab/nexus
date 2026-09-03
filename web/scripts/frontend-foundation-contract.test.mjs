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

const DESIGN_DEBT_RATCHETS = [
  {
    allowances: {
    },
    label: "arbitrary shadow",
    pattern: /(?:drop-)?shadow-\[[^\]]+\]/g,
  },
  {
    allowances: {
      "src/features/conversation/shared/session-navigator/conversation-session-navigator.tsx": 1,
    },
    label: "arbitrary numeric z-index",
    pattern: /\bz-\[\d+\]/g,
  },
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
    getUiDialogViewportClassName("adaptive"),
    "ui-dialog-viewport-adaptive",
  );
  assert.equal(
    getUiDialogViewportClassName("adaptiveMax"),
    "ui-dialog-viewport-adaptive-max",
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
      && /(?:min\(82dvh,\s*760px\)|calc\(100dvh\s*-\s*16px\)|min\(820px,\s*calc\(100dvh\s*-\s*56px\)\)|min\(94vw,\s*1440px\))/.test(source)
    ) {
      violations.push(`${relativePath}: duplicated dialog viewport formula`);
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
  assert.match(tokens, /--dialog-adaptive-height:\s*min\(82dvh, 760px\)/);
  assert.match(tokens, /--dialog-workbench-height:\s*min\(820px, calc\(100dvh - 56px\)\)/);
  assert.match(recipes, /\.ui-layer-dialog\s*\{/);
  assert.match(recipes, /\.ui-dialog-viewport-adaptive\s*\{/);
  assert.match(recipes, /\.ui-dialog-viewport-workbench\s*\{/);
  assert.match(recipes, /\.ui-dialog-size-workbench\s*\{/);
  assert.match(recipes, /\.ui-dialog-backdrop-compact\s*\{/);
});

test("known product design debt cannot spread to new files or exceed its baseline", async () => {
  const files = (await Promise.all(productUiRoots.map(collectSourceFiles))).flat();
  const violations = [];

  for (const ratchet of DESIGN_DEBT_RATCHETS) {
    for (const file of files) {
      const source = await readFile(file, "utf8");
      const relativePath = path.relative(webRoot, file);
      const count = source.match(ratchet.pattern)?.length ?? 0;
      const allowance = ratchet.allowances[relativePath] ?? 0;
      if (count > allowance) {
        violations.push(
          `${relativePath}: ${ratchet.label} count ${count} exceeds baseline ${allowance}`,
        );
      }
    }
  }

  assert.deepEqual(violations, []);
});
