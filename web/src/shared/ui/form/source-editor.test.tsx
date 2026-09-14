// INPUT: Exact Field labels, native typing/composition, refs and read-only/disabled source controls.
// OUTPUT: Shared source input preserves browser editing while keeping field actions independent.
// POS: Primitive behavior regression; real focus paint and geometry belong to browser coverage.

import { createRef, useState } from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { UiButton } from "@/shared/ui/button/button";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { UiField } from "./form-control";
import { UiSourceEditor } from "./source-editor";

it("associates only the exact field and keeps its label action outside the label", async () => {
  const action = vi.fn();
  render(<I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => key }}><UiField htmlFor="source" label="Draft" description="Markdown source" error="Review this change"
    labelAction={<UiButton onClick={action}>Save draft</UiButton>}>
    <UiSourceEditor id="source" defaultValue="line 1\n\tline 2" />
    <UiSourceEditor aria-label="Saved version" defaultValue="saved" readOnly />
  </UiField></I18N_CONTEXT.Provider>);
  const draft = screen.getByRole("textbox", { name: "Draft" });
  const saved = screen.getByRole("textbox", { name: "Saved version" });
  expect(draft.getAttribute("aria-invalid")).toBe("true");
  expect(document.getElementById(draft.getAttribute("aria-errormessage")!)?.textContent).toBe("Review this change");
  expect(saved.getAttribute("aria-invalid")).toBeNull();
  const button = screen.getByRole("button", { name: "Save draft" });
  expect(button.closest("label")).toBeNull();
  await userEvent.click(button); expect(action).toHaveBeenCalledOnce();
  await userEvent.click(screen.getByText("Draft")); expect(document.activeElement).toBe(draft);
});

it("preserves raw newlines, spaces, composition events and native Tab navigation", async () => {
  const composition = vi.fn();
  function Source() {
    const [value, setValue] = useState("a  b\n\t中文");
    return <><UiSourceEditor aria-label="Source" onChange={(event) => setValue(event.target.value)}
      onCompositionEnd={composition} value={value} /><UiButton>Next control</UiButton></>;
  }
  render(<Source />);
  const source = screen.getByRole("textbox") as HTMLTextAreaElement;
  fireEvent.compositionStart(source);
  fireEvent.change(source, { target: { value: "a  b\n\t中文输入" } });
  fireEvent.compositionEnd(source, { data: "输入" });
  expect(composition).toHaveBeenCalledOnce();
  expect(source.value).toBe("a  b\n\t中文输入");
  await userEvent.click(source); await userEvent.tab();
  expect(document.activeElement).toBe(screen.getByRole("button", { name: "Next control" }));
  expect(source.getAttribute("spellcheck")).toBe("false");
});

it("forwards focus refs and preserves native read-only selection and disabled behavior", async () => {
  const ref = createRef<HTMLTextAreaElement>();
  const change = vi.fn();
  render(<><UiSourceEditor aria-label="Saved" defaultValue="saved text" onChange={change} readOnly ref={ref} />
    <UiSourceEditor aria-label="Unavailable" defaultValue="unavailable" disabled onChange={change} /></>);
  ref.current?.focus(); ref.current?.setSelectionRange(0, 5);
  expect(document.activeElement).toBe(ref.current);
  expect(ref.current?.selectionEnd).toBe(5);
  await userEvent.type(screen.getByRole("textbox", { name: "Saved" }), "replace");
  await userEvent.type(screen.getByRole("textbox", { name: "Unavailable" }), "replace");
  expect(change).not.toHaveBeenCalled(); expect(ref.current?.value).toBe("saved text");
});
