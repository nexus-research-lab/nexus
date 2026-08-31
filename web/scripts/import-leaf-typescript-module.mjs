import { readFile } from "node:fs/promises";
import path from "node:path";
import { pathToFileURL } from "node:url";

import ts from "typescript";

/**
 * Import a TypeScript leaf module without starting Vite's native dev runtime.
 *
 * These source-level regression tests exercise pure modules whose only imports
 * are erased `import type` declarations. Keeping that boundary explicit avoids
 * both a second application bundler configuration and the rolldown-vite native
 * teardown race seen when short-lived `node:test` workers call `server.close()`.
 */
export async function importLeafTypeScriptModule(webRoot, relativePath) {
  const filePath = path.resolve(webRoot, relativePath);
  const pathWithinRoot = path.relative(webRoot, filePath);
  if (
    pathWithinRoot === ""
    || pathWithinRoot.startsWith(`..${path.sep}`)
    || path.isAbsolute(pathWithinRoot)
  ) {
    throw new Error(`TypeScript test module must be inside the web root: ${relativePath}`);
  }

  const source = await readFile(filePath, "utf8");
  const result = ts.transpileModule(source, {
    compilerOptions: {
      isolatedModules: true,
      jsx: ts.JsxEmit.ReactJSX,
      module: ts.ModuleKind.ESNext,
      target: ts.ScriptTarget.ES2022,
      verbatimModuleSyntax: true,
    },
    fileName: filePath,
    reportDiagnostics: true,
  });
  const errors = result.diagnostics?.filter(
    (diagnostic) => diagnostic.category === ts.DiagnosticCategory.Error,
  ) ?? [];
  if (errors.length > 0) {
    throw new SyntaxError(ts.formatDiagnosticsWithColorAndContext(errors, {
      getCanonicalFileName: (name) => name,
      getCurrentDirectory: () => webRoot,
      getNewLine: () => "\n",
    }));
  }

  const runtimeImport = result.outputText.match(
    /^\s*(?:import\s+(?!\()|export\s+(?:\*|\{)[^;]*\s+from\s+)[^;]+/m,
  );
  if (runtimeImport) {
    throw new Error(
      `TypeScript test module is not a leaf module: ${relativePath} (${runtimeImport[0].trim()})`,
    );
  }

  const sourceUrl = pathToFileURL(filePath).href;
  const moduleSource = `${result.outputText}\n//# sourceURL=${sourceUrl}\n`;
  return import(`data:text/javascript;base64,${Buffer.from(moduleSource).toString("base64")}`);
}
