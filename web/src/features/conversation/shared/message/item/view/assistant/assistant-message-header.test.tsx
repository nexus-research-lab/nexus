// INPUT: Assistant avatar URL failing to load.
// OUTPUT: Existing Bot fallback remains visible and contact action retains its identity.
// POS: Header and shared avatar integration regression.
import { fireEvent, render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { AssistantMessageHeader } from "./assistant-message-header";
import { MessageUserSection } from "../user/message-user-section";

it.each([false, true])("shares author avatar geometry with human messages (compact=%s)", (compact) => {
  const { container } = render(<I18nProvider>
    <AssistantMessageHeader name="Agent" compact={compact} canStop={false} echo={false} onStop={vi.fn()} showMetadata={false} />
    <MessageUserSection compact={compact} alignment="left" author={{name: "User"}} message={{message_id: "one", session_key: "session", agent_id: "agent", round_id: "round", role: "user", timestamp: 1, content: "Hello"}} />
  </I18nProvider>);
  const avatars = container.querySelectorAll(".nexus-chat-avatar");
  expect(avatars).toHaveLength(2);
  expect(avatars[0].className).toBe(avatars[1].className);
  expect(avatars[1].textContent).toBe("U");
});
it("keeps an avatar fallback after a failed image", () => {
  const open = vi.fn();
  const { container } = render(<I18nProvider><AssistantMessageHeader name="Nova" avatarUrl="https://example.com/missing.png" onOpenContact={open} canStop={false} compact={false} echo={false} onStop={vi.fn()} showMetadata={false} /></I18nProvider>);
  fireEvent.error(container.querySelector("img")!);
  const avatar = screen.getByRole("button", { name: /Nova/ });
  expect(avatar.querySelector("svg")).toBeTruthy();
  fireEvent.click(avatar);
  expect(open).toHaveBeenCalledOnce();
});
