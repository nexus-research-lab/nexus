// INPUT: Room 历史条目的读取/编辑展示模型与动作回调。
// OUTPUT: 证明条目编辑与行内动作直接复用共享 Form/List 控件并保持命令边界。
// POS: RoomHistoryItemView DOM 行为测试；权限和条目状态投影由 model 测试负责。

import { createRef } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { RoomHistoryItemView } from "./room-history-item-view";
import type { RoomHistoryItemPresentation } from "./room-history-item-model";

const READING_PRESENTATION: RoomHistoryItemPresentation = {
  actionLabels: { delete: "删除会话", rename: "重命名会话" },
  actions: ["rename", "delete"],
  actionsPersistent: true,
  activityLabel: "刚刚",
  editorLabels: {
    cancel: "取消",
    confirm: "确认重命名",
    input: "编辑会话标题",
  },
  externalSessionLabel: null,
  mode: "reading",
  selection: null,
  state: "active",
  title: "项目讨论",
};

describe("RoomHistoryItemView", () => {
  it("uses shared inline actions and the shared compact input", async () => {
    const user = userEvent.setup();
    const editor = {
      cancel: vi.fn(),
      confirm: vi.fn(),
      draft: "项目讨论",
      inputRef: createRef<HTMLInputElement>(),
      setDraft: vi.fn(),
      start: vi.fn(),
    };
    const onDelete = vi.fn();
    const { rerender } = render(
      <RoomHistoryItemView
        editor={editor}
        onDelete={onDelete}
        onSelect={vi.fn()}
        onToggleSelection={vi.fn()}
        presentation={READING_PRESENTATION}
        selectionLabel="选择项目讨论"
      />,
    );

    const rename = screen.getByRole("button", { name: "重命名会话" });
    const remove = screen.getByRole("button", { name: "删除会话" });
    expect(rename.className).toContain("h-6 w-6");
    expect(remove.className).toContain("hover:text-(--destructive)");
    await user.click(rename);
    await user.click(remove);
    expect(editor.start).toHaveBeenCalledOnce();
    expect(onDelete).toHaveBeenCalledOnce();

    rerender(
      <RoomHistoryItemView
        editor={editor}
        onDelete={onDelete}
        onSelect={vi.fn()}
        onToggleSelection={vi.fn()}
        presentation={{
          ...READING_PRESENTATION,
          actions: [],
          mode: "editing",
        }}
        selectionLabel="选择项目讨论"
      />,
    );

    const input = screen.getByRole("textbox", { name: "编辑会话标题" });
    expect(input.className).toContain("input-shell");
    expect(input.className).toContain("h-7");
    const confirm = screen.getByRole("button", { name: "确认重命名" });
    expect(confirm.className).toContain("text-(--brand-action)");
    await user.click(confirm);
    await user.click(screen.getByRole("button", { name: "取消" }));
    expect(editor.confirm).toHaveBeenCalledOnce();
    expect(editor.cancel).toHaveBeenCalledOnce();
  });
});
