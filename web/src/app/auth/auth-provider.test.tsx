// INPUT: Authoritative auth reads and a deferred owner configuration refresh.
// OUTPUT: Previous owner content is withdrawn before new configuration is ready.
// POS: Auth Provider publication boundary regression.
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { useAuth } from "@/shared/auth/auth-context";
import { AuthProvider } from "./auth-provider";
const api = vi.hoisted(() => ({ status: vi.fn(), hydrate: vi.fn(), apply: vi.fn() }));
vi.mock("@/lib/api/account/auth-api", () => ({ getAuthStatus: api.status, loginApi: vi.fn(), logoutApi: vi.fn() }));
vi.mock("@/app/runtime-options-resource", () => ({ hydrateRuntimeOptions: api.hydrate }));
vi.mock("./auth-owner-scope", () => ({ applyAuthOwnerScope: api.apply, invalidateLocalAuthOwnerScope: vi.fn(), isAuthOwnerScopeStorageEvent: () => false }));
vi.mock("@/store/room-navigation", () => ({ synchronizeRoomNavigationStorage: vi.fn() }));
function Consumer() {
  const auth = useAuth();
  return <><output>{auth.isBootstrapped ? auth.status?.username ?? "empty" : "loading"}</output><button onClick={() => { void auth.refreshStatus().catch(() => {}); }}>Refresh</button></>;
}
const status = (username: string) => ({ auth_required: true, authenticated: true, username });
beforeEach(() => { api.status.mockReset(); api.hydrate.mockReset(); api.apply.mockReset(); api.apply.mockReturnValue(false); });
it("withdraws the previous owner while the new owner's runtime configuration loads", async () => {
  api.status.mockResolvedValueOnce(status("first"));
  render(<AuthProvider><Consumer /></AuthProvider>);
  await screen.findByText("first");
  let complete!: () => void;
  api.hydrate.mockImplementationOnce(() => new Promise<void>((resolve) => { complete = resolve; }));
  api.apply.mockReturnValueOnce(true);
  api.status.mockResolvedValueOnce(status("second"));
  fireEvent.click(screen.getByRole("button"));
  await screen.findByText("loading");
  expect(screen.queryByText("first")).toBeNull();
  expect(screen.queryByText("second")).toBeNull();
  await act(async () => { complete(); });
  await screen.findByText("second");
});
it("preserves the current authenticated snapshot when a same-owner status refresh fails", async () => {
  api.status.mockResolvedValueOnce(status("current"));
  render(<AuthProvider><Consumer /></AuthProvider>);
  await screen.findByText("current");
  api.status.mockRejectedValueOnce(new Error("offline"));
  fireEvent.click(screen.getByRole("button"));
  await waitFor(() => expect(api.status).toHaveBeenCalledTimes(2));
  expect(screen.getByText("current")).toBeTruthy();
  expect(api.hydrate).not.toHaveBeenCalled();
});
