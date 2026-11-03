import assert from "node:assert/strict";
import test from "node:test";
import { getFileContractErrors, isProductionTypeScriptFile } from "./frontend-file-contract.mjs";

test("file contracts accept leading line comments and JSDoc without a fixed line budget", () => {
  const contract = [
    "INPUT: Controlled field value and native input attributes.",
    "OUTPUT: Accessible input events and a shared form surface.",
    "POS: Form primitive; the caller owns validation and persistence.",
  ];
  const lineComments = contract.map((line) => `// ${line}`).join("\r\n");
  const blockComment = `/**\n${contract.map((line) => ` * ${line}`).join("\n")}\n */`;
  for (const header of [lineComments, blockComment]) {
    assert.deepEqual(getFileContractErrors(`\uFEFF\n/* ${"License notice. ".repeat(120)} */\n${header}\n"use client";\nexport const field = 1;`), []);
  }
});

test("missing, repeated, reordered, empty and placeholder contract fields fail", () => {
  const valid = "// INPUT: A controlled value.\n// OUTPUT: A native field.\n// POS: The input primitive.";
  for (const invalid of [
    valid.replace("// INPUT: A controlled value.\n", ""),
    `${valid}\n// POS: Another owner.`,
    valid.replace("INPUT:", "TEMP:").replace("OUTPUT:", "INPUT:").replace("TEMP:", "OUTPUT:"),
    valid.replace("A native field.", ""),
    valid.replace("A native field.", "TODO: fill in later"),
    valid.replace("A native field.", "待补充"),
  ]) {
    assert.ok(getFileContractErrors(invalid).length > 0, invalid);
  }
});

test("body comments, directives and strings cannot masquerade as the file header", () => {
  const contract = "// INPUT: A controlled value.\n// OUTPUT: A native field.\n// POS: The input primitive.";
  for (const source of [
    `export const value = 1;\n${contract}`,
    `"use client";\n${contract}`,
    `const text = ${JSON.stringify(contract)};`,
    `function example() {\n${contract}\n}`,
  ]) {
    assert.ok(getFileContractErrors(source).length > 0, source);
  }
});

test("production file selection includes nested modules and excludes tests and declarations", () => {
  for (const fileName of ["button.tsx", "form-control-styles.ts", "nested/control.tsx"]) {
    assert.equal(isProductionTypeScriptFile(fileName), true, fileName);
  }
  for (const fileName of ["button.test.tsx", "search-query.test.ts", "field.spec.tsx", "env.d.ts", "CLAUDE.md"]) {
    assert.equal(isProductionTypeScriptFile(fileName), false, fileName);
  }
});


import { Linter } from "eslint";
import parser from "typescript-eslint";
import { frontendRules } from "./frontend-lint-rules.mjs";
const linter = new Linter();
function lint(source, filename, rule) {
  return linter.verify(source, [{ files: ["**/*.{ts,tsx}"], languageOptions: { parser: parser.parser }, plugins: { nexus: { rules: frontendRules } }, rules: { [`nexus/${rule}`]: "error" } }], { filename });
}
test("ESLint diagnoses relative, alias, re-export and dynamic upward dependencies", () => {
  for (const source of ['import "@/features/example/public";', 'export { value } from "../features/example/public";', 'const load = () => import("@/features/example/public");']) {
    assert.equal(lint(source, "src/shared/example.ts", "dependency-direction").length, 1);
  }
  assert.equal(lint('import "@/shared/ui/button";', "src/features/example.ts", "dependency-direction").length, 0);
});
test("semantic style lint examines syntax values without interpreting comments as UI", () => {
  assert.equal(lint('const style = "z-[999]";', "src/features/example.ts", "semantic-style-owners").length, 1);
  assert.equal(lint('// z-[999]\nconst style = "ui-layer-popover";', "src/features/example.ts", "semantic-style-owners").length, 0);
});
test("file contract lint preserves structural ownership without freezing prose", () => {
  const file = "src/shared/ui/button/example.ts";
  assert.equal(lint('export const value = 1;', file, "file-contract").length, 1);
  assert.equal(lint('// INPUT: value\n// OUTPUT: action\n// POS: owner\nexport const value = 1;', file, "file-contract").length, 0);
});
