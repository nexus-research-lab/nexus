// INPUT: Canonical theme tokens and every production CSS/TS module.
// OUTPUT: Undefined static token references, broken theme aliases and duplicate token declarations fail the frontend gate.
// POS: Static token integrity, paired with computed styles in the real-browser matrix; no per-file exception budget.

import assert from "node:assert/strict";
import { readFile, readdir } from "node:fs/promises";
import test from "node:test";
import { findUnboundTokenReferences, inspectThemeAliases, inspectTokenSource } from "./frontend-token-policy.mjs";

test("token references distinguish required values, nested fallbacks and Tailwind shorthand", () => {
  const sources = [
    { path: "theme.css", source: ":root { --text: black; --color: var(--optional, var(--text)); }" },
    { path: "view.tsx", source: '/* bg-(--comment) */ const view = <div className="text-(color:--missing) bg-[var(--local)]" style={{ "--local": "var(--text)" }} />;' },
  ];
  assert.deepEqual(findUnboundTokenReferences(sources).map((item) => item.name), ["--missing"]);
  assert.deepEqual(findUnboundTokenReferences([{ path: "bad.css", source: "a { color: var(--optional, var(--missing)); }" }]).map((item) => item.name), ["--missing"]);
});

test("generated document CSS templates retain their own declared variables", () => {
  const source = 'const css = `:root { --document-color: ${color}; --document-font: sans-serif; } body { color: var(--document-color); font-family: var(--document-font); }`;';
  assert.deepEqual(findUnboundTokenReferences([{ path: "document.ts", source }]), []);
});

test("quoted CSS content and CSS comments are not active token references", () => {
  assert.deepEqual(findUnboundTokenReferences([
    { path: "label.css", source: "a::before { content: 'var(--example)'; color: red /* var(--comment) */; }" },
    { path: "frame.ts", source: 'const css = `/* --comment-only: red; */ body { color: var(--comment-only); }`;' },
  ]).map((item) => item.name), ["--comment-only"]);
});

test("CSS comments do not declare tokens and runtime geometry needs an explicit fallback", () => {
  const result = inspectTokenSource("view.css", "/* --missing: red */ a { color: var(--missing); width: var(--runtime-width, 48px); } @property --registered { syntax: '<color>'; inherits: true; initial-value: red; }");
  assert.equal(result.declarations.has("--missing"), false);
  assert.equal(result.declarations.has("--registered"), true);
  assert.deepEqual(findUnboundTokenReferences([{ path: "view.css", source: "a { width: var(--runtime-width, 48px); color: var(--missing); }" }]).map((item) => item.name), ["--missing"]);
});

test("theme resolution catches per-theme cycles, missing aliases and local duplicates", () => {
  const source = ':root { --base: red; --alias: var(--base); } :root[data-theme="dark"] { --base: var(--alias); } @theme inline { --base: var(--base); }';
  assert.deepEqual(inspectThemeAliases(source, "light").issues, []);
  assert.match(inspectThemeAliases(source, "dark").issues.join("\n"), /cycle --base -> --alias -> --base/);
  assert.deepEqual(inspectThemeAliases(":root { --x: red; --x: var(--absent); }", "rain").issues, ["duplicate --x", "--x requires --absent"]);
});

async function productionSources(directory, prefix = "src") {
  const sources = [];
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const path = `${prefix}/${entry.name}`;
    const url = new URL(entry.name + (entry.isDirectory() ? "/" : ""), directory);
    if (entry.isDirectory()) {
      // Gallery deliberately contains source examples as inert preview content.
      if (path !== "src/dev" && path !== "src/test" && entry.name !== "__tests__") sources.push(...await productionSources(url, path));
    } else if (/\.(?:css|tsx?)$/.test(entry.name) && !/\.(?:test|spec|d)\.tsx?$/.test(entry.name)) {
      sources.push({ path, source: await readFile(url, "utf8") });
    }
  }
  return sources;
}

test("every production static CSS or Tailwind token reference has a declared owner or fallback", async () => {
  const sources = await productionSources(new URL("../src/", import.meta.url));
  const issues = findUnboundTokenReferences(sources);
  assert.deepEqual(issues, [], issues.map(({ path, line, name }) => `${path}:${line} requires ${name}`).join("\n"));
});

test("all three themes resolve every canonical token alias without cycles", async () => {
  const source = await readFile(new URL("../src/app/styles/theme-tokens.css", import.meta.url), "utf8");
  for (const theme of ["light", "dark", "rain"]) {
    const { issues } = inspectThemeAliases(source, theme);
    assert.deepEqual(issues, [], `${theme}: ${issues.join("; ")}`);
  }
});
