// INPUT: Exact Agent/path selection, chrome updates and local editor drafts.
// OUTPUT: Chrome updates preserve the renderer; file/Agent changes and close discard its draft.
// POS: Preview scope boundary tests; the renderer is a local stateful fixture, without file I/O.
import { useState, type ComponentProps } from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { MESSAGES, type Locale } from "@/shared/i18n/messages";
import { WorkspaceFilePreviewPanel } from "./workspace-file-preview-panel";
import type { WorkspaceFilePreviewProps } from "./workspace-file-preview-types";

vi.mock("./workspace-file-preview-router", () => ({
  WorkspaceFilePreviewRouter: function Preview({ agentId, fileName, path }: WorkspaceFilePreviewProps) {
    const [draft, setDraft] = useState(`${agentId}:${path}`);
    return <input aria-label={fileName} onChange={(event) => setDraft(event.target.value)} value={draft} />;
  },
}));
const file: ComponentProps<typeof WorkspaceFilePreviewPanel> = {
  agentId: "agent-a", path: "one/report.md", headerLocationSegments: ["Agent A", "one"],
  isPreviewFocused: false, onTogglePreviewFocus: vi.fn(),
};
function panel(overrides: Partial<typeof file> = {}, locale: Locale = "en") {
  return <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t: (key) => MESSAGES[locale][key] }}>
    <WorkspaceFilePreviewPanel {...file} {...overrides} />
  </I18N_CONTEXT.Provider>;
}

it.each([{ path: "two/report.md" }, { agentId: "agent-b" }])("preserves drafts for chrome updates and resets on exact scope change: %j", (change) => {
  const view = render(panel());
  const original = screen.getByRole("textbox", { name: "report.md" }) as HTMLInputElement;
  fireEvent.change(original, { target: { value: "unsaved local draft" } });
  view.rerender(panel({ isPreviewFocused: true, headerLocationSegments: ["renamed Agent", "one"] }, "zh"));
  expect(screen.getByRole("textbox")).toBe(original);
  expect(original.value).toBe("unsaved local draft");
  view.rerender(panel(change));
  const replacement = screen.getByRole("textbox") as HTMLInputElement;
  expect(replacement).not.toBe(original);
  expect(replacement.value).toBe(`${change.agentId ?? file.agentId}:${change.path ?? file.path}`);
  view.rerender(panel());
  expect((screen.getByRole("textbox") as HTMLInputElement).value).toBe("agent-a:one/report.md");
});

it("closes the renderer into the current language's shared empty state and reopens fresh", () => {
  const view = render(panel());
  fireEvent.change(screen.getByRole("textbox"), { target: { value: "unsaved" } });
  view.rerender(panel({ path: null }));
  expect(screen.queryByRole("textbox")).toBeNull();
  expect(screen.getByText(MESSAGES.en["room.workspace_preview_title"])).toBeTruthy();
  view.rerender(panel({ path: null }, "zh"));
  expect(screen.getByText(MESSAGES.zh["room.workspace_preview_empty_description"])).toBeTruthy();
  view.rerender(panel());
  expect((screen.getByRole("textbox") as HTMLInputElement).value).toBe("agent-a:one/report.md");
});
