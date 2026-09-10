// INPUT: OAuth callback 成功回执、语言变更和受控事件。
// OUTPUT: 切换语言不重复提交 OAuth，页面不回显 code/state。
// POS: 回调页面本地化/副作用回归。
import { act, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { MemoryRouter } from "react-router-dom";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { ConnectorOAuthCallbackPage } from "./connector-oauth-callback-page";
const h = vi.hoisted(() => ({ complete: vi.fn(), publish: vi.fn() }));
vi.mock("@/lib/api/capability/connector-api", () => ({ completeConnectorOAuthApi: h.complete }));
vi.mock("@/config/desktop-runtime", () => ({ getConnectorOauthRedirectUri: () => "https://local/callback", getDesktopConnectorsReturnUri: () => "nexus://connectors", isDesktopLoopbackOauthCallback: () => false }));
vi.mock("@/lib/desktop-bridge", () => ({ isDesktopBridgeAvailable: () => false, openDesktopRoute: vi.fn() }));
vi.mock("@/features/capability/connectors/auth/connector-oauth-events", () => ({ readPendingConnectorOauth: () => "connector", clearPendingConnectorOauth: vi.fn(), publishConnectorOauthEvent: h.publish }));
afterEach(() => { vi.useRealTimers(); vi.clearAllMocks(); });
it("translates current status without consuming authorization a second time", async () => {
  vi.useFakeTimers();
  let finish!: (value: { connector_id: string }) => void;
  h.complete.mockReturnValue(new Promise((resolve) => { finish = resolve; }));
  const view = (locale: "zh" | "en") => <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t: (key) => `${locale}:${key}` }}>
    <MemoryRouter initialEntries={["/callback?code=secret-code&state=secret-state"]}><ConnectorOAuthCallbackPage /></MemoryRouter>
  </I18N_CONTEXT.Provider>;
  const { rerender, unmount } = render(view("zh"));
  expect(screen.getByRole("heading").textContent).toBe("zh:capability.oauth_callback.checking_title");
  rerender(view("en"));
  expect(screen.getByRole("heading").textContent).toBe("en:capability.oauth_callback.checking_title");
  await act(async () => finish({ connector_id: "connector" }));
  expect(screen.getByRole("heading").textContent).toBe("en:capability.oauth_callback.success_title");
  expect(h.complete).toHaveBeenCalledExactlyOnceWith("secret-code", "secret-state", "https://local/callback");
  expect(document.body.textContent).not.toContain("secret-code");
  expect(document.body.textContent).not.toContain("secret-state");
  unmount();
});
