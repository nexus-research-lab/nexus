type MarkdownAstNode = {
  children?: MarkdownAstNode[];
  data?: {
    hName?: string;
    hProperties?: Record<string, unknown>;
  };
  type?: string;
  value?: string;
};

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
const ENCODED_INLINE_HTML_TAG_PATTERN = new RegExp(
  `&lt;(${INLINE_HTML_TAGS.join("|")})&gt;(.*?)&lt;\\/\\1&gt;`,
  "gis",
);

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

function replaceChild(parent: MarkdownAstNode, index: number, nextNodes: MarkdownAstNode[]) {
  parent.children?.splice(index, 1, ...nextNodes);
}

function visitChildren(node: MarkdownAstNode, visitor: (node: MarkdownAstNode) => void) {
  visitor(node);
  node.children?.forEach((child) => visitChildren(child, visitor));
}

function splitTextByBr(value: string): MarkdownAstNode[] {
  const nodes: MarkdownAstNode[] = [];
  const brPattern = /<\s*br\s*\/?>/gi;
  let lastIndex = 0;
  let match: RegExpExecArray | null;

  while ((match = brPattern.exec(value)) !== null) {
    if (match.index > lastIndex) {
      nodes.push({ type: "text", value: value.slice(lastIndex, match.index) });
    }
    nodes.push({ type: "break" });
    lastIndex = match.index + match[0].length;
  }

  if (lastIndex < value.length) {
    nodes.push({ type: "text", value: value.slice(lastIndex) });
  }

  return nodes;
}

function splitTextByEncodedInlineHtml(value: string): MarkdownAstNode[] {
  const nodes: MarkdownAstNode[] = [];
  let lastIndex = 0;
  let match: RegExpExecArray | null;

  ENCODED_INLINE_HTML_TAG_PATTERN.lastIndex = 0;
  while ((match = ENCODED_INLINE_HTML_TAG_PATTERN.exec(value)) !== null) {
    if (match.index > lastIndex) {
      nodes.push({ type: "text", value: value.slice(lastIndex, match.index) });
    }

    nodes.push(createInlineHtmlNode(match[1], match[2]));
    lastIndex = match.index + match[0].length;
  }

  if (lastIndex < value.length) {
    nodes.push({ type: "text", value: value.slice(lastIndex) });
  }

  return nodes.length > 0 ? nodes : [{ type: "text", value }];
}

export function remarkMarkdownBreaks() {
  return (tree: MarkdownAstNode) => {
    visitChildren(tree, (node) => {
      if (!node.children) {
        return;
      }

      for (let index = 0; index < node.children.length; index += 1) {
        const child = node.children[index];
        if (child.type === "html" && /^\s*<\s*br\s*\/?>\s*$/i.test(child.value ?? "")) {
          replaceChild(node, index, [{ type: "break" }]);
          continue;
        }

        if (child.type === "text" && /<\s*br\s*\/?>/i.test(child.value ?? "")) {
          const nextNodes = splitTextByBr(child.value ?? "");
          replaceChild(node, index, nextNodes);
          index += nextNodes.length - 1;
        }
      }
    });
  };
}

export function remarkInlineHtmlTags() {
  return (tree: MarkdownAstNode) => {
    visitChildren(tree, (node) => {
      if (!node.children) {
        return;
      }

      for (let index = 0; index < node.children.length; index += 1) {
        const child = node.children[index];

        if (child.type === "html" && child.value) {
          const completeTagMatch = INLINE_HTML_COMPLETE_TAG_PATTERN.exec(child.value);
          if (completeTagMatch) {
            replaceChild(node, index, [
              createInlineHtmlNode(completeTagMatch[1], completeTagMatch[2]),
            ]);
            continue;
          }

          const startTagMatch = INLINE_HTML_TAG_PATTERN.exec(child.value);
          if (startTagMatch) {
            const tagName = startTagMatch[1];
            const inlineTextNodes: MarkdownAstNode[] = [];
            let endIndex = -1;

            for (let cursor = index + 1; cursor < node.children.length; cursor += 1) {
              const nextChild = node.children[cursor];
              if (
                nextChild.type === "html" &&
                nextChild.value?.toLowerCase() === `</${tagName.toLowerCase()}>`
              ) {
                endIndex = cursor;
                break;
              }

              if (nextChild.type !== "text") {
                break;
              }

              inlineTextNodes.push(nextChild);
            }

            if (endIndex >= 0) {
              node.children.splice(
                index,
                endIndex - index + 1,
                createInlineHtmlNode(
                  tagName,
                  inlineTextNodes.map((item) => item.value ?? "").join(""),
                ),
              );
            }
          }
        }

        ENCODED_INLINE_HTML_TAG_PATTERN.lastIndex = 0;
        if (child.type === "text" && ENCODED_INLINE_HTML_TAG_PATTERN.test(child.value ?? "")) {
          const nextNodes = splitTextByEncodedInlineHtml(child.value ?? "");
          replaceChild(node, index, nextNodes);
          index += nextNodes.length - 1;
        }
      }
    });
  };
}
