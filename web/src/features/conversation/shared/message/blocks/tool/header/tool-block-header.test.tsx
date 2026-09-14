// INPUT: Real tool header with nested copy control and keyboard activation.
// OUTPUT: Disclosure state is accessible and nested actions do not collapse details.
// POS: Shared tool header interaction regression.
import { useState } from "react";
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { buildToolBlockViewModel } from "../tool-block-model";
import { ToolBlockHeader } from "./tool-block-header";

it("exposes disclosure state and keeps keyboard copy independent from the row", async () => {
  const user = userEvent.setup();
  const copy = vi.fn();
  const model = buildToolBlockViewModel({
    localization: { locale: "en", t: (key) => key },
    status: "success",
    toolUse: { type: "tool_use", id: "read", name: "Read", input: { file_path: "report.md" } },
    toolResult: { type: "tool_result", tool_use_id: "read", content: "Report content" },
  });
  function Header() {
    const [expanded, setExpanded] = useState(false);
    return <I18nProvider><ToolBlockHeader copied={false} interactionDisabled={false}
      isExpanded={expanded} model={model} onCopyResult={copy}
      onToggle={() => setExpanded((value) => !value)} /></I18nProvider>;
  }
  render(<Header />);
  const row = screen.getByRole("button", { expanded: false });
  act(() => row.focus());
  await user.keyboard("{Enter}");
  expect(row.getAttribute("aria-expanded")).toBe("true");
  const copyButton = screen.getByRole("button", { name: /copy result|复制结果/i });
  act(() => copyButton.focus());
  await user.keyboard("{Enter}");
  await user.keyboard(" ");
  expect(copy).toHaveBeenCalledTimes(2);
  expect(row.getAttribute("aria-expanded")).toBe("true");
  act(() => row.focus());
  await user.keyboard(" ");
  expect(row.getAttribute("aria-expanded")).toBe("false");
});
