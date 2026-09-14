// INPUT: Independent tag drafts, keyboard composition and confirmed values.
// OUTPUT: IME-safe addition, exact removal, duplicate handling and scope reset.
// POS: Agent tag field regression; business/vibe persistence stays separate.

import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import { IdentityTags } from "./identity-tags";

function TagsFixture({ scope = "agent-a" }: { scope?: string }) {
  const [business, setBusiness] = useState(["Research"]);
  const [vibe, setVibe] = useState(["Concise"]);
  return <I18nProvider>
    <IdentityTags addLabel="Add business tag" label="Business tags" onChange={setBusiness} resetKey={`${scope}:business`} tags={business} />
    <IdentityTags addLabel="Add style tag" label="Style tags" onChange={setVibe} resetKey={`${scope}:vibe`} tags={vibe} />
  </I18nProvider>;
}

it.each([{ isComposing: true }, { keyCode: 229 }])("keeps IME confirmation in the draft: %j", async (composition) => {
  const onChange = vi.fn();
  render(<I18nProvider><IdentityTags addLabel="Add" label="Tags" onChange={onChange} resetKey="one" tags={[]} /></I18nProvider>);
  const input = screen.getByRole("textbox", { name: "Tags" });
  await userEvent.type(input, "研究");
  expect(fireEvent.keyDown(input, { key: "Enter", ...composition })).toBe(true);
  expect(onChange).not.toHaveBeenCalled();
  expect((input as HTMLInputElement).value).toBe("研究");
  fireEvent.keyDown(input, { key: "Enter" });
  expect(onChange).toHaveBeenCalledWith(["研究"]);
});

it("keeps tag sets independent and restores typing focus after add/remove", async () => {
  render(<TagsFixture />);
  const business = screen.getByRole("textbox", { name: "Business tags" });
  const style = screen.getByRole("textbox", { name: "Style tags" });
  await userEvent.type(business, "  Planning  ");
  await userEvent.type(style, "Warm");
  await userEvent.click(screen.getByRole("button", { name: "Add business tag" }));
  expect(document.activeElement).toBe(business);
  expect(screen.getByText("Planning")).toBeTruthy();
  expect((style as HTMLInputElement).value).toBe("Warm");
  await userEvent.type(business, "Planning{Enter}");
  expect(screen.getAllByText("Planning")).toHaveLength(1);
  expect((business as HTMLInputElement).value).toBe("");
  const remove = screen.getByRole("button", { name: /Planning/ });
  await userEvent.click(remove);
  expect(screen.queryByText("Planning")).toBeNull();
  expect(document.activeElement).toBe(business);
  expect(screen.getByText("Concise")).toBeTruthy();
});

it("discards only unfinished drafts when the Agent scope changes", async () => {
  const { rerender } = render(<TagsFixture />);
  await userEvent.type(screen.getByRole("textbox", { name: "Business tags" }), "draft");
  await userEvent.type(screen.getByRole("textbox", { name: "Style tags" }), "another draft");
  rerender(<TagsFixture scope="agent-b" />);
  expect(screen.getAllByRole("textbox").every((input) => (input as HTMLInputElement).value === "")).toBe(true);
  expect(screen.getByText("Research")).toBeTruthy();
  expect(screen.getByText("Concise")).toBeTruthy();
});

it.each(["en", "zh"] as const)("localizes the exact remove action in %s", (locale) => {
  render(<I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t: (key, params) => MESSAGES[locale][key].replace("{tag}", String(params?.tag ?? "")) }}>
    <IdentityTags addLabel="Add" label="Tags" onChange={() => undefined} resetKey="one" tags={["Research"]} />
  </I18N_CONTEXT.Provider>);
  expect(screen.getByRole("button", { name: locale === "en" ? "Remove Research" : "移除 Research" })).toBeTruthy();
});
