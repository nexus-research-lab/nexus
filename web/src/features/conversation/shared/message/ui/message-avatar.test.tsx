// INPUT: Message avatar sources and optional native detail commands.
// OUTPUT: Failed images fall back, new sources retry and named actions remain accessible.
// POS: Local message avatar regression.
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { MessageAvatar } from "./message-avatar";

describe("MessageAvatar", () => {
  it("recovers failed images and retries only for a new source", () => {
    const view = (url: string) => <I18nProvider><MessageAvatar avatarUrl={url}>Fallback</MessageAvatar></I18nProvider>;
    const { container, rerender } = render(view("https://example.com/one.png"));
    fireEvent.error(container.querySelector("img")!);
    expect(screen.getByText("Fallback")).toBeTruthy();
    expect(container.querySelector("img")).toBeNull();
    rerender(view("https://example.com/two.png"));
    expect(container.querySelector("img")?.getAttribute("src")).toBe("https://example.com/two.png");
  });
  it("names the native detail action and preserves an explicit name", () => {
    const onClick = vi.fn();
    const { rerender } = render(<I18nProvider><MessageAvatar onClick={onClick}>N</MessageAvatar></I18nProvider>);
    fireEvent.click(screen.getByRole("button", { name: /View avatar details|查看头像详情/ }));
    expect(onClick).toHaveBeenCalledOnce();
    rerender(<I18nProvider><MessageAvatar onClick={onClick} ariaLabel="Nova details">N</MessageAvatar></I18nProvider>);
    expect(screen.getByRole("button", { name: "Nova details" })).toBeTruthy();
  });
});
