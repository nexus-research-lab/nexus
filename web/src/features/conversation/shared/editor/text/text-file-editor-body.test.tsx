// INPUT: Preview mode/Agent, loading state, blur policy and real source editor events.
// OUTPUT: Named keyboard scrolling and exact renderer props preserve each edit lifecycle.
// POS: Body composition regression; renderer internals, resource reads and saving stay separate.

import { useState } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { TextFileEditorBody } from "./text-file-editor-body";
import { TextFileContent } from "./text-file-content";

vi.mock("./text-file-content", () => ({ TextFileContent: vi.fn(() => <div>Preview boundary</div>) }));
beforeEach(() => {
  vi.clearAllMocks();
  vi.stubGlobal("ResizeObserver", class { observe() {} disconnect() {} });
});
afterEach(() => vi.unstubAllGlobals());

it.each([undefined, false])("preserves source focus and exitEditingOnBlur=%s", async (exitEditingOnBlur) => {
  const editing = vi.fn();
  function Body() {
    const [content, setContent] = useState("original  text\n\t中文");
    return <I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => key }}>
      <TextFileEditorBody agentId="agent" content={content} exitEditingOnBlur={exitEditingOnBlur}
        fileName="AGENTS.md" fileType="markdown" isLoading={false} isStreaming={false}
        mode="editing" setContent={setContent} setIsEditing={editing} />
      <UiButton>After editor</UiButton>
    </I18N_CONTEXT.Provider>;
  }
  render(<Body />);
  const source = screen.getByRole("textbox") as HTMLTextAreaElement;
  expect(document.activeElement).toBe(source);
  await userEvent.clear(source); await userEvent.type(source, "line  1{Enter}line 2");
  expect(source.value).toBe("line  1\nline 2");
  expect(editing).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole("button", { name: "After editor" }));
  if (exitEditingOnBlur === false) expect(editing).not.toHaveBeenCalled();
  else expect(editing).toHaveBeenCalledExactlyOnceWith(false);
});

it.each(["html", "preview"] as const)("keeps %s preview ownership and scopes keyboard scrolling to the outer text viewport", async (mode) => {
  render(<TextFileEditorBody agentId="file-agent" content="exact content" fileName="report.html" fileType="html"
    isLoading={false} isStreaming mode={mode} setContent={vi.fn()} setIsEditing={vi.fn()} />);
  expect(TextFileContent).toHaveBeenLastCalledWith(expect.objectContaining({
    agentId: "file-agent", content: "exact content", fileName: "report.html", fileType: "html", isStreaming: mode === "html",
  }), undefined);
  if (mode === "preview") {
    const viewport = screen.getByRole("region", { name: "report.html" });
    await userEvent.tab();
    expect(document.activeElement).toBe(viewport);
  } else {
    expect(screen.queryByRole("region")).toBeNull();
  }
});

it("does not expose a loading editor as writable or autofocus a disabled field", async () => {
  const setContent = vi.fn();
  render(<I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => key }}>
    <TextFileEditorBody agentId="file-agent" content="stale" fileName="report.txt" fileType="text"
      isLoading isStreaming={false} mode="editing" setContent={setContent} setIsEditing={vi.fn()} />
  </I18N_CONTEXT.Provider>);
  const editor = screen.getByRole("textbox") as HTMLTextAreaElement;
  expect(editor.disabled).toBe(true);
  expect(editor.value).toBe("workspace_file.loading");
  expect(document.activeElement).not.toBe(editor);
  await userEvent.type(editor, "replacement");
  expect(setContent).not.toHaveBeenCalled();
});
