import ts from "typescript";

export const GOVERNED_ROOTS = [
  ...["button", "dialog", "form", "list", "menu", "navigation", "overlay", "typography", "workspace/catalog"]
    .map((owner) => `src/shared/ui/${owner}`),
  "src/shared/lib",
  "src/shared/navigation",
];
const CONTRACT_FIELDS = ["INPUT", "OUTPUT", "POS"];

export function isProductionTypeScriptFile(fileName) {
  return /\.tsx?$/.test(fileName) && !/(?:\.(?:test|spec)\.tsx?|\.d\.ts)$/.test(fileName);
}

export function getFileContractErrors(source) {
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
