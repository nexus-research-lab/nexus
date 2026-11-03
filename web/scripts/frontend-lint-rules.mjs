// INPUT: ESLint's parsed production modules and repository-relative ownership.
// OUTPUT: Editor and CI diagnostics for dependency, file contract and shared style ownership.
// POS: Static engineering rules; does not assert component implementation or visual output.
import path from "node:path";
import { fileURLToPath } from "node:url";
import { findFrontendBoundaryViolations } from "./frontend-dependency-model.mjs";
import { GOVERNED_ROOTS, getFileContractErrors, isProductionTypeScriptFile } from "./frontend-file-contract.mjs";
const webRoot = fileURLToPath(new URL("..", import.meta.url));
function relativeFile(context) { return path.relative(webRoot, context.filename).split(path.sep).join("/"); }
const meta = { type: "problem", schema: [] };
export const frontendRules = {
  "dependency-direction": {
    meta,
    create(context) {
      return { Program(node) {
        for (const edge of findFrontendBoundaryViolations(relativeFile(context), context.sourceCode.text)) {
          context.report({ node, loc: { line: edge.line, column: 0 }, message: `Upward dependency: ${edge.from} -> ${edge.to} (${edge.kind}). Move the dependency to its owner.` });
        }
      } };
    },
  },
  "file-contract": {
    meta,
    create(context) {
      const file = relativeFile(context);
      if (!isProductionTypeScriptFile(file) || !GOVERNED_ROOTS.some(root => file.startsWith(root + "/"))) return {};
      return { Program(node) {
        for (const message of getFileContractErrors(context.sourceCode.text)) context.report({ node, message });
      } };
    },
  },
  "semantic-style-owners": {
    meta,
    create(context) {
      const file = relativeFile(context);
      if (!isProductionTypeScriptFile(file) || !/^src\/(features|pages)\//.test(file)) return {};
      function check(node, value) {
        if (typeof value !== "string") return;
        if (/(?:drop-)?shadow-\[[^\]]+\]|\bz-\[\d+\]/.test(value)) {
          context.report({ node, message: "Use shared semantic elevation and layer recipes instead of arbitrary shadows or numeric z-index utilities." });
        }
      }
      return { Literal: node => check(node, node.value), TemplateElement: node => check(node, node.value.cooked) };
    },
  },
};
