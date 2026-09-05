// INPUT: React source and canonical shared control imports.
// OUTPUT: Statically provable className/style overrides of control-owned visuals.
// POS: Architecture gate; layout is allowed, dynamic runtime CSS still requires review.

import ts from "typescript";
import { resolveFrontendModule } from "./frontend-dependency-model.mjs";

const CONTROL_MODULES = new Map([
  ["src/shared/ui/button/button", new Set(["UiButton", "UiLinkButton", "UiIconButton"])],
  ["src/shared/ui/button/split-button", new Set(["UiSplitButton"])],
  ["src/shared/ui/list/list-action", new Set(["UiListActionButton"])],
  ["src/shared/ui/menu/select-menu", new Set(["UiSelectMenu"])],
  ["src/shared/ui/form/form-control", new Set(["UiInput", "UiTextarea", "UiNativeSelect", "UiSearchInput"])],
  ["src/shared/ui/form/checkbox", new Set(["UiCheckbox"])],
  ["src/shared/ui/form/choice", new Set(["UiChoiceButton", "UiRadioChoice"])],
]);
const VISUAL_CLASS = /^(?:bg-|border(?:$|-)|text-(?!(?:left|right|center|justify|start|end|ellipsis|clip|wrap|nowrap|balance|pretty)$)|font-|ui-type-|message-code-font$|accent-|caret-|leading-|tracking-|rounded(?:$|-)|radius-control-|surface-radius-|(?:drop-)?shadow(?:$|-)|(?:ring|outline)(?:$|-)|opacity-|transition(?:$|-)|duration-|ease-|animate-|scale-|rotate-|skew-)/;
const VISUAL_PROPERTIES = new Set([
  "color", "background", "backgroundColor", "backgroundImage", "border", "borderColor",
  "borderWidth", "borderStyle", "borderRadius", "boxShadow", "outline", "outlineColor",
  "outlineWidth", "font", "fontSize", "fontWeight", "fontFamily", "lineHeight",
  "letterSpacing", "opacity", "transition", "animation",
  "textShadow", "textDecoration", "textDecorationColor", "accentColor", "caretColor",
]);

function isVisualProperty(name) {
  const camelName = name.replace(/-([a-z])/g, (_, letter) => letter.toUpperCase());
  return name.startsWith("--") || VISUAL_PROPERTIES.has(camelName)
    || /^(?:background|border|outline|font|transition|animation)[A-Z]/.test(camelName);
}

function utilityAfterVariants(token) {
  let nesting = 0;
  let start = 0;
  for (let index = 0; index < token.length; index += 1) {
    const char = token[index];
    if (char === "[" || char === "(") nesting += 1;
    else if (char === "]" || char === ")") nesting -= 1;
    else if (char === ":" && nesting === 0) start = index + 1;
  }
  return token.slice(start).replace(/^!|!$/g, "");
}

function isVisualClass(token) {
  const utility = utilityAfterVariants(token);
  const arbitraryProperty = utility.match(/^\[([^:]+):/);
  return VISUAL_CLASS.test(utility) || Boolean(arbitraryProperty && isVisualProperty(arbitraryProperty[1]));
}

export function findControlVisualOverrides(filePath, source) {
  const tree = ts.createSourceFile(filePath, source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  const controls = new Set();
  const scopeBindings = new Map();
  const violations = [];
  for (const node of tree.statements) {
    if (ts.isImportDeclaration(node) && ts.isStringLiteral(node.moduleSpecifier)) {
      const names = CONTROL_MODULES.get(resolveFrontendModule(filePath, node.moduleSpecifier.text));
      const bindings = node.importClause?.namedBindings;
      if (!names || !bindings) continue;
      if (ts.isNamespaceImport(bindings)) {
        for (const name of names) controls.add(`${bindings.name.text}.${name}`);
      } else {
        for (const element of bindings.elements) {
          if (names.has((element.propertyName ?? element.name).text)) controls.add(element.name.text);
        }
      }
    }
  }
  function bindingScope(node) {
    while (node && !ts.isSourceFile(node) && !ts.isBlock(node) && !ts.isFunctionLike(node)) node = node.parent;
    return node;
  }
  function bind(scope, name, value) {
    if (!scopeBindings.has(scope)) scopeBindings.set(scope, new Map());
    if (ts.isIdentifier(name)) scopeBindings.get(scope).set(name.text, value);
    else if (ts.isObjectBindingPattern(name) || ts.isArrayBindingPattern(name)) {
      for (const element of name.elements) if (ts.isBindingElement(element)) bind(scope, element.name, undefined);
    }
  }
  // Keep lexical shadowing and parameter barriers; never execute caller code.
  function indexConstants(node) {
    if (ts.isVariableDeclaration(node)) {
      const immutable = ts.isVariableDeclarationList(node.parent) && (node.parent.flags & ts.NodeFlags.Const);
      bind(bindingScope(node.parent), node.name, immutable ? node.initializer : undefined);
    } else if (ts.isParameter(node)) {
      bind(bindingScope(node.parent), node.name, undefined);
    }
    ts.forEachChild(node, indexConstants);
  }
  indexConstants(tree);

  function unwrap(node, seen = new Set()) {
    if (!node) return node;
    if (ts.isParenthesizedExpression(node) || ts.isAsExpression(node) || ts.isSatisfiesExpression(node)) return unwrap(node.expression, seen);
    if (ts.isIdentifier(node) && !seen.has(node)) {
      for (let scope = node.parent; scope; scope = scope.parent) {
        if (scopeBindings.get(scope)?.has(node.text)) {
          const value = scopeBindings.get(scope).get(node.text);
          return value ? unwrap(value, new Set([...seen, node])) : node;
        }
      }
    }
    return node;
  }
  function strings(node, seen = new Set()) {
    node = unwrap(node);
    if (!node || seen.has(node)) return [];
    const next = new Set([...seen, node]);
    if (ts.isStringLiteralLike(node) || ts.isTemplateHead(node) || ts.isTemplateMiddle(node) || ts.isTemplateTail(node)) return [node.text];
    if (ts.isPropertyAssignment(node)) return [...strings(node.name, next), ...strings(node.initializer, next)];
    const result = [];
    ts.forEachChild(node, (child) => { result.push(...strings(child, next)); });
    return result;
  }
  function objectEntries(node, seen = new Set()) {
    node = unwrap(node);
    if (!node || seen.has(node) || !ts.isObjectLiteralExpression(node)) return [];
    const next = new Set([...seen, node]);
    return node.properties.flatMap((property) => {
      if (ts.isSpreadAssignment(property)) return objectEntries(property.expression, next);
      if (ts.isPropertyAssignment(property) || ts.isShorthandPropertyAssignment(property)) {
        const name = property.name;
        if (ts.isIdentifier(name) || ts.isStringLiteralLike(name)) return [[name.text, property.initializer ?? name]];
      }
      return [];
    });
  }
  function inspectProps(node, entries) {
    for (const [property, value] of entries) {
      const invalid = property === "style"
        ? objectEntries(value).map(([name]) => name).filter(isVisualProperty)
        : ["className", "buttonClassName", "inputClassName"].includes(property)
          ? strings(value).flatMap((text) => text.split(/\s+/)).filter(isVisualClass)
          : [];
      for (const token of new Set(invalid)) violations.push({
        line: tree.getLineAndCharacterOfPosition(node.getStart(tree)).line + 1,
        control: node.tagName.getText(tree), property, value: token,
      });
    }
  }
  function visit(node) {
    if ((ts.isJsxOpeningElement(node) || ts.isJsxSelfClosingElement(node)) && controls.has(node.tagName.getText(tree))) {
      const props = node.attributes.properties.flatMap((attr) => {
        if (ts.isJsxSpreadAttribute(attr)) return objectEntries(attr.expression);
        const value = attr.initializer;
        return [[attr.name.getText(tree), value && ts.isJsxExpression(value) ? value.expression : value]];
      });
      inspectProps(node, props);
    }
    ts.forEachChild(node, visit);
  }
  visit(tree);
  return violations;
}
