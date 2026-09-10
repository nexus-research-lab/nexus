// INPUT: Production CSS/TypeScript sources and the canonical theme token stylesheet.
// OUTPUT: Static custom-property declarations/references and per-theme missing/cyclic aliases.
// POS: Token integrity checker using the existing CSS build parser and TypeScript AST; dynamic values and DOM inheritance still need browser review.

import { createRequire } from "node:module";
import ts from "typescript";

// Resolve the parser through its existing build-tool owner, including pnpm's
// isolated dependency layout; this checker introduces no separate CSS parser.
const postcss = createRequire(import.meta.resolve("@tailwindcss/postcss"))("postcss");

function cssSyntax(value) {
  return value.replace(/\/\*[\s\S]*?\*\/|"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'/g, "");
}

export function tokenReferences(value) {
  const references = [];
  const syntax = cssSyntax(value);
  for (const match of syntax.matchAll(/var\(\s*(--[\w-]+)\s*([,)])/g)) {
    references.push({ name: match[1], optional: match[2] === "," });
  }
  // Tailwind's property shorthand is a required var() after CSS generation.
  for (const match of syntax.matchAll(/(?<![\w])\((?:[a-z-]+:)?(--[\w-]+)\)/g)) {
    references.push({ name: match[1], optional: false });
  }
  return references;
}

export function inspectTokenSource(path, source) {
  const declarations = new Set();
  const references = [];
  const addReferences = (value, line) => {
    for (const reference of tokenReferences(value)) references.push({ ...reference, path, line });
  };
  if (path.endsWith(".css")) {
    const root = postcss.parse(source, { from: path });
    root.walkDecls((declaration) => {
      if (declaration.prop.startsWith("--")) declarations.add(declaration.prop);
      addReferences(declaration.value, declaration.source.start.line);
    });
    root.walkAtRules("property", (rule) => declarations.add(rule.params.trim()));
  } else {
    const root = ts.createSourceFile(path, source, ts.ScriptTarget.Latest, true,
      path.endsWith(".tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS);
    function visit(node) {
      if (ts.isPropertyAssignment(node) && ts.isStringLiteralLike(node.name) && node.name.text.startsWith("--")) {
        declarations.add(node.name.text);
      }
      if (ts.isStringLiteralLike(node) || ts.isTemplateHead(node) || ts.isTemplateMiddle(node) || ts.isTemplateTail(node)) {
        // Generated iframe documents declare CSS inside template fragments.
        for (const match of cssSyntax(node.text).matchAll(/(?:^|[;{])\s*(--[\w-]+)\s*:/g)) declarations.add(match[1]);
        addReferences(node.text, root.getLineAndCharacterOfPosition(node.getStart(root)).line + 1);
      }
      ts.forEachChild(node, visit);
    }
    visit(root);
  }
  return { declarations, references };
}

export function findUnboundTokenReferences(sources) {
  const inspected = sources.map(({ path, source }) => inspectTokenSource(path, source));
  const declarations = new Set(inspected.flatMap((item) => [...item.declarations]));
  return inspected.flatMap((item) => item.references)
    .filter((reference) => !reference.optional && !declarations.has(reference.name));
}

export function inspectThemeAliases(source, theme) {
  const tokens = new Map();
  const issues = [];
  postcss.parse(source).walkRules((rule) => {
    const selectors = rule.selectors ?? [];
    if (!selectors.some((selector) => selector === ":root" || selector === `:root[data-theme="${theme}"]`)) return;
    const localNames = new Set();
    rule.nodes.filter((node) => node.type === "decl" && node.prop.startsWith("--")).forEach((node) => {
      if (localNames.has(node.prop)) issues.push(`duplicate ${node.prop}`);
      localNames.add(node.prop);
      tokens.set(node.prop, node.value);
    });
  });
  const visited = new Set();
  function inspect(name, stack) {
    if (stack.includes(name)) {
      issues.push(`cycle ${[...stack.slice(stack.indexOf(name)), name].join(" -> ")}`);
      return;
    }
    if (visited.has(name)) return;
    visited.add(name);
    for (const reference of tokenReferences(tokens.get(name))) {
      if (tokens.has(reference.name)) inspect(reference.name, [...stack, name]);
      else if (!reference.optional) issues.push(`${name} requires ${reference.name}`);
    }
  }
  for (const name of tokens.keys()) inspect(name, []);
  return { tokens, issues };
}
