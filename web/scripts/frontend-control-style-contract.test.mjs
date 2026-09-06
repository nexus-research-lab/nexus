// INPUT: Shared control consumers and representative TypeScript syntax fixtures.
// OUTPUT: Reject private visual classes and inline styles while allowing caller layout.
// POS: Static ownership guard; behavior and computed CSS remain browser test responsibilities.

import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import test from "node:test";
import { findControlVisualOverrides } from "./frontend-control-style-policy.mjs";
import { resolveFrontendModule } from "./frontend-dependency-model.mjs";

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

test("directory filters share select visuals while allowing content-specific widths", () => {
  const source = `
    import { UiFilterSelect as Filter } from "@/shared/ui/menu/filter-select";
    const valid = <Filter className="w-full sm:w-[232px] min-w-0" />;
    const invalid = <Filter className="rounded-full text-xs" style={{ backgroundColor: "red" }} />;
  `;
  assert.deepEqual(findControlVisualOverrides(samplePath, source).map((issue) => issue.value), [
    "rounded-full", "text-xs", "backgroundColor",
  ]);
});

test("visual guard owns segmented paint and typography while allowing layout", () => {
  const source = `
    import { UiSegmentedControl as Choices } from "@/shared/ui/form/segmented-control";
    const custom = { className: "font-bold bg-red-500 rounded-full" };
    const invalid = <Choices {...custom} />;
    const layout = <Choices density="compact" stretch className="min-w-0 w-full shrink-0" />;
  `;
  assert.deepEqual(findControlVisualOverrides(samplePath, source).map((issue) => issue.value), [
    "font-bold", "bg-red-500", "rounded-full",
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

test("visual guard follows imported constants and named re-exports without executing modules", () => {
  const modules = new Map([
    ["src/features/legacy", 'export const field = cn("w-full", "focus-visible:ring-0", "text-xs"); throw new Error("never execute");'],
    ["src/features/forward", 'export { field as legacyField } from "./legacy";'],
  ]);
  const source = `import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
    import { legacyField as custom } from "./forward";
    const view = <UiSelectMenu buttonClassName={custom} />;`;
  assert.deepEqual(findControlVisualOverrides(samplePath, source, modules).map((issue) => issue.value), ["focus-visible:ring-0", "text-xs"]);
});

test("imported style resolution preserves local shadowing, layout-only values and cycle bounds", () => {
  const modules = new Map([
    ["src/features/styles", 'export const custom = "bg-red-500"; const layout = "w-full min-w-0"; export { layout };'],
    ["src/features/cycle", 'export { cycle } from "./cycle";'],
  ]);
  const source = header + `import { custom, layout } from "./styles";
    import { cycle } from "./cycle";
    function View({ custom }) { return <Action className={custom} />; }
    const view = <Action className={layout} data-reference={custom} style={cycle} />;`;
  assert.deepEqual(findControlVisualOverrides(samplePath, source, modules), []);
});

test("visual guard covers field content, search inputs and native selection controls", () => {
  const source = `
    import { UiInput as Input, UiTextarea, UiNativeSelect, UiSearchInput } from "@/shared/ui/form/form-control";
    import { UiCheckbox } from "@/shared/ui/form/checkbox";
    import { UiSourceEditor } from "@/shared/ui/form/source-editor";
    import { UiChoiceButton, UiRadioChoice } from "@/shared/ui/form/choice";
    const a = <Input className="font-mono" />;
    const b = <UiTextarea className="message-code-font leading-relaxed" />;
    const c = <UiNativeSelect className="rounded-full" />;
    const d = <UiSearchInput inputClassName="ui-type-caption font-medium" />;
    const e = <UiCheckbox className="accent-red-500" />;
    const f = <UiChoiceButton style={{ borderColor: "red" }} />;
    const g = <UiRadioChoice className="bg-red-500" />;
    const h = <UiSourceEditor className="font-sans focus-visible:ring-0" />;
  `;
  assert.deepEqual(findControlVisualOverrides(samplePath, source).map((issue) => issue.value), [
    "font-mono", "message-code-font", "leading-relaxed", "rounded-full", "ui-type-caption",
    "font-medium", "accent-red-500", "borderColor", "bg-red-500", "font-sans", "focus-visible:ring-0",
  ]);
});

async function sourceFiles(directory) {
  const children = await readdir(new URL(`../${directory}/`, import.meta.url), { withFileTypes: true });
  const paths = await Promise.all(children.map((entry) => entry.isDirectory()
    ? sourceFiles(`${directory}/${entry.name}`)
    : /\.[cm]?tsx?$/.test(entry.name) && !/\.(test|spec)\./.test(entry.name) ? [`${directory}/${entry.name}`] : []));
  return paths.flat();
}

test("product controls consume shared visual owners without private overrides", async () => {
  const files = (await sourceFiles("src")).sort();
  const sources = new Map(await Promise.all(files.map(async (file) => [
    resolveFrontendModule(file, `/${file}`), await readFile(new URL(`../${file}`, import.meta.url), "utf8"),
  ])));
  const violations = [];
  for (const file of files) {
    if (!/^src\/(?:features|pages)\//.test(file) || !file.endsWith(".tsx")) continue;
    const source = sources.get(resolveFrontendModule(file, `/${file}`));
    for (const issue of findControlVisualOverrides(file, source, sources)) {
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
