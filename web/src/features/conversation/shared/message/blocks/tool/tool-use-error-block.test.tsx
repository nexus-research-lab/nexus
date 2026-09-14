// INPUT: Provider failure text with multiple diagnostic issues.
// OUTPUT: Shared error surface preserves every issue and exact tool identity.
// POS: Tool invocation error rendering regression.
import { render, screen } from "@testing-library/react";
import { expect, it } from "vitest";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { ToolUseErrorBlock } from "./tool-use-error-block";
it("preserves provider issues in a shared compact error notice", () => {
  render(<I18nProvider><ToolUseErrorBlock content={"ValidationError: Read failed due to the following issues:\nPath missing\nMode invalid"} /></I18nProvider>);
  const notice = screen.getByRole("status");
  expect(notice.getAttribute("data-inline-notice-tone")).toBe("danger");
  expect(notice.getAttribute("data-inline-notice-width")).toBe("compact");
  expect(screen.getByText("Path missing")).toBeTruthy();
  expect(screen.getByText("Mode invalid")).toBeTruthy();
  expect(screen.getByText("ValidationError")).toBeTruthy();
});
