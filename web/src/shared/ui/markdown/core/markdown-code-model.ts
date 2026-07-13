export interface MarkdownCodeNode {
  position?: {
    start?: { line?: number };
    end?: { line?: number };
  };
}

export type MarkdownCodePresentation =
  | { kind: "inline"; value: string }
  | { kind: "block"; language: string; value: string }
  | { kind: "mermaid"; value: string };

interface MarkdownCodeContext {
  language: string;
  node?: MarkdownCodeNode;
  value: string;
}

const LANGUAGE_CLASS_PATTERN = /language-(\w+)/;
const MERMAID_LANGUAGES = new Set(["mermaid", "mmd"]);

const BLOCK_CODE_RULES: ReadonlyArray<(
  context: MarkdownCodeContext,
) => boolean> = [
  ({ language }) => Boolean(language),
  ({ value }) => value.includes("\n"),
  ({ node }) => {
    const startLine = node?.position?.start?.line;
    const endLine = node?.position?.end?.line;
    return typeof startLine === "number"
      && typeof endLine === "number"
      && startLine !== endLine;
  },
];

export function buildMarkdownCodePresentation(
  node: MarkdownCodeNode | undefined,
  className: string | undefined,
  children: unknown,
): MarkdownCodePresentation {
  const normalizedClassName = className ?? "";
  const language = LANGUAGE_CLASS_PATTERN.exec(normalizedClassName)?.[1]
    ?.toLowerCase() ?? "";
  const context: MarkdownCodeContext = {
    language,
    node,
    value: String(children).replace(/\n$/, ""),
  };

  if (!BLOCK_CODE_RULES.some((matches) => matches(context))) {
    return { kind: "inline", value: context.value };
  }
  return MERMAID_LANGUAGES.has(language)
    ? { kind: "mermaid", value: context.value }
    : { kind: "block", language: language || "text", value: context.value };
}
