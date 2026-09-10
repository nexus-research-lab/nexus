// INPUT: Real login page/controller, locale changes and deferred authentication.
// OUTPUT: Loading visibility, translated introduction, scoped fields and synchronous submit exclusion.
// POS: Login composition regression; no requests reach a real account.
import { act, fireEvent, render, renderHook, screen } from "@testing-library/react";
import type { FormEvent, ReactNode } from "react";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { enNavigationMessages } from "@/shared/i18n/catalog/en/navigation";
import { zhNavigationMessages } from "@/shared/i18n/catalog/zh/navigation";
import { LoginPage } from "./login-page";
import { useLoginPageController } from "./use-login-page-controller";

const auth = vi.hoisted(() => ({
  error: null, isBootstrapped: false, loading: false, login: vi.fn(), refreshStatus: vi.fn(),
  status: { authenticated: false, auth_required: true, password_login_enabled: true },
}));
vi.mock("@/shared/auth/auth-context", () => ({ useAuth: () => auth }));

function Wrapper({ children, locale = "zh" }: { children: ReactNode; locale?: "zh" | "en" }) {
  const catalog = locale === "zh" ? zhNavigationMessages : enNavigationMessages;
  return <MemoryRouter initialEntries={["/login"]}><I18N_CONTEXT.Provider value={{
    locale, setLocale: vi.fn(), t: (key) => catalog[key as keyof typeof catalog] ?? key,
  }}>{children}</I18N_CONTEXT.Provider></MemoryRouter>;
}

beforeEach(() => { auth.isBootstrapped = false; auth.login.mockReset(); });

describe("Login page", () => {
  it("replaces bootstrap loading with the current-language introduction and keeps credentials across locale changes", () => {
    const { rerender } = render(<Wrapper><LoginPage /></Wrapper>);
    expect(screen.getByRole("status").getAttribute("aria-busy")).toBe("true");
    expect(screen.queryByLabelText(zhNavigationMessages["login.password"])).toBeNull();
    auth.isBootstrapped = true;
    rerender(<Wrapper><LoginPage /></Wrapper>);
    expect(screen.getByRole("heading", { name: zhNavigationMessages["login.intro_headline"] })).toBeTruthy();
    const username = screen.getByLabelText(zhNavigationMessages["login.username"]) as HTMLInputElement;
    const password = screen.getByLabelText(zhNavigationMessages["login.password"]) as HTMLInputElement;
    fireEvent.change(username, { target: { value: "owner" } });
    fireEvent.change(password, { target: { value: "draft-secret" } });
    rerender(<Wrapper locale="en"><LoginPage /></Wrapper>);
    expect(screen.getByRole("heading", { name: enNavigationMessages["login.intro_headline"] })).toBeTruthy();
    expect(screen.queryByText(zhNavigationMessages["login.intro_headline"])).toBeNull();
    expect(username.value).toBe("owner");
    expect(password.value).toBe("draft-secret");
  });

  it("coalesces two synchronous submissions and releases the lock after rejection", async () => {
    auth.isBootstrapped = true;
    let reject!: (error: Error) => void;
    auth.login.mockImplementation(() => new Promise((_resolve, rejectRequest) => { reject = rejectRequest; }));
    const { result } = renderHook(() => useLoginPageController(), { wrapper: Wrapper });
    act(() => { result.current.setUsername("owner"); result.current.setPassword("secret"); });
    const event = { preventDefault: vi.fn() } as unknown as FormEvent<HTMLFormElement>;
    let first!: Promise<void>;
    act(() => { first = result.current.submit(event); void result.current.submit(event); });
    expect(auth.login).toHaveBeenCalledExactlyOnceWith("owner", "secret");
    expect(result.current.isSubmitting).toBe(true);
    await act(async () => { reject(new Error("offline")); await first; });
    expect(result.current.isSubmitting).toBe(false);
    expect(result.current.submitFailure?.blocksSubmit).toBe(true);
  });
});
