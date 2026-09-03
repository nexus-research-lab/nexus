import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const readSource = (relativePath) => readFile(
  new URL(`../${relativePath}`, import.meta.url),
  "utf8",
);

test("anchored overlay entry motion never transitions measured geometry", async () => {
  const [motionSource, recipeSource] = await Promise.all([
    readSource("src/shared/ui/overlay/overlay-styles.ts"),
    readSource("src/app/styles/theme-recipes.css"),
  ]);
  const motionRule = recipeSource.match(
    /\.ui-anchored-overlay-motion\s*\{(?<body>[\s\S]*?)\n\s*\}/,
  );

  assert.match(motionSource, /"ui-anchored-overlay-motion"/);
  assert.doesNotMatch(motionSource, /duration-\(/);
  assert.ok(motionRule?.groups?.body, "shared anchored overlay motion rule is missing");
  assert.match(
    motionRule.groups.body,
    /animation:\s*nexus-anchored-overlay-enter/,
  );
  assert.match(
    motionRule.groups.body,
    /transition-property:\s*opacity, transform/,
  );
  assert.doesNotMatch(
    motionRule.groups.body,
    /^\s*(?:bottom|left|right|top)\s*:/m,
  );
});
