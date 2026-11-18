// INPUT: 流式追加使尾块收口为已完成块，以及 runtime 终态。
// OUTPUT: 已展示区块的 DOM 节点与代码块外壳不因 streaming → revealed 被替换。
// POS: Markdown 区块身份回归；防止静态/流式两张组件表让 React 重挂载整块。
import { render } from "@testing-library/react";
import type { ReactNode } from "react";
import { expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { createMarkdownComponents } from "../core/markdown-components";
import { MARKDOWN_PLUGINS, REHYPE_PLUGINS } from "../core/markdown-renderer-shared";
import { MarkdownText } from "./markdown-streaming";

const props = {
  components: createMarkdownComponents(() => null),
  rehypePlugins: REHYPE_PLUGINS,
  remarkPlugins: MARKDOWN_PLUGINS,
  urlTransform: (url: string) => url,
};

it("keeps finished paragraphs mounted when the tail block closes", () => {
  const view = render(<MarkdownText {...props} isStreaming content="第一段" />);
  const paragraph = view.container.querySelector("[data-markdown-anchor]");
  view.rerender(<MarkdownText {...props} isStreaming content={"第一段\n\n第二"} />);
  expect(view.container.querySelector("[data-markdown-anchor]")).toBe(paragraph);
  view.rerender(<MarkdownText {...props} isStreaming={false} content={"第一段\n\n第二段"} />);
  expect(view.container.querySelector("[data-markdown-anchor]")).toBe(paragraph);
});

const i18n = ({ children }: { children: ReactNode }) => (
  <I18N_CONTEXT.Provider value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}>{children}</I18N_CONTEXT.Provider>
);

// CodeBlock 内部从流式纯文本切到延迟高亮时同几何替换外壳；这里锁住 Markdown 组件表负责的外层身份。
it("keeps the code block wrapper mounted while the block closes", () => {
  const view = render(<MarkdownText {...props} isStreaming content={"```ts\nconst a = 1;\n"} />, { wrapper: i18n });
  const shell = view.container.firstElementChild;
  expect(view.container.textContent).toContain("markdown.code.streaming");
  view.rerender(<MarkdownText {...props} isStreaming content={"```ts\nconst a = 1;\n```\n\n后续"} />);
  expect(view.container.textContent).not.toContain("markdown.code.streaming");
  expect(view.container.firstElementChild).toBe(shell);
});
