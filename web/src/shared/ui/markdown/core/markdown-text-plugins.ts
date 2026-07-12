type MarkdownAstNode = {
  children?: MarkdownAstNode[];
  data?: {
    hName?: string;
    hProperties?: Record<string, unknown>;
  };
  type?: string;
  value?: string;
};

interface MarkdownChildContext {
  children: MarkdownAstNode[];
  index: number;
  node: MarkdownAstNode;
}

interface MarkdownChildReplacement {
  deleteCount: number;
  nodes: MarkdownAstNode[];
}

interface PairedInlineHtmlContent {
  endIndex: number;
  value: string;
}

type MarkdownChildRule = (
  context: MarkdownChildContext,
) => MarkdownChildReplacement | null;

const INLINE_HTML_TAGS = [
  "sub",
  "sup",
  "ins",
  "kbd",
  "b",
  "strong",
  "i",
  "em",
  "mark",
  "del",
  "u",
];

const INLINE_HTML_TAG_PATTERN = new RegExp(`^<(${INLINE_HTML_TAGS.join("|")})>$`, "i");
const INLINE_HTML_COMPLETE_TAG_PATTERN = new RegExp(
  `^<(${INLINE_HTML_TAGS.join("|")})>(.*?)<\\/\\1>$`,
  "is",
);
const ENCODED_INLINE_HTML_TAG_SOURCE =
  `&lt;(${INLINE_HTML_TAGS.join("|")})&gt;(.*?)&lt;\\/\\1&gt;`;

const BREAK_RULES: MarkdownChildRule[] = [
  replaceHtmlBreak,
  replaceTextBreaks,
];

const INLINE_HTML_RULES: MarkdownChildRule[] = [
  replaceCompleteInlineHtml,
  replacePairedInlineHtml,
  replaceEncodedInlineHtml,
];

function createInlineHtmlNode(tagName: string, value: string): MarkdownAstNode {
  return {
    children: [{ type: "text", value }],
    data: {
      hName: tagName.toLowerCase(),
      hProperties: {},
    },
    type: "inlineHtmlTag",
  };
}

function createSingleNodeReplacement(
  node: MarkdownAstNode,
): MarkdownChildReplacement {
  return { deleteCount: 1, nodes: [node] };
}

function visitChildren(
  node: MarkdownAstNode,
  visitor: (node: MarkdownAstNode) => void,
) {
  visitor(node);
  node.children?.forEach((child) => visitChildren(child, visitor));
}

function applyChildRules(
  node: MarkdownAstNode,
  rules: MarkdownChildRule[],
) {
  const children = node.children;
  if (!children) {
    return;
  }

  for (let index = 0; index < children.length; index += 1) {
    const replacement = findChildReplacement({
      children,
      index,
      node: children[index],
    }, rules);
    if (!replacement) {
      continue;
    }
    children.splice(index, replacement.deleteCount, ...replacement.nodes);
    index += replacement.nodes.length - 1;
  }
}

function findChildReplacement(
  context: MarkdownChildContext,
  rules: MarkdownChildRule[],
): MarkdownChildReplacement | null {
  for (const rule of rules) {
    const replacement = rule(context);
    if (replacement) {
      return replacement;
    }
  }
  return null;
}

function splitTextByPattern(
  value: string,
  pattern: RegExp,
  createMatchNode: (match: RegExpExecArray) => MarkdownAstNode,
): MarkdownAstNode[] | null {
  const nodes: MarkdownAstNode[] = [];
  let lastIndex = 0;
  let match: RegExpExecArray | null;

  while ((match = pattern.exec(value)) !== null) {
    if (match.index > lastIndex) {
      nodes.push({ type: "text", value: value.slice(lastIndex, match.index) });
    }
    nodes.push(createMatchNode(match));
    lastIndex = match.index + match[0].length;
  }

  if (nodes.length === 0) {
    return null;
  }
  if (lastIndex < value.length) {
    nodes.push({ type: "text", value: value.slice(lastIndex) });
  }
  return nodes;
}

function replaceHtmlBreak(
  context: MarkdownChildContext,
): MarkdownChildReplacement | null {
  const isBreak = context.node.type === "html"
    && /^\s*<\s*br\s*\/?>\s*$/i.test(context.node.value ?? "");
  return isBreak ? createSingleNodeReplacement({ type: "break" }) : null;
}

function replaceTextBreaks(
  context: MarkdownChildContext,
): MarkdownChildReplacement | null {
  if (context.node.type !== "text") {
    return null;
  }
  const nodes = splitTextByPattern(
    context.node.value ?? "",
    /<\s*br\s*\/?>/gi,
    () => ({ type: "break" }),
  );
  return nodes ? { deleteCount: 1, nodes } : null;
}

function replaceCompleteInlineHtml(
  context: MarkdownChildContext,
): MarkdownChildReplacement | null {
  if (context.node.type !== "html" || !context.node.value) {
    return null;
  }
  const match = INLINE_HTML_COMPLETE_TAG_PATTERN.exec(context.node.value);
  return match
    ? createSingleNodeReplacement(createInlineHtmlNode(match[1], match[2]))
    : null;
}

function replacePairedInlineHtml(
  context: MarkdownChildContext,
): MarkdownChildReplacement | null {
  if (context.node.type !== "html" || !context.node.value) {
    return null;
  }
  const tagName = INLINE_HTML_TAG_PATTERN.exec(context.node.value)?.[1];
  if (!tagName) {
    return null;
  }
  const content = collectPairedInlineHtmlContent(context, tagName);
  return content
    ? {
        deleteCount: content.endIndex - context.index + 1,
        nodes: [createInlineHtmlNode(tagName, content.value)],
      }
    : null;
}

function collectPairedInlineHtmlContent(
  context: MarkdownChildContext,
  tagName: string,
): PairedInlineHtmlContent | null {
  const values: string[] = [];
  const closingTag = `</${tagName.toLowerCase()}>`;

  for (let cursor = context.index + 1; cursor < context.children.length; cursor += 1) {
    const node = context.children[cursor];
    if (node.type === "html" && node.value?.toLowerCase() === closingTag) {
      return { endIndex: cursor, value: values.join("") };
    }
    if (node.type !== "text") {
      return null;
    }
    values.push(node.value ?? "");
  }
  return null;
}

function replaceEncodedInlineHtml(
  context: MarkdownChildContext,
): MarkdownChildReplacement | null {
  if (context.node.type !== "text") {
    return null;
  }
  const nodes = splitTextByPattern(
    context.node.value ?? "",
    new RegExp(ENCODED_INLINE_HTML_TAG_SOURCE, "gis"),
    (match) => createInlineHtmlNode(match[1], match[2]),
  );
  return nodes ? { deleteCount: 1, nodes } : null;
}

export function remarkMarkdownBreaks() {
  return (tree: MarkdownAstNode) => {
    visitChildren(tree, (node) => applyChildRules(node, BREAK_RULES));
  };
}

export function remarkInlineHtmlTags() {
  return (tree: MarkdownAstNode) => {
    visitChildren(tree, (node) => applyChildRules(node, INLINE_HTML_RULES));
  };
}
