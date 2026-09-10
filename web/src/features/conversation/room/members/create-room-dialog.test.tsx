// INPUT: Room 管理草稿、异步保存与输入法键盘事件。
// OUTPUT: 验证同步防重、pending 草稿锁、失败保留与 IME 确认不提交。
// POS: 成员弹窗交互回归；不执行持久化或视觉验收。
import { act, fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { CreateRoomDialog } from "./create-room-dialog";

vi.mock("./skills/use-room-skill-options", () => ({
  useRoomSkillOptions: () => ({ options: [], error: null, loading: false }),
}));
vi.mock("./room-avatar-picker", () => ({ RoomAvatarPicker: () => null }));

function setup(onConfirm: () => Promise<void>) {
  const onCancel = vi.fn();
  render(<I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => key }}>
    <CreateRoomDialog agents={[{ agent_id: "a", name: "Nova" }]} initialName="Research"
      initialSelectedAgentIds={["a"]} isOpen mode="manage" onConfirm={onConfirm} onCancel={onCancel} />
  </I18N_CONTEXT.Provider>);
  return onCancel;
}

describe("CreateRoomDialog submission", () => {
  it("uses the same form to select online Room members", async () => {
    const onConfirm = vi.fn(async () => undefined);
    render(<I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key, values) => values?.name ? `${key} ${values.name}` : key }}>
      <CreateRoomDialog
        agents={[{ agent_id: "agent", name: "Nova" }]}
        initialName="Research"
        initialSelectedAgentIds={["agent"]}
        isOpen
        onlineAvailable
        onCancel={vi.fn()}
        onConfirm={onConfirm}
        users={[{ user_id: "user", username: "lee", display_name: "Lee" }]}
      />
    </I18N_CONTEXT.Provider>);

    fireEvent.click(screen.getByRole("button", { name: "room.location_online" }));
    fireEvent.click(screen.getByRole("button", { name: "room.user_select_add Lee" }));
    await act(async () => { fireEvent.click(screen.getByRole("button", { name: "room.create_action" })); });
    expect(onConfirm).toHaveBeenCalledWith(expect.objectContaining({
      agentIds: ["agent"],
      location: "online",
      userIds: ["user"],
    }));
  });

  it("freezes the draft, blocks duplicate submits, and preserves it on rejection", async () => {
    let reject!: (reason: Error) => void;
    const onConfirm = vi.fn(() => new Promise<void>((_, fail) => { reject = fail; }));
    const onCancel = setup(onConfirm);
    const save = screen.getByRole("button", { name: "common.save" });
    act(() => { save.click(); save.click(); });
    expect(onConfirm).toHaveBeenCalledTimes(1);
    expect((screen.getByRole("textbox", { name: "room.name" }) as HTMLInputElement).disabled).toBe(true);
    expect(screen.getByRole("button", { name: "room.agent_select_remove" }).getAttribute("aria-disabled")).toBe("true");
    fireEvent.click(screen.getByRole("button", { name: "common.cancel" }));
    expect(onCancel).not.toHaveBeenCalled();
    await act(async () => { reject(new Error("request failed")); });
    expect(screen.getByText("room.save_unconfirmed")).toBeTruthy();
    expect((screen.getByRole("textbox", { name: "room.name" }) as HTMLInputElement).value).toBe("Research");
    expect((screen.getByRole("button", { name: "common.save" }) as HTMLButtonElement).disabled).toBe(true);
    fireEvent.click(screen.getByRole("button", { name: "common.save" }));
    fireEvent.keyDown(screen.getByRole("textbox", { name: "room.name" }), { key: "Enter" });
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it("does not save while accepting an IME candidate", async () => {
    const onConfirm = vi.fn(async () => undefined);
    setup(onConfirm);
    const name = screen.getByRole("textbox", { name: "room.name" });
    fireEvent.keyDown(name, { key: "Enter", isComposing: true });
    fireEvent.keyDown(name, { key: "Enter", keyCode: 229 });
    expect(onConfirm).not.toHaveBeenCalled();
    await act(async () => { fireEvent.keyDown(name, { key: "Enter" }); });
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });
});
