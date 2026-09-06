// INPUT: Workspace versus explicit-save blur policy and real source editor events.
// OUTPUT: Source focus/typing preserve each consumer's existing edit lifecycle.
// POS: Body composition regression; previews, resource reads and saving are outside this boundary.

import { useState } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { TextFileEditorBody } from "./text-file-editor-body";

vi.mock("./text-file-content", () => ({ TextFileContent: () => <div>Preview boundary</div> }));
beforeEach(() => vi.stubGlobal("ResizeObserver", class { observe() {} disconnect() {} }));
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
