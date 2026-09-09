// INPUT: Assistant avatar URL failing to load.
// OUTPUT: Existing Bot fallback remains visible and contact action retains its identity.
// POS: Header and shared avatar integration regression.
import { fireEvent, render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { AssistantMessageHeader } from "./assistant-message-header";
it("keeps an avatar fallback after a failed image", () => {
  const open = vi.fn();
  const { container } = render(<I18nProvider><AssistantMessageHeader name="Nova" avatarUrl="https://example.com/missing.png" onOpenContact={open} canStop={false} compact={false} echo={false} onStop={vi.fn()} showMetadata={false} /></I18nProvider>);
  fireEvent.error(container.querySelector("img")!);
  const avatar = screen.getByRole("button", { name: /Nova/ });
  expect(avatar.querySelector("svg")).toBeTruthy();
  fireEvent.click(avatar);
  expect(open).toHaveBeenCalledOnce();
});
