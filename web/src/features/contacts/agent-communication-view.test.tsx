// INPUT: Selected contact, external directory changes and pending removal state.
// OUTPUT: Exact confirmation identity, busy close lock and isolated late completion.
// POS: Real communication Header/confirmation regression; Composer internals are outside this test.

import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentProps } from "react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import type { AgentContact } from "@/types/agent/agent";
import { AgentCommunicationView } from "./agent-communication-view";

const followState = vi.hoisted(() => ({ visible: false, scrollToBottom: vi.fn() }));
vi.mock("@/features/conversation/shared/timeline/scroll/use-follow-scroll", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/features/conversation/shared/timeline/scroll/use-follow-scroll")>();
  return { useFollowScroll: (options: Parameters<typeof actual.useFollowScroll>[0]) => ({
    ...actual.useFollowScroll(options),
    showScrollToBottom: followState.visible,
    scrollToBottom: followState.scrollToBottom,
  }) };
});

vi.mock("@/features/conversation/shared/composer/composer-panel", () => ({
  ComposerPanel: () => <div>Composer boundary</div>,
}));

// jsdom has no native media-query surface; keep this confirmation fixture on desktop.
beforeEach(() => vi.stubGlobal("matchMedia", (media: string) => ({
  media, matches: false, onchange: null,
  addEventListener() {}, removeEventListener() {}, addListener() {}, removeListener() {}, dispatchEvent: () => true,
})));
afterEach(() => {
  vi.unstubAllGlobals();
  followState.visible = false;
  followState.scrollToBottom.mockClear();
});

const contacts: AgentContact[] = ["Alpha", "Beta"].map((name) => ({
  id: name, contact_agent_id: name, owner_agent_id: "owner", name,
  created_at: "2026-09-06", updated_at: "2026-09-06",
}));
const base: ComponentProps<typeof AgentCommunicationView> = {
  agent: { agent_id: "owner", name: "Owner", created_at: 1, options: {}, status: "idle", workspace_path: "/workspace/owner" },
  agents: [],
  state: {
    contacts, conversationId: null, conversationFailure: null, directEvents: [], directoryFailure: null,
    hasMoreHistory: false, historyPrependToken: 0, isDirectoryLoading: false, isHistoryLoading: false,
    isMessagesLoading: false, isRemoving: false, isSending: false, mutationFailure: null, pendingAgentId: null,
    roomContexts: [], selectedContactId: "Alpha",
  },
  onAddContact: vi.fn(async () => false), onBackToDirectory: vi.fn(), onClearMutationFailure: vi.fn(),
  onCreateConversation: vi.fn(async () => null), onLoadOlderMessages: vi.fn(async () => false), onRefresh: vi.fn(),
  onRemoveContact: vi.fn(async () => false), onSelectContact: vi.fn(), onSelectConversation: vi.fn(), onSendMessage: vi.fn(async () => undefined),
};

function view(overrides: Partial<typeof base> = {}) {
  return <I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key, params) => `${key}${params ? ` ${Object.values(params).join(" ")}` : ""}` }}>
    <AgentCommunicationView {...base} {...overrides} />
  </I18N_CONTEXT.Provider>;
}

it("locks confirmation while removing and retains it when the operation did not complete", async () => {
  let finish!: (removed: boolean) => void;
  const onRemoveContact = vi.fn(() => new Promise<boolean>((resolve) => { finish = resolve; }));
  const { rerender } = render(view({ onRemoveContact }));
  await userEvent.click(screen.getByRole("button", { name: "agent_options.contact.remove_friend" }));
  const dialog = screen.getByRole("dialog");
  await userEvent.click(within(dialog).getByRole("button", { name: "agent_options.contact.remove_friend" }));
  rerender(view({ onRemoveContact, state: { ...base.state, isRemoving: true } }));
  expect(within(dialog).getAllByRole("button").every((button) => (button as HTMLButtonElement).disabled)).toBe(true);
  await userEvent.keyboard("{Escape}");
  expect(screen.getByRole("dialog")).toBe(dialog);
  expect(onRemoveContact).toHaveBeenCalledExactlyOnceWith("Alpha");
  await act(async () => finish(false));
  rerender(view({ onRemoveContact }));
  expect(screen.getByRole("dialog")).toBe(dialog);
});

it("keeps the original confirmation target when the live selected contact changes", async () => {
  const onRemoveContact = vi.fn(async () => true);
  const { rerender } = render(view({ onRemoveContact }));
  await userEvent.click(screen.getByRole("button", { name: "agent_options.contact.remove_friend" }));
  rerender(view({ onRemoveContact, state: { ...base.state, selectedContactId: "Beta" } }));
  const dialog = screen.getByRole("dialog");
  expect(within(dialog).getByText("agent_options.contact.remove_friend_confirm Alpha")).toBeTruthy();
  await userEvent.click(within(dialog).getByRole("button", { name: "agent_options.contact.remove_friend" }));
  expect(onRemoveContact).toHaveBeenCalledExactlyOnceWith("Alpha");
});

it("does not close a new Agent's confirmation when an old removal finishes", async () => {
  let finish!: (removed: boolean) => void;
  const onRemoveContact = vi.fn(() => new Promise<boolean>((resolve) => { finish = resolve; }));
  const { rerender } = render(view({ onRemoveContact }));
  await userEvent.click(screen.getByRole("button", { name: "agent_options.contact.remove_friend" }));
  await userEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "agent_options.contact.remove_friend" }));
  rerender(view({ agent: { ...base.agent, agent_id: "other" }, onRemoveContact, state: { ...base.state, selectedContactId: "Beta" } }));
  expect(screen.queryByRole("dialog")).toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "agent_options.contact.remove_friend" }));
  const current = screen.getByRole("dialog");
  await act(async () => finish(true));
  expect(screen.getByRole("dialog")).toBe(current);
  expect(within(current).getByText("agent_options.contact.remove_friend_confirm Beta")).toBeTruthy();
});


it("projects the shared reading state into the standard return-to-latest action", async () => {
  const { rerender } = render(view());
  expect(screen.queryByRole("button", { name: "room.scroll_to_latest" })).toBeNull();
  followState.visible = true;
  rerender(view());
  await userEvent.click(screen.getByRole("button", { name: "room.scroll_to_latest" }));
  expect(followState.scrollToBottom).toHaveBeenCalledExactlyOnceWith();
  followState.visible = false;
  rerender(view());
  expect(screen.queryByRole("button", { name: "room.scroll_to_latest" })).toBeNull();
});
