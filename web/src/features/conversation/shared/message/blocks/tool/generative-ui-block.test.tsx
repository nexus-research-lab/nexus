// INPUT: An isolated widget reporting failure and explicit user retry.
// OUTPUT: Retry creates a fresh iframe instead of replaying the failed final HTML cache.
// POS: Generative UI recovery regression without browser or network access.
import { act, fireEvent, render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { GenerativeUIBlock } from "./generative-ui-block";
import { GENERATIVE_UI_ERROR_MESSAGE, GENERATIVE_UI_MESSAGE_SOURCE } from "./generative-ui-document";
vi.mock("@/shared/theme/theme-context", () => ({ useTheme: () => ({ theme: "light" }) }));
it("replaces the failed iframe on retry and rejects messages from its old window", async () => {
  const { container } = render(<I18nProvider><GenerativeUIBlock complete toolUse={{ type: "tool_use", id: "widget", name: "show_widget", input: { widget_code: "<p>Hello</p>" } }} /></I18nProvider>);
  const frame = container.querySelector("iframe")!;
  const source = frame.contentWindow;
  const fail = () => window.dispatchEvent(new MessageEvent("message", { source, data: { source: GENERATIVE_UI_MESSAGE_SOURCE, type: GENERATIVE_UI_ERROR_MESSAGE, message: "Failed" } }));
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
  act(fail);
  expect(container.querySelector('[data-generative-ui-status="error"]')).toBeTruthy();
  expect(container.querySelector("[data-generative-ui-error-overlay]")).toBeTruthy();
  expect(container.querySelectorAll("iframe")).toHaveLength(1);
  fireEvent.click(screen.getByRole("button", { name: /Display again|重新显示|重试/i }));
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
  expect(container.querySelector("iframe")).not.toBe(frame);
  expect(container.querySelector("iframe")?.getAttribute("sandbox")).toBe("allow-scripts");
  act(fail);
  expect(container.querySelector('[data-generative-ui-status="loading"]')).toBeTruthy();
});

it("keeps the streaming placeholder at the iframe initial height", () => {
  const { container } = render(
    <I18nProvider>
      <GenerativeUIBlock
        complete={false}
        toolUse={{ type: "tool_use", id: "widget", name: "show_widget", input: {} }}
      />
    </I18nProvider>,
  );

  const placeholder = Array.from(container.querySelectorAll("span[aria-hidden='true']"))
    .find((element) => element.className.includes("w-full")) as HTMLElement | undefined;
  expect(placeholder?.style.height).toBe("320px");
});

it("keeps a missing widget error in the reserved content slot", () => {
  const { container } = render(
    <I18nProvider>
      <GenerativeUIBlock
        complete
        toolUse={{ type: "tool_use", id: "widget", name: "show_widget", input: {} }}
      />
    </I18nProvider>,
  );

  const errorSurface = container.querySelector("[data-generative-ui-error-overlay]") as HTMLElement | null;
  expect(errorSurface?.style.height).toBe("320px");
  expect(errorSurface?.textContent).toContain("No interactive content to display");
});
