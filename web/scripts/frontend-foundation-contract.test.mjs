import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

import { importLeafTypeScriptModule } from "./import-leaf-typescript-module.mjs";

const webRoot = fileURLToPath(new URL("..", import.meta.url));
const srcRoot = path.join(webRoot, "src");

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
});

test("product source does not reintroduce numeric high layers or shared dialog viewport formulas", async () => {
  const files = await collectSourceFiles(srcRoot);
  const violations = [];
  for (const file of files) {
    const source = await readFile(file, "utf8");
    const relativePath = path.relative(webRoot, file);
    if (/z-\[(?:9998|9999|10000|10020|10030|11000|11050|12000)\]/.test(source)) {
      violations.push(`${relativePath}: numeric high overlay layer`);
    }
    if (
      /\.(?:ts|tsx)$/.test(file)
      && /(?:min\(82dvh,\s*760px\)|calc\(100dvh\s*-\s*16px\))/.test(source)
    ) {
      violations.push(`${relativePath}: duplicated dialog viewport formula`);
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
  assert.match(recipes, /\.ui-layer-dialog\s*\{/);
  assert.match(recipes, /\.ui-dialog-viewport-adaptive\s*\{/);
  assert.match(recipes, /\.ui-dialog-backdrop-compact\s*\{/);
});
