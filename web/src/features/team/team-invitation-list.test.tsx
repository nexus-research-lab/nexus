// INPUT: 空邀请目录、读取失败和群主接管待办。
// OUTPUT: 空目录不占位，失败可重试，接管入口不丢失。
// POS: 邀请侧栏可见性回归，不调用远程接口。
import { fireEvent, render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { TeamInvitationList } from "./team-invitation-list";

it("hides the empty section while preserving recovery and failed-read retry", () => {
  const onRefresh = vi.fn();
  const view = (failed = false, recoveryRooms = [] as {id: string; name: string; membership_version: number}[]) => (
    <I18N_CONTEXT.Provider value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}>
      <TeamInvitationList busyRoomId={null} errorRoomId={null} invitations={[]} failed={failed}
        loading={false} onRefresh={onRefresh} onResolve={vi.fn()} recoveryRooms={recoveryRooms} onRecover={vi.fn()} />
    </I18N_CONTEXT.Provider>
  );
  const result = render(view());
  expect(screen.queryByRole("region")).toBeNull();
  result.rerender(view(true));
  fireEvent.click(screen.getByRole("button", { name: "state.retry" }));
  expect(onRefresh).toHaveBeenCalledOnce();
  result.rerender(view(false, [{ id: "room", name: "待接管群聊", membership_version: 1 }]));
  expect(screen.getByRole("button", { name: "team.recover_owner" })).toBeTruthy();
  expect(screen.queryByRole("button", { name: "state.retry" })).toBeNull();
});
