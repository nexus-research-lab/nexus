// INPUT: TypeScript source files and the repository's @/ alias or relative module references.
// OUTPUT: Normalized import edges and forbidden upward dependencies for the frontend gate.
// POS: Static ownership analysis only; module execution and migration decisions remain outside this model.

import path from "node:path";
import ts from "typescript";

const LAYER_ORDER = ["generated", "shared", "entities", "features", "widgets", "pages", "app", "entries"];

// Existing support directories keep their actual responsibility until their files migrate.
// Mixed support trees do not invent a target layer; their explicit prohibitions still apply.
const LEGACY_LAYERS = { bootstrap: "app", hooks: "entities", store: "entities" };
const LEGACY_SUPPORT_ROOTS = new Set(["lib", "config", "types"]);
const APPLICATION_ROOTS = new Set(["entries", "app", "bootstrap", "pages", "widgets", "features"]);
const TYPE_IMPLEMENTATION_ROOTS = new Set(["config", "lib", "hooks", "store", "entities"]);

function isForbiddenLegacyDependency(from, fromRoot, toRoot, toLayer) {
  if (LEGACY_SUPPORT_ROOTS.has(fromRoot)
    && (APPLICATION_ROOTS.has(toRoot) || APPLICATION_ROOTS.has(toLayer))) return true;
  if (fromRoot === "types" && TYPE_IMPLEMENTATION_ROOTS.has(toRoot)) return true;
  return from.startsWith("src/lib/api/") && toRoot === "store";
}

export function getFrontendLayer(filePath) {
  const normalized = filePath.replaceAll("\\", "/");
  if (normalized.startsWith("src/types/generated/")) return "generated";
  if (normalized === "src/App" || normalized === "src/App.tsx") return "app";
  if (normalized === "src/main" || normalized === "src/main.tsx") return "entries";
  const [root, directory] = normalized.split("/");
  if (root !== "src") return null;
  return LAYER_ORDER.includes(directory) ? directory : LEGACY_LAYERS[directory] ?? null;
}

export function resolveFrontendModule(filePath, specifier) {
  const modulePath = specifier.replace(/[?#].*$/, "");
  let target;
  if (modulePath.startsWith("@/")) {
    target = `src/${modulePath.slice(2)}`;
  } else if (modulePath.startsWith("./") || modulePath.startsWith("../")) {
    target = path.posix.join(path.posix.dirname(filePath.replaceAll("\\", "/")), modulePath);
  } else if (modulePath.startsWith("src/")) {
    target = modulePath;
  } else if (modulePath.startsWith("/src/")) {
    target = modulePath.slice(1);
  } else {
    return null;
  }
  target = path.posix.normalize(target);
  if (!target.startsWith("src/")) return null;
  // Alias, relative, explicit-extension and index imports identify the same module edge.
  return target.replace(/(?:\.d)?\.[cm]?[jt]sx?$/, "").replace(/\/index$/, "");
}

export function collectFrontendModuleReferences(filePath, source) {
  const sourceFile = ts.createSourceFile(filePath, source, ts.ScriptTarget.Latest, true);
  const references = [];
  function addReference(literal, kind) {
    if (!literal || !ts.isStringLiteralLike(literal)) return;
    references.push({
      specifier: literal.text,
      kind,
      line: sourceFile.getLineAndCharacterOfPosition(literal.getStart(sourceFile)).line + 1,
    });
  }
  function visit(node) {
    if (ts.isImportDeclaration(node)) {
      addReference(node.moduleSpecifier, node.importClause ? "import" : "side-effect");
    } else if (ts.isExportDeclaration(node)) {
      addReference(node.moduleSpecifier, "re-export");
    } else if (ts.isImportTypeNode(node) && ts.isLiteralTypeNode(node.argument)) {
      addReference(node.argument.literal, "type-import");
    } else if (ts.isImportEqualsDeclaration(node) && ts.isExternalModuleReference(node.moduleReference)) {
      addReference(node.moduleReference.expression, "import-equals");
    } else if (ts.isCallExpression(node)) {
      if (node.expression.kind === ts.SyntaxKind.ImportKeyword) {
        addReference(node.arguments[0], "dynamic");
      } else if (ts.isIdentifier(node.expression) && node.expression.text === "require") {
        addReference(node.arguments[0], "require");
      }
    }
    ts.forEachChild(node, visit);
  }
  visit(sourceFile);
  return references;
}

export function findFrontendBoundaryViolations(filePath, source) {
  const from = filePath.replaceAll("\\", "/");
  const fromLayer = getFrontendLayer(from);
  const fromRoot = from.split("/")[1];
  if (!fromLayer && !LEGACY_SUPPORT_ROOTS.has(fromRoot)) return [];
  return collectFrontendModuleReferences(from, source).flatMap((reference) => {
    const to = resolveFrontendModule(from, reference.specifier);
    if (!to) return [];
    const toLayer = getFrontendLayer(to);
    const toRoot = to.split("/")[1];
    const upwardLayer = fromLayer && toLayer
      && LAYER_ORDER.indexOf(toLayer) > LAYER_ORDER.indexOf(fromLayer);
    if (!upwardLayer && !isForbiddenLegacyDependency(from, fromRoot, toRoot, toLayer)) return [];
    return [{ from, to, fromLayer: fromLayer ?? fromRoot, toLayer: toLayer ?? toRoot, ...reference }];
  });
}
