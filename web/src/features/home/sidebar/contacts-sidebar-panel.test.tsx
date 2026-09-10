// INPUT: 联系人导航期间的异步结果、失败与用户新选择。
// OUTPUT: 确认重复准备受阻，迟到结果不导航，未知结果只进入目录核对。
// POS: 联系人侧栏导航边界回归，不调用真实服务。
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiRequestError } from "@/lib/api/core/http-error";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { ContactsSidebarPanelContent } from "./contacts-sidebar-panel";

const mocks = vi.hoisted(() => ({
  navigate: vi.fn(), resolve: vi.fn(), setActive: vi.fn(), clear: vi.fn(),
}));
vi.mock("react-router-dom", () => ({
  useNavigate: () => mocks.navigate,
  useLocation: () => ({ pathname: "/contacts", search: "", key: "contacts" }),
}));
vi.mock("@/features/navigation/direct-room/direct-room-navigation", () => ({
  resolveDirectRoomNavigationTarget: mocks.resolve,
}));
vi.mock("@/store/sidebar", () => ({
  useSidebarStore: (select: (value: unknown) => unknown) => select({
    set_active_panel_item: mocks.setActive,
    clear_chat_notifications_for_target: mocks.clear,
  }),
}));
vi.mock("./sidebar-directory", () => ({
  useSidebarDirectory: () => ({ agents: [{ id: "nova", name: "Nova" }], hasError: false, hasLoaded: true, isLoading: false }),
}));
vi.mock("./sidebar-list-rows", () => ({
  ContactRow: ({ onChat, onOpenDirectory }: { onChat: () => void; onOpenDirectory: () => void }) => <><button onClick={onChat}>chat</button><button onClick={onOpenDirectory}>detail</button></>,
  SidebarListLoadingRows: () => null,
}));
const target = { route: "/room/result", context: { room: { id: "room" }, conversation: { id: "conversation" } } };
function mount() {
  return render(<I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => key }}><ContactsSidebarPanelContent /></I18N_CONTEXT.Provider>);
}
beforeEach(() => vi.clearAllMocks());

describe("contacts direct chat navigation", () => {
  it("deduplicates pending preparation and discards completion after a newer selection", async () => {
    let finish!: (value: typeof target) => void;
    mocks.resolve.mockReturnValue(new Promise((resolve) => { finish = resolve; }));
    mount();
    fireEvent.click(screen.getByText("chat"));
    fireEvent.click(screen.getByText("chat"));
    expect(mocks.resolve).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByText("detail"));
    await act(async () => finish(target));
    expect(mocks.navigate).toHaveBeenCalledTimes(1);
    expect(mocks.navigate).not.toHaveBeenCalledWith(target.route);
    expect(mocks.clear).not.toHaveBeenCalled();
  });
  it("ignores completion after unmount", async () => {
    let finish!: (value: typeof target) => void;
    mocks.resolve.mockReturnValue(new Promise((resolve) => { finish = resolve; }));
    const view = mount();
    fireEvent.click(screen.getByText("chat"));
    view.unmount();
    await act(async () => finish(target));
    expect(mocks.navigate).not.toHaveBeenCalled();
    expect(mocks.clear).not.toHaveBeenCalled();
  });
  it("retries only when the server proves the command was not applied", async () => {
    mocks.resolve.mockRejectedValueOnce(new ApiRequestError("detail", 409, {
      version: 1, code: "conflict", category: "conflict", effect: "not_applied",
    })).mockResolvedValueOnce(target);
    mount();
    await act(async () => fireEvent.click(screen.getByText("chat")));
    await act(async () => fireEvent.click(screen.getByText("sidebar.retry")));
    expect(mocks.resolve).toHaveBeenCalledTimes(2);
    expect(mocks.navigate).toHaveBeenCalledWith(target.route);
    expect(mocks.clear).toHaveBeenCalledTimes(1);
  });
  it("contains an unknown failure and checks the directory without replay", async () => {
    mocks.resolve.mockRejectedValue(new Error("private backend detail"));
    mount();
    await act(async () => fireEvent.click(screen.getByText("chat")));
    expect(screen.getByText("sidebar.dm_check_impact")).toBeTruthy();
    expect(screen.queryByText("private backend detail")).toBeNull();
    fireEvent.click(screen.getByText("sidebar.dm_check_directory"));
    expect(mocks.resolve).toHaveBeenCalledTimes(1);
    expect(mocks.navigate).toHaveBeenCalledTimes(1);
  });
});
