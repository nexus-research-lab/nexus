// INPUT: Complete and growing HTML comments in assistant Markdown.
// OUTPUT: Metadata stays hidden while explicit code examples remain readable.
// POS: Shared Markdown rendering and stream-boundary regression.
import { render } from "@testing-library/react";
import { expect, it } from "vitest";
import { MarkdownText } from "../streaming/markdown-streaming";
import { MARKDOWN_PLUGINS, REHYPE_PLUGINS } from "./markdown-renderer-shared";

it("hides complete and streaming comments across empty lines without hiding code examples", () => {
  const props = { components: undefined, streamingComponents: undefined, remarkPlugins: MARKDOWN_PLUGINS, rehypePlugins: REHYPE_PLUGINS, urlTransform: undefined };
  const prefix = "Delivered the report.\n\n<!-- nexus-workgraph-result\n";
  const { container, rerender } = render(<MarkdownText {...props} content={prefix} isStreaming />);
  for (const content of [prefix, prefix + '{"version":1,\n\n"delivery":{"result_summary":"private protocol payload"}}', prefix + '{"version":1}\n-->']) {
    rerender(<MarkdownText {...props} content={content} isStreaming />);
    expect(container.textContent).toBe("Delivered the report.");
  }
  rerender(<MarkdownText {...props} content={prefix + '{"version":1}\n-->'} isStreaming={false} />);
  expect(container.textContent).toBe("Delivered the report.");
  rerender(<MarkdownText {...props} content={'```html\n<!-- example -->\n```\n\n`<!-- inline -->`'} isStreaming={false} />);
  expect(container.textContent).toContain("<!-- example -->");
  expect(container.textContent).toContain("<!-- inline -->");
});
