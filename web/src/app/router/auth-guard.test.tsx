// INPUT: Authentication snapshots and route destinations.
// OUTPUT: Safe localized recovery, no account rendering before auth, preserved redirect.
// POS: Router guard contract; authentication requests are mocked.
import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { expect, it, vi } from "vitest";
import { AUTH_CONTEXT, type AuthContextValue } from "@/shared/auth/auth-context";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { AuthGuard } from "./auth-guard";
function Destination() { const location = useLocation(); return <output>{location.pathname}{location.search}</output>; }
function view(overrides: Partial<AuthContextValue> = {}) {
  const auth: AuthContextValue = { status: null, loading: false, isBootstrapped: true, error: "private server diagnostic", refreshStatus: vi.fn(), login: vi.fn(), logout: vi.fn(), ...overrides };
  return <I18N_CONTEXT.Provider value={{locale: "en", setLocale: vi.fn(), t: key => key}}><AUTH_CONTEXT.Provider value={auth}>
    <MemoryRouter initialEntries={["/protected?tab=one#end"]}><Routes>
      <Route element={<AuthGuard />}><Route path="/protected" element={<p>Account content</p>} /></Route>
      <Route path="*" element={<Destination />} />
    </Routes></MemoryRouter>
  </AUTH_CONTEXT.Provider></I18N_CONTEXT.Provider>;
}
it("hides diagnostics and freezes the read retry while refreshing", () => {
  const refreshStatus = vi.fn().mockResolvedValue({});
  const { rerender } = render(view({ refreshStatus }));
  expect(screen.queryByText("private server diagnostic")).toBeNull();
  expect(screen.queryByText("Account content")).toBeNull();
  fireEvent.click(screen.getByRole("button", {name: "state.retry"}));
  expect(refreshStatus).toHaveBeenCalledOnce();
  rerender(view({refreshStatus, loading: true}));
  expect((screen.getByRole("button", {name: "state.retry"}) as HTMLButtonElement).disabled).toBe(true);
});
it("shows named bootstrap loading", () => {
  render(view({isBootstrapped: false}));
  expect(screen.getByRole("status").textContent).toContain("auth_guard.connecting");
  expect(screen.queryByText("Account content")).toBeNull();
});
it.each([
  [{auth_required: true, authenticated: false, password_login_enabled: true}, "/login?redirect=%2Fprotected%3Ftab%3Done%23end"],
  [{auth_required: true, authenticated: false, password_login_enabled: true, setup_required: true}, "/setup"],
  [{auth_required: true, authenticated: true, password_login_enabled: true}, "Account content"],
] as const)("preserves guard routing for %j", (status, expected) => {
  render(view({status: {...status, username: ""}}));
  expect(screen.getByText(expected)).toBeTruthy();
});
