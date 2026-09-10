// INPUT: 流式原文、语言与延迟加载的高亮模块。
// OUTPUT: 流式和加载占位保留完整代码及本地化状态，不出现重复代码块。
// POS: CodeBlock 入口回归；复制与高亮行为由 CodeBlockContent 测试负责。
import { render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { CodeBlock } from "./code-block";
vi.mock("./code-block-content", () => new Promise(() => {}));
it("shares the plain presentation for streaming updates and pending highlighting", () => {
  const renderCode = (value: string, isStreaming: boolean) => (
    <I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => key }}>
      <CodeBlock language="ts" value={value} isStreaming={isStreaming} />
    </I18N_CONTEXT.Provider>
  );
  const { container, rerender } = render(renderCode("  const x =", true));
  expect(screen.getByText("markdown.code.streaming")).toBeTruthy();
  expect(container.querySelector("pre")?.textContent).toBe("  const x =");
  rerender(renderCode("  const x = 1;\n", true));
  expect(container.querySelector("pre")?.textContent).toBe("  const x = 1;\n");
  rerender(renderCode("  const x = 1;\n", false));
  expect(screen.getByText("common.loading")).toBeTruthy();
  expect(container.querySelectorAll("pre")).toHaveLength(1);
  expect(container.querySelector("pre")?.textContent).toBe("  const x = 1;\n");
});
