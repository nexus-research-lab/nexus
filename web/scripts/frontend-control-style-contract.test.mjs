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

test("visual guard handles arbitrary CSS properties without banning arbitrary layout", () => {
  const source = header + `
    const view = <Action className="[width:240px] hover:[color:red] focus:[font-weight:600]! [--button-primary-background:red]"
      style={{ borderTopColor: "red", outlineOffset: 8, width: 240 }} />;
  `;
  assert.deepEqual(findControlVisualOverrides(samplePath, source).map((issue) => issue.value), [
    "hover:[color:red]", "focus:[font-weight:600]!", "[--button-primary-background:red]", "borderTopColor", "outlineOffset",
  ]);
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

test("visual guard keeps list row state and surface choices in their owner", () => {
  const source = `
    import { UiListRow as Row } from "@/shared/ui/list/list-row";
    const a = <Row className="rounded-none hover:bg-red-500 opacity-70" />;
    const b = <Row density="sidebar" muted variant="outlined" className="min-w-0" />;
  `;
  assert.deepEqual(findControlVisualOverrides(samplePath, source).map((issue) => issue.value), [
    "rounded-none", "hover:bg-red-500", "opacity-70",
  ]);
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

test("visual guard covers field content, search inputs and native selection controls", () => {
  const source = `
    import { UiInput as Input, UiTextarea, UiNativeSelect, UiSearchInput } from "@/shared/ui/form/form-control";
    import { UiCheckbox } from "@/shared/ui/form/checkbox";
    import { UiChoiceButton, UiRadioChoice } from "@/shared/ui/form/choice";
    const a = <Input className="font-mono" />;
    const b = <UiTextarea className="message-code-font leading-relaxed" />;
    const c = <UiNativeSelect className="rounded-full" />;
    const d = <UiSearchInput inputClassName="ui-type-caption font-medium" />;
    const e = <UiCheckbox className="accent-red-500" />;
    const f = <UiChoiceButton style={{ borderColor: "red" }} />;
    const g = <UiRadioChoice className="bg-red-500" />;
  `;
  assert.deepEqual(findControlVisualOverrides(samplePath, source).map((issue) => issue.value), [
    "font-mono", "message-code-font", "leading-relaxed", "rounded-full", "ui-type-caption",
    "font-medium", "accent-red-500", "borderColor", "bg-red-500",
  ]);
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
