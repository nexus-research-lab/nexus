import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

import { findFrontendBoundaryViolations } from "./frontend-dependency-model.mjs";

const webRoot = fileURLToPath(new URL("..", import.meta.url));
const srcRoot = path.join(webRoot, "src");

async function collectTypeScriptFiles(directory) {
  const entries = await readdir(directory, { withFileTypes: true });
  const nested = await Promise.all(entries.map(async (entry) => {
    const target = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      return collectTypeScriptFiles(target);
    }
    return /\.tsx?$/.test(entry.name) ? [target] : [];
  }));
  return nested.flat();
}

test("all frontend layers reject upward dependencies without legacy exemptions", async () => {
  const files = await collectTypeScriptFiles(srcRoot);
  const violations = (await Promise.all(files.map(async (file) => {
    const source = await readFile(file, "utf8");
    return findFrontendBoundaryViolations(path.relative(webRoot, file), source);
  }))).flat();
  assert.deepEqual(
    violations.map((edge) => `${edge.from}:${edge.line} (${edge.kind}) -> ${edge.to}`)
      .sort(),
    [],
    "Upward imports must be fixed at their owner; changing import syntax or adding a legacy allowlist does not establish ownership",
  );
});
