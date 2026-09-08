// INPUT: Production TypeScript files in the explicitly governed foundation owners.
// OUTPUT: A failing contract gate when a file loses its leading INPUT / OUTPUT / POS documentation.
// POS: Checks comment structure and scope; reviewers still verify the declared semantics against code.

import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

import ts from "typescript";

const webRoot = fileURLToPath(new URL("..", import.meta.url));
const GOVERNED_ROOTS = [
  ...["button", "dialog", "form", "list", "menu", "navigation", "overlay", "typography", "workspace/catalog"]
    .map((owner) => `src/shared/ui/${owner}`),
  "src/shared/lib",
  "src/shared/navigation",
];
const CONTRACT_FIELDS = ["INPUT", "OUTPUT", "POS"];

function isProductionTypeScriptFile(fileName) {
  return /\.tsx?$/.test(fileName) && !/(?:\.(?:test|spec)\.tsx?|\.d\.ts)$/.test(fileName);
}

function getFileContractErrors(source) {
  const scanner = ts.createScanner(ts.ScriptTarget.Latest, false, ts.LanguageVariant.Standard, source);
  const comments = [];
  for (let token = scanner.scan(); token !== ts.SyntaxKind.EndOfFileToken; token = scanner.scan()) {
    if (token === ts.SyntaxKind.SingleLineCommentTrivia || token === ts.SyntaxKind.MultiLineCommentTrivia) {
      comments.push(scanner.getTokenText());
    } else if (token !== ts.SyntaxKind.WhitespaceTrivia && token !== ts.SyntaxKind.NewLineTrivia && token !== ts.SyntaxKind.ShebangTrivia) {
      break;
    }
  }
  const fields = comments.flatMap((comment) => comment
    .replace(/^\/\*+|\*\/$/g, "")
    .split(/\r?\n/)
    .flatMap((line) => {
      const match = line.replace(/^\s*(?:\/\/|\*)\s?/, "").match(/^\s*(INPUT|OUTPUT|POS)\s*[:：]\s*(.*?)\s*$/);
      return match ? [{ name: match[1], description: match[2] }] : [];
    }));
  const errors = [];
  if (fields.map(({ name }) => name).join("/") !== CONTRACT_FIELDS.join("/")) {
    errors.push("Declare INPUT / OUTPUT / POS exactly once, in order, in comments before the first code token");
  }
  for (const { name, description } of fields) {
    if (!description || /^(?:TODO\b|TBD\b|FIXME\b|待补充|待填写|占位|\.{3}|…)/i.test(description)) {
      errors.push(`${name} requires a non-empty description of this file's actual responsibility`);
    }
  }
  return errors;
}

async function collectProductionFiles(directory) {
  const entries = await readdir(directory, { withFileTypes: true });
  return (await Promise.all(entries.map(async (entry) => {
    const target = path.join(directory, entry.name);
    if (entry.isDirectory()) return collectProductionFiles(target);
    return isProductionTypeScriptFile(entry.name) ? [target] : [];
  }))).flat();
}

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

test("governed interaction, browser and React owners keep leading ownership contracts", async () => {
  const files = (await Promise.all(GOVERNED_ROOTS.map(async (owner) => {
    const ownedFiles = await collectProductionFiles(path.join(webRoot, owner));
    assert.ok(ownedFiles.length > 0, `${owner} must remain a real governed owner`);
    return ownedFiles;
  }))).flat();
  const violations = (await Promise.all(files.map(async (file) => {
    const source = await readFile(file, "utf8");
    return getFileContractErrors(source).map((error) => `${path.relative(webRoot, file)}: ${error}`);
  }))).flat().sort();
  assert.deepEqual(violations, []);
});
