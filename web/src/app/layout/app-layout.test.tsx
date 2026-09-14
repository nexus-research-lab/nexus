// INPUT: 窄/宽视口以及目录/详情路由切换。
// OUTPUT: 验证隐藏目录时仍保留通知订阅，页头动作目标随布局释放。
// POS: App 壳装配回归，不请求领域数据。
import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { expect, it, vi } from "vitest";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { usePageHeaderActionsTarget } from "@/shared/lib/react/page-header-actions-context";
import { AppLayout } from "./app-layout";

const state = vi.hoisted(() => ({ narrow: true, notifications: vi.fn() }));
vi.mock("@/shared/lib/react/use-media-query", () => ({ useMediaQuery: () => state.narrow }));
vi.mock("@/features/home/notifications/use-chat-completion-notifications", () => ({ useChatCompletionNotifications: state.notifications }));
vi.mock("@/features/navigation/sidebar/sidebar-wide-panel", () => ({ SidebarWidePanel: () => <aside aria-label="目录" /> }));
function Content() {
  const target = usePageHeaderActionsTarget();
  return <section aria-label="正文">{target ? "有动作目标" : "无动作目标"}</section>;
}
function Page({ path }: { path: string }) {
  return <I18nProvider><MemoryRouter initialEntries={[path]}><Routes>
    <Route element={<AppLayout />}><Route path="*" element={<Content />} /></Route>
  </Routes></MemoryRouter></I18nProvider>;
}

it("unmounts the mobile action target when returning to a wide layout", () => {
  state.narrow = true;
  const view = render(<Page path="/settings" />);
  expect(screen.queryByRole("complementary", { name: "目录" })).toBeNull();
  expect(screen.getByText("有动作目标")).toBeTruthy();
  expect(state.notifications).toHaveBeenCalled();
  state.narrow = false;
  view.rerender(<Page path="/settings" />);
  expect(screen.getByRole("complementary", { name: "目录" })).toBeTruthy();
  expect(screen.getByText("无动作目标")).toBeTruthy();
});

it("keeps narrow directories from mounting a hidden detail outlet", () => {
  state.narrow = true;
  render(<Page path="/contacts" />);
  expect(screen.getByRole("complementary", { name: "目录" })).toBeTruthy();
  expect(screen.queryByRole("region", { name: "正文" })).toBeNull();
});
