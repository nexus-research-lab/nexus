// INPUT: 在线 Room 名称与创建对话框提交动作。
// OUTPUT: 表单只在有效名称下提交一次原始输入。
// POS: 在线建群入口的最小组件回归。

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { CreateOnlineRoomDialog } from "./create-online-room-dialog";

describe("CreateOnlineRoomDialog", () => {
  it("submits the entered online Room name", () => {
    const onConfirm = vi.fn();
    render(
      <I18N_CONTEXT.Provider value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}>
        <CreateOnlineRoomDialog
          error={false}
          isCreating={false}
          isOpen
          onCancel={vi.fn()}
          onConfirm={onConfirm}
        />
      </I18N_CONTEXT.Provider>,
    );
    fireEvent.change(screen.getByRole("textbox", { name: /^team\.room_name/ }), {
      target: { value: "  研发群  " },
    });
    fireEvent.click(screen.getByRole("button", { name: "team.create_room" }));
    expect(onConfirm).toHaveBeenCalledExactlyOnceWith("  研发群  ");
  });
});

it("does not dismiss a pending creation through Escape or a close button", () => {
  const onCancel = vi.fn();
  render(<I18N_CONTEXT.Provider value={{locale: "zh", setLocale: vi.fn(), t: key => key}}><CreateOnlineRoomDialog error={false} isCreating isOpen onCancel={onCancel} onConfirm={vi.fn()} /></I18N_CONTEXT.Provider>);
  fireEvent.keyDown(document, {key: "Escape"});
  expect(onCancel).not.toHaveBeenCalled();
  expect(screen.getByRole("button", {name: "common.cancel"}).hasAttribute("disabled")).toBe(true);
  expect(screen.getByRole("dialog").getAttribute("aria-labelledby")).toBeTruthy();
});
