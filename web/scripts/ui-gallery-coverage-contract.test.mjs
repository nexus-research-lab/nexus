import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const webRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const sharedUiRoot = path.join(webRoot, "src", "shared", "ui");
const inventoryPath = path.join(
  webRoot,
  "src",
  "dev",
  "ui-gallery",
  "ui-gallery-inventory.ts",
);

async function listFiles(directory) {
  const entries = await readdir(directory, { withFileTypes: true });
  const nested = await Promise.all(entries.map(async (entry) => {
    const entryPath = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      return listFiles(entryPath);
    }
    return entryPath.endsWith(".tsx") ? [entryPath] : [];
  }));
  return nested.flat();
}

function collectExportedComponents(source) {
  return Array.from(
    source.matchAll(/^export\s+(?:function|const)\s+([A-Z][a-z][A-Za-z0-9]*)/gm),
    (match) => match[1],
  );
}

function collectInventoriedComponents(source) {
  return Array.from(
    source.matchAll(/components:\s*\[([\s\S]*?)\],/g),
    (match) => Array.from(
      match[1].matchAll(/"([A-Z][A-Za-z0-9]*)"/g),
      (componentMatch) => componentMatch[1],
    ),
  ).flat();
}

test("UI Gallery inventories every exported shared UI React component exactly once", async () => {
  const componentFiles = await listFiles(sharedUiRoot);
  const exportedComponents = (
    await Promise.all(componentFiles.map(async (file) => (
      collectExportedComponents(await readFile(file, "utf8"))
    )))
  ).flat().sort();
  const inventorySource = await readFile(inventoryPath, "utf8");
  const inventoriedComponents = collectInventoriedComponents(inventorySource).sort();

  assert.deepEqual(
    inventoriedComponents,
    exportedComponents,
    "Update the real UI Gallery preview and its coverage group when shared/ui exports change.",
  );
});
