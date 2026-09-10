// INPUT: 业务层生产 CSS 与 TypeScript 源码。
// OUTPUT: 禁止业务组件绕过共享 elevation 和 layer 所有者。
// POS: 前端基础样式禁止项门禁；组件行为与视觉结果由组件和浏览器测试负责。

import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const webRoot = fileURLToPath(new URL("..", import.meta.url));
const productRoots = ["src/features", "src/pages"];
const forbiddenPatterns = [
  ["arbitrary shadow", /(?:drop-)?shadow-\[[^\]]+\]/g],
  ["numeric z-index", /\bz-\[\d+\]/g],
];

async function collectSources(directory) {
  const entries = await readdir(directory, { withFileTypes: true });
  const nested = await Promise.all(entries.map(async (entry) => {
    const target = path.join(directory, entry.name);
    if (entry.isDirectory()) return collectSources(target);
    if (!/\.(?:css|ts|tsx)$/.test(entry.name) || /\.(?:test|spec)\./.test(entry.name)) return [];
    return [target];
  }));
  return nested.flat();
}

test("product source uses semantic elevation and layer owners", async () => {
  const files = (await Promise.all(
    productRoots.map((root) => collectSources(path.join(webRoot, root))),
  )).flat();
  const violations = [];

  for (const file of files) {
    const source = await readFile(file, "utf8");
    for (const [label, pattern] of forbiddenPatterns) {
      for (const match of source.matchAll(pattern)) {
        const line = source.slice(0, match.index).split("\n").length;
        violations.push(`${path.relative(webRoot, file)}:${line} ${label}: ${match[0]}`);
      }
    }
  }

  assert.deepEqual(violations, []);
});
