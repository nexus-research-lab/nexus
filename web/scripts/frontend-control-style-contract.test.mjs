// INPUT: Shared control consumers and representative TypeScript syntax fixtures.
// OUTPUT: Reject private visual classes and inline styles while allowing caller layout.
// POS: Static ownership guard; behavior and computed CSS remain browser test responsibilities.

import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import test from "node:test";
import { findControlVisualOverrides } from "./frontend-control-style-policy.mjs";

const samplePath = "src/features/example.tsx";
const header = 'import { UiButton as Action } from "@/shared/ui/button/button";\n';

test("visual guard follows aliases, variants, constants and spread props", () => {
  const source = header + `
    const local = "hover:bg-(--primary) focus-visible:ring-0";
    const props = { className: cn("font-bold", selected && local) };
    const view = <Action {...props} className={wide ? "w-full text-left" : "border-[color:var(--primary)]"} />;
  `;
  assert.deepEqual(findControlVisualOverrides(samplePath, source).map((issue) => issue.value), [
    "font-bold", "hover:bg-(--primary)", "focus-visible:ring-0", "border-[color:var(--primary)]",
  ]);
});

test("visual guard resolves namespace and relative imports and inline styles", () => {
  const source = `
    import * as Controls from "../shared/ui/button/button.tsx";
    const style = { width: 120, color: "red", "--button-primary-background": "blue" };
    const props = { style };
    const view = <Controls.UiIconButton {...props} />;
  `;
  assert.deepEqual(findControlVisualOverrides(samplePath, source).map((issue) => issue.value), ["color", "--button-primary-background"]);
});

test("visual guard allows external geometry and independent icon artwork", () => {
  assert.deepEqual(findControlVisualOverrides(samplePath, header + `
    const view = <Action className="absolute right-2 w-full min-w-0 text-left sm:hidden" style={{ width: 240 }}>
      <svg className="text-(--warning) h-4 w-4" />
    </Action>;
  `), []);
});

test("visual guard checks list actions and select trigger classes", () => {
  const source = `
    import { UiListActionButton } from "@/shared/ui/list/list-action";
    import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
    const a = <UiListActionButton className="opacity-60 hover:opacity-100" />;
    const b = <UiSelectMenu buttonClassName="shadow-none" />;
  `;
  assert.deepEqual(findControlVisualOverrides(samplePath, source).map((issue) => issue.value), ["opacity-60", "hover:opacity-100", "shadow-none"]);
});

test("visual guard respects block and parameter shadowing", () => {
  const source = header + `
    const style = "bg-(--primary)";
    function Local() {
      const style = "w-full";
      return <Action className={style} />;
    }
    function Forward({ style }) { return <Action className={style} />; }
    const view = <Action className={style} />;
  `;
  assert.deepEqual(findControlVisualOverrides(samplePath, source).map((issue) => issue.value), ["bg-(--primary)"]);
});

async function sourceFiles(directory) {
  const children = await readdir(new URL(`../${directory}/`, import.meta.url), { withFileTypes: true });
  const paths = await Promise.all(children.map((entry) => entry.isDirectory()
    ? sourceFiles(`${directory}/${entry.name}`)
    : entry.name.endsWith(".tsx") && !entry.name.endsWith(".test.tsx") ? [`${directory}/${entry.name}`] : []));
  return paths.flat();
}

test("product controls consume shared visual owners without private overrides", async () => {
  const files = (await Promise.all([sourceFiles("src/features"), sourceFiles("src/pages")])).flat().sort();
  const violations = [];
  for (const file of files) {
    const source = await readFile(new URL(`../${file}`, import.meta.url), "utf8");
    for (const issue of findControlVisualOverrides(file, source)) {
      violations.push(`${file}:${issue.line} ${issue.control}.${issue.property}: ${issue.value}`);
    }
  }
  assert.deepEqual(violations, [], "Choose shared tone/variant/size/visibility instead of consumer visual classes.");
});

test("catalog primary actions and list actions keep one native button owner", async () => {
  const read = (file) => readFile(new URL(`../src/shared/ui/${file}`, import.meta.url), "utf8");
  const [catalog, action, visibility] = await Promise.all([
    read("workspace/catalog/workspace-catalog-card.tsx"), read("list/list-action.tsx"), read("list/list-action-styles.ts"),
  ]);
  assert.match(catalog, /<UiButton[\s\S]*?data-slot="catalog-primary-action"/);
  assert.match(action, /<UiIconButton/);
  assert.doesNotMatch(action, /<button\b/);
  assert.doesNotMatch(visibility, /border-|bg-|text-|rounded-|ring-/);
});
