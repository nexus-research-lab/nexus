// INPUT: Launcher 页面导航和延迟完成的 DM 准备请求。
// OUTPUT: 旧请求和卸载后的结果不改写当前导航。
// POS: 页面导航投影代次回归，领域请求仅为受控 Promise。
import { act, render } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { LauncherPage } from "./launcher-page";
const h = vi.hoisted(() => ({ navigate: vi.fn(), resolve: vi.fn(), setCurrent: vi.fn(), setActive: vi.fn(), select: (_id: string) => {} }));
vi.mock("react-router-dom", () => ({ useNavigate: () => h.navigate, useSearchParams: () => [new URLSearchParams()] }));
vi.mock("@/shared/i18n/i18n-context", () => ({ useI18n: () => ({ t: (key: string) => key }) }));
vi.mock("@/shared/theme/theme-context", () => ({ useTheme: () => ({ theme: "light" }) }));
vi.mock("@/features/home/home-directory-resource", () => ({ useHomeDirectory: () => ({ agents: [], conversations: [], rooms: [], hasLoaded: true, hasError: false, isLoading: false }), refreshHomeDirectory: vi.fn() }));
vi.mock("@/config/runtime-options", () => ({ getDefaultAgentId: () => "main" }));
vi.mock("@/store/agent", () => ({ useAgentStore: (select: (state: unknown) => unknown) => select({ current_agent_id: null, set_current_agent: h.setCurrent }) }));
vi.mock("@/store/sidebar", () => ({ useSidebarStore: (select: (state: unknown) => unknown) => select({ set_active_panel_item: h.setActive }) }));
vi.mock("@/features/navigation/direct-room/direct-room-navigation", () => ({ resolveDirectRoomNavigationTarget: h.resolve }));
vi.mock("@/features/launcher/console/launcher-console", () => ({ LauncherConsole: ({ onSelectAgent }: { onSelectAgent: (id: string) => void }) => { h.select = onSelectAgent; return null; } }));
beforeEach(() => vi.clearAllMocks());
function deferred() { let resolve!: (value: unknown) => void; const promise = new Promise((done) => { resolve = done; }); return { promise, resolve }; }
const result = (id: string) => ({ context: { room: { id } }, route: `/${id}` });
it("only applies the latest requested Agent navigation", async () => {
  const first = deferred(); const second = deferred();
  h.resolve.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise);
  render(<LauncherPage />);
  act(() => { h.select("first"); h.select("second"); });
  await act(async () => second.resolve(result("second")));
  await act(async () => first.resolve(result("first")));
  expect(h.navigate).toHaveBeenCalledExactlyOnceWith("/second");
  expect(h.setCurrent).toHaveBeenCalledExactlyOnceWith("second");
  expect(h.setActive).toHaveBeenCalledExactlyOnceWith("second");
});
it("does not navigate after the launcher has unmounted", async () => {
  const pending = deferred(); h.resolve.mockReturnValueOnce(pending.promise);
  const { unmount } = render(<LauncherPage />);
  act(() => h.select("first")); unmount();
  await act(async () => pending.resolve(result("first")));
  expect(h.navigate).not.toHaveBeenCalled();
  expect(h.setCurrent).not.toHaveBeenCalled();
});
