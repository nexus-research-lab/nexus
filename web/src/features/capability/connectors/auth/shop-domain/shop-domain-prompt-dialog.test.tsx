// INPUT: Real Shopify input request lifecycle, locale and user keyboard/confirmation events.
// OUTPUT: Invalid input stays editable and described; locale changes preserve drafts and exact normalized settlement.
// POS: Connector Prompt integration without starting OAuth or calling a backend.

import { act, render, renderHook, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { MESSAGES, type Locale } from "@/shared/i18n/messages";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { ShopDomainPromptDialog } from "./shop-domain-prompt-dialog";
import { useShopDomainPrompt } from "./use-shop-domain-prompt";

describe("Shopify domain prompt", () => {
  it("preserves invalid drafts across locale changes and settles a normalized value only on confirmation", async () => {
    const user = userEvent.setup();
    const settled = vi.fn();
    function Harness() {
      const controller = useShopDomainPrompt();
      return <><button onClick={() => { void controller.request().then(settled); }}>Connect store</button>
        <ShopDomainPromptDialog state={controller.state} onCancel={controller.cancel} onConfirm={controller.confirm} />
      </>;
    }
    const view = (locale: Locale) => <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t: (key) => MESSAGES[locale][key] }}><Harness /></I18N_CONTEXT.Provider>;
    const { rerender } = render(view("en"));
    await user.click(screen.getByRole("button", { name: "Connect store" }));
    const input = screen.getByRole("textbox", { name: "Store subdomain" }) as HTMLInputElement;
    await waitFor(() => expect(document.activeElement).toBe(input));
    await user.type(input, "invalid name{Enter}");
    expect(settled).not.toHaveBeenCalled();
    expect(input.getAttribute("aria-invalid")).toBe("true");
    expect(input.getAttribute("aria-errormessage")).toBe(screen.getByRole("alert").id);
    expect(screen.getByText("Enter the store subdomain before myshopify.com.")).toBeTruthy();
    rerender(view("zh"));
    expect(screen.getByRole("textbox", { name: "店铺子域名" })).toBe(input);
    expect(input.value).toBe("invalid name");
    expect(screen.getByRole("alert").textContent).toBe("请输入有效的 Shopify 店铺子域名。");
    await user.clear(input);
    await user.type(input, "https://Example-Store.myshopify.com/admin{Enter}");
    expect(settled).toHaveBeenCalledExactlyOnceWith("example-store");
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("does not overwrite an outstanding request and settles cancellation/unmount once", async () => {
    const { result, unmount } = renderHook(useShopDomainPrompt, { wrapper: I18nProvider });
    let first!: Promise<string | null>;
    act(() => { first = result.current.request(); });
    await expect(result.current.request()).rejects.toThrow();
    act(() => result.current.cancel());
    await expect(first).resolves.toBeNull();
    let second!: Promise<string | null>;
    act(() => { second = result.current.request(); });
    unmount();
    await expect(second).resolves.toBeNull();
  });
});
