// INPUT: Contacts 目录视图源码。
// OUTPUT: 证明目录视图切换仍由共享分段控件拥有。
// POS: Contacts 静态架构门禁；真实筛选行为由共置 Vitest 测试直接导入执行。

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const webRoot = fileURLToPath(new URL("..", import.meta.url));

test("Agent 目录视图切换由共享分段控件统一管理", async () => {
  const source = await readFile(
    path.join(webRoot, "src/features/contacts/contacts-directory.tsx"),
    "utf8",
  );

  assert.match(source, /<UiSegmentedControl/);
  assert.match(source, /contacts\.views\.title/);
  assert.doesNotMatch(source, /aria-pressed=\{view ===/);
});
