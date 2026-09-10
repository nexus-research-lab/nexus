// INPUT: Plain string content with the timeline requested but no custom classes.
// OUTPUT: Timeline presence depends solely on its explicit control.
// POS: Content entry behavior regression.
import { render } from "@testing-library/react";
import { expect, it } from "vitest";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { ContentRenderer } from "./content-renderer";
it("honors string content timeline without requiring a className", () => {
  const { container, rerender } = render(<I18nProvider><ContentRenderer content="Hello" showTimelineDots /></I18nProvider>);
  expect(container.querySelector("[data-timeline-dot]")).toBeTruthy();
  rerender(<I18nProvider><ContentRenderer content="Hello" /></I18nProvider>);
  expect(container.querySelector("[data-timeline-dot]")).toBeNull();
});
