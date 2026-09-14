// INPUT: Launcher drafts, acceptance/rejection, busy state, IME events and real Mention choices.
// OUTPUT: Shared input retains exact submit/navigation behavior and composition cannot trigger an action.
// POS: Offline Hero interaction regressions; decorative canvas/Lottie rendering is excluded.

import { afterAll, beforeAll, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import type { HeroStageProps } from "../console/launcher-console-types";
import { LauncherHeroStage } from "./launcher-hero-stage";

vi.mock("./pile/launcher-agent-pile", () => ({ AgentPile: () => null }));
vi.mock("@/shared/ui/feedback/lottie-player", () => ({ LottiePlayer: () => null }));
const originalScroll = HTMLElement.prototype.scrollIntoView;
beforeAll(() => { HTMLElement.prototype.scrollIntoView = vi.fn(); });
afterAll(() => { HTMLElement.prototype.scrollIntoView = originalScroll; });

function props(overrides: Partial<HeroStageProps> = {}): HeroStageProps {
  return {
    currentAgentId: null, decorativeTokens: [], mentionTargets: [], recentEntries: [],
    query: "", isQueryLoading: false,
    onEnterHome: vi.fn(), onOpenMainAgentDm: vi.fn(), onQueryChange: vi.fn(),
    onSelectAgent: vi.fn(), onOpenRecentEntry: vi.fn(), onSubmit: vi.fn(() => true),
    ...overrides,
  };
}
function hero(properties: HeroStageProps) {
  return <I18N_CONTEXT.Provider value={{ locale: "zh", setLocale: vi.fn(), t: (key) => MESSAGES.zh[key] }}>
    <LauncherHeroStage {...properties} />
  </I18N_CONTEXT.Provider>;
}
const input = () => screen.getByRole("textbox", { name: MESSAGES.zh["launcher.query_input"] }) as HTMLInputElement;
const send = () => screen.getByRole("button", { name: MESSAGES.zh["launcher.send"] }) as HTMLButtonElement;

it("trims accepted input, clears the draft once and ignores whitespace-only submission", async () => {
  const user = userEvent.setup();
  const properties = props({ query: "  整理资料  " });
  render(hero(properties));
  await user.click(send());
  expect(properties.onSubmit).toHaveBeenCalledExactlyOnceWith("整理资料");
  expect(properties.onQueryChange).toHaveBeenLastCalledWith("");
  expect(input().value).toBe("");
  fireEvent.change(input(), { target: { value: "   " } });
  fireEvent.keyDown(input(), { key: "Enter" });
  expect(properties.onSubmit).toHaveBeenCalledTimes(1);
});

it("keeps a rejected draft and accepts a later external query replacement", () => {
  const properties = props({ query: "保留草稿", onSubmit: vi.fn(() => false) });
  const view = render(hero(properties));
  fireEvent.keyDown(input(), { key: "Enter" });
  expect(properties.onSubmit).toHaveBeenCalledWith("保留草稿");
  expect(input().value).toBe("保留草稿");
  expect(properties.onQueryChange).not.toHaveBeenCalled();
  view.rerender(hero({ ...properties, query: "恢复指令" }));
  expect(input().value).toBe("恢复指令");
});

it("keeps the send name while busy and blocks duplicate clicks", async () => {
  const user = userEvent.setup();
  const properties = props({ query: "等待受理", isQueryLoading: true });
  render(hero(properties));
  expect(send().disabled).toBe(true);
  expect(send().getAttribute("aria-busy")).toBe("true");
  expect(input().disabled).toBe(true);
  await user.click(send());
  expect(properties.onSubmit).not.toHaveBeenCalled();
});

it("uses the shared IME boundary before treating Enter as a submit command", () => {
  const properties = props({ query: "输入中的中文" });
  render(hero(properties));
  fireEvent.compositionStart(input());
  fireEvent.keyDown(input(), { key: "Enter" });
  fireEvent.compositionEnd(input());
  fireEvent.keyDown(input(), { key: "Enter", isComposing: true });
  fireEvent.keyDown(input(), { key: "Enter", keyCode: 229 });
  fireEvent.keyDown(input(), { key: "Process" });
  expect(properties.onSubmit).not.toHaveBeenCalled();
  fireEvent.keyDown(input(), { key: "Enter" });
  expect(properties.onSubmit).toHaveBeenCalledExactlyOnceWith("输入中的中文");
});

it.each([
  { trigger: "@", label: "Nova", expected: "@Nova ", hidden: "Design" },
  { trigger: "#", label: "Design", expected: "#Design ", hidden: "Nova" },
])("inserts a $trigger target after composition without submitting the query", async ({ trigger, label, expected, hidden }) => {
  const properties = props({ mentionTargets: [
    { id: "nova", kind: "agent", label: "Nova", marker: "N" },
    { id: "design", kind: "room", label: "Design", marker: "#" },
  ] });
  render(hero(properties));
  input().focus();
  fireEvent.change(input(), { target: { value: trigger } });
  expect(screen.getByText(label)).toBeTruthy();
  expect(screen.queryByText(hidden)).toBeNull();
  fireEvent.keyDown(input(), { key: "Enter", isComposing: true });
  fireEvent.keyDown(input(), { key: "Enter", keyCode: 229 });
  expect(input().value).toBe(trigger);
  expect(screen.getByText(label)).toBeTruthy();
  expect(properties.onSubmit).not.toHaveBeenCalled();
  fireEvent.keyDown(input(), { key: "Enter" });
  await waitFor(() => expect(input().value).toBe(expected));
  await waitFor(() => expect(input().selectionStart).toBe(expected.length));
  expect(document.activeElement).toBe(input());
  expect(properties.onSubmit).not.toHaveBeenCalled();
});

it("keeps home navigation and draft handoff as independent actions", async () => {
  const user = userEvent.setup();
  const properties = props({ query: "交接草稿" });
  render(hero(properties));
  await user.click(screen.getByRole("button", { name: /进入工作台/ }));
  expect(properties.onEnterHome).toHaveBeenCalledTimes(1);
  await user.click(screen.getByRole("button", { name: MESSAGES.zh["launcher.handoff"] }));
  expect(properties.onOpenMainAgentDm).toHaveBeenCalledExactlyOnceWith("交接草稿");
  expect(properties.onSubmit).not.toHaveBeenCalled();
});
