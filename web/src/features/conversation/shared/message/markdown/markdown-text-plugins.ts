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

function create_inline_html_node(tag_name: string, value: string): MarkdownAstNode {
  return {
    children: [{ type: "text", value }],
    data: {
      hName: tag_name.toLowerCase(),
      hProperties: {},
    },
    type: "inlineHtmlTag",
  };
}

function replace_child(parent: MarkdownAstNode, index: number, next_nodes: MarkdownAstNode[]) {
  parent.children?.splice(index, 1, ...next_nodes);
}

function visit_children(node: MarkdownAstNode, visitor: (node: MarkdownAstNode) => void) {
  visitor(node);
  node.children?.forEach((child) => visit_children(child, visitor));
}

function split_text_by_br(value: string): MarkdownAstNode[] {
  const nodes: MarkdownAstNode[] = [];
  const br_pattern = /<\s*br\s*\/?>/gi;
  let last_index = 0;
  let match: RegExpExecArray | null;

  while ((match = br_pattern.exec(value)) !== null) {
    if (match.index > last_index) {
      nodes.push({ type: "text", value: value.slice(last_index, match.index) });
    }
    nodes.push({ type: "break" });
    last_index = match.index + match[0].length;
  }

  if (last_index < value.length) {
    nodes.push({ type: "text", value: value.slice(last_index) });
  }

  return nodes;
}

function split_text_by_encoded_inline_html(value: string): MarkdownAstNode[] {
  const nodes: MarkdownAstNode[] = [];
  let last_index = 0;
  let match: RegExpExecArray | null;

  ENCODED_INLINE_HTML_TAG_PATTERN.lastIndex = 0;
  while ((match = ENCODED_INLINE_HTML_TAG_PATTERN.exec(value)) !== null) {
    if (match.index > last_index) {
      nodes.push({ type: "text", value: value.slice(last_index, match.index) });
    }

    nodes.push(create_inline_html_node(match[1], match[2]));
    last_index = match.index + match[0].length;
  }

  if (last_index < value.length) {
    nodes.push({ type: "text", value: value.slice(last_index) });
  }

  return nodes.length > 0 ? nodes : [{ type: "text", value }];
}

export function remarkMarkdownBreaks() {
  return (tree: MarkdownAstNode) => {
    visit_children(tree, (node) => {
      if (!node.children) {
        return;
      }

      for (let index = 0; index < node.children.length; index += 1) {
        const child = node.children[index];
        if (child.type === "html" && /^\s*<\s*br\s*\/?>\s*$/i.test(child.value ?? "")) {
          replace_child(node, index, [{ type: "break" }]);
          continue;
        }

        if (child.type === "text" && /<\s*br\s*\/?>/i.test(child.value ?? "")) {
          const next_nodes = split_text_by_br(child.value ?? "");
          replace_child(node, index, next_nodes);
          index += next_nodes.length - 1;
        }
      }
    });
  };
}

export function remarkInlineHtmlTags() {
  return (tree: MarkdownAstNode) => {
    visit_children(tree, (node) => {
      if (!node.children) {
        return;
      }

      for (let index = 0; index < node.children.length; index += 1) {
        const child = node.children[index];

        if (child.type === "html" && child.value) {
          const complete_tag_match = INLINE_HTML_COMPLETE_TAG_PATTERN.exec(child.value);
          if (complete_tag_match) {
            replace_child(node, index, [
              create_inline_html_node(complete_tag_match[1], complete_tag_match[2]),
            ]);
            continue;
          }

          const start_tag_match = INLINE_HTML_TAG_PATTERN.exec(child.value);
          if (start_tag_match) {
            const tag_name = start_tag_match[1];
            const inline_text_nodes: MarkdownAstNode[] = [];
            let end_index = -1;

            for (let cursor = index + 1; cursor < node.children.length; cursor += 1) {
              const next_child = node.children[cursor];
              if (
                next_child.type === "html" &&
                next_child.value?.toLowerCase() === `</${tag_name.toLowerCase()}>`
              ) {
                end_index = cursor;
                break;
              }

              if (next_child.type !== "text") {
                break;
              }

              inline_text_nodes.push(next_child);
            }

            if (end_index >= 0) {
              node.children.splice(
                index,
                end_index - index + 1,
                create_inline_html_node(
                  tag_name,
                  inline_text_nodes.map((item) => item.value ?? "").join(""),
                ),
              );
            }
          }
        }

        ENCODED_INLINE_HTML_TAG_PATTERN.lastIndex = 0;
        if (child.type === "text" && ENCODED_INLINE_HTML_TAG_PATTERN.test(child.value ?? "")) {
          const next_nodes = split_text_by_encoded_inline_html(child.value ?? "");
          replace_child(node, index, next_nodes);
          index += next_nodes.length - 1;
        }
      }
    });
  };
}
