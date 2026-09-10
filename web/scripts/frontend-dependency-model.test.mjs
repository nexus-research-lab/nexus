import assert from "node:assert/strict";
import test from "node:test";

import {
  collectFrontendModuleReferences,
  findFrontendBoundaryViolations,
  getFrontendLayer,
  resolveFrontendModule,
} from "./frontend-dependency-model.mjs";

test("all target layers reject every upward direction and allow lower or same layers", () => {
  const layers = ["generated", "shared", "entities", "features", "widgets", "pages", "app", "entries"];
  for (const [fromIndex, from] of layers.entries()) {
    for (const [toIndex, to] of layers.entries()) {
      const violations = findFrontendBoundaryViolations(
        `src/${from}/example/consumer.ts`,
        `import { value } from "@/${to}/other/public";`,
      );
      assert.equal(violations.length, Number(toIndex > fromIndex), `${from} -> ${to}`);
    }
  }
});

test("module extraction covers static, side-effect, dynamic, type and re-export syntax", () => {
  const source = [
    'import { value } from "@/features/example/public";',
    'import type { Value } from "@/features/example/public";',
    'import "@/features/example/public";',
    'export { value } from "@/features/example/public";',
    'export * from "@/features/example/public";',
    'export type { Value } from "@/features/example/public";',
    'const load = () => import(/* chunk */ "@/features/example/public");',
    'const literalTemplate = () => import(`@/features/example/public`);',
    'type Value = import("@/features/example/public").Value;',
    'import example = require("@/features/example/public");',
    'const example = require("@/features/example/public");',
  ].join("\n");
  const references = collectFrontendModuleReferences("src/shared/example.ts", source);
  assert.deepEqual(references.map(({ kind }) => kind), [
    "import", "import", "side-effect", "re-export", "re-export", "re-export",
    "dynamic", "dynamic", "type-import", "import-equals", "require",
  ]);
  assert.deepEqual(references.map(({ line }) => line), Array.from({ length: 11 }, (_, index) => index + 1));
  assert.equal(findFrontendBoundaryViolations("src/shared/example.ts", source).length, 11);
});

test("alias, relative and baseUrl references normalize to the same boundary", () => {
  const sourcePath = "src/shared/ui/example.tsx";
  for (const specifier of [
    "@/features/task/public",
    "../../features/task/public",
    "../../features/task/private/../public.ts",
    "src/features/task/public.tsx",
    "/src/features/task/public/index.ts",
    "@/features/task/public.ts?raw",
  ]) {
    assert.equal(resolveFrontendModule(sourcePath, specifier), "src/features/task/public", specifier);
    const [violation] = findFrontendBoundaryViolations(sourcePath, `import "${specifier}";`);
    assert.equal(violation?.to, "src/features/task/public", specifier);
  }
  assert.equal(resolveFrontendModule(sourcePath, "react"), null);
  assert.equal(resolveFrontendModule(sourcePath, "@testing-library/react"), null);
  assert.equal(resolveFrontendModule(sourcePath, "../../../scripts/fixture"), null);
});

test("comments, content strings, JSX text and external modules do not invent imports", () => {
  const source = `
    // import { example } from "@/features/example/public";
    /* export * from "@/app/private"; */
    const text = 'import("@/pages/example/page")';
    const view = <div>from "@/features/example/public"</div>;
    import { useState } from "react";
    import("mermaid");
  `;
  assert.equal(collectFrontendModuleReferences("src/shared/example.tsx", source).length, 2);
  assert.deepEqual(findFrontendBoundaryViolations("src/shared/example.tsx", source), []);
});

test("legacy hook, store, bootstrap and root entry responsibilities remain guarded", () => {
  assert.equal(getFrontendLayer("src/hooks/ui/use-example.ts"), "entities");
  assert.equal(getFrontendLayer("src/store/agent.ts"), "entities");
  assert.equal(getFrontendLayer("src/bootstrap/start.ts"), "app");
  assert.equal(getFrontendLayer("src/App.tsx"), "app");
  assert.equal(getFrontendLayer("src/main.tsx"), "entries");
  assert.equal(getFrontendLayer("src/types/generated/protocol.ts"), "generated");
  assert.equal(findFrontendBoundaryViolations("src/hooks/agent/use-example.ts", 'import "@/features/example/public";').length, 1);
  assert.equal(findFrontendBoundaryViolations("src/shared/ui/example.tsx", 'import "../../store/agent";').length, 1);
  assert.equal(findFrontendBoundaryViolations("src/pages/example/page.tsx", 'import "@/bootstrap/start";').length, 1);
  assert.equal(findFrontendBoundaryViolations("src/app/start.ts", 'import "@/main";').length, 1);
});

test("mixed legacy support directories keep explicit bans without being assigned target layers", () => {
  for (const fromRoot of ["lib", "config", "types"]) {
    const sourcePath = `src/${fromRoot}/example.ts`;
    assert.equal(getFrontendLayer(sourcePath), null);
    for (const toRoot of ["features", "widgets", "pages", "app", "entries", "bootstrap"]) {
      for (const specifier of [`@/${toRoot}/example`, `../${toRoot}/example`]) {
        assert.equal(
          findFrontendBoundaryViolations(sourcePath, `export * from "${specifier}";`).length,
          1,
          `${fromRoot} -> ${specifier}`,
        );
      }
    }
    for (const specifier of ["@/App", "../main.tsx"]) {
      assert.equal(findFrontendBoundaryViolations(sourcePath, `import "${specifier}";`).length, 1, `${fromRoot} -> ${specifier}`);
    }
  }
});

test("protocol declarations and API clients cannot pull runtime implementation or store ownership upward", () => {
  for (const toRoot of ["config", "lib", "hooks", "store", "entities"]) {
    assert.equal(findFrontendBoundaryViolations(
      "src/types/conversation/message.ts",
      `import type { Value } from "@/${toRoot}/example";`,
    ).length, 1, `types -> ${toRoot}`);
  }
  assert.equal(findFrontendBoundaryViolations(
    "src/lib/api/agent/agent-api.ts",
    'const store = () => import("@/store/agent");',
  ).length, 1);
  for (const [from, to] of [
    ["src/config/runtime-options.ts", "@/lib/agent-options"],
    ["src/lib/api/core/api-client.ts", "@/config/runtime-endpoints"],
    ["src/lib/conversation/session.ts", "@/types/conversation/conversation"],
    ["src/types/conversation/message.ts", "@/types/generated/protocol"],
  ]) {
    assert.deepEqual(findFrontendBoundaryViolations(from, `import type { Value } from "${to}";`), [], `${from} -> ${to}`);
  }
});
