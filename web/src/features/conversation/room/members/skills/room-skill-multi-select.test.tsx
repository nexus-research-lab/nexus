// INPUT: 已选 Room Skill、打开菜单命令和禁用态。
// OUTPUT: 证明菜单触发器与可移除 Chip 是合法兄弟控件，且禁用态不可移除。
// POS: Room Skill 多选 DOM 行为测试；筛选模型和目录请求另行覆盖。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { RoomSkillMultiSelect } from "./room-skill-multi-select";

const options = [
  { description: "搜索资料", label: "研究", value: "research" },
  { description: "编写内容", label: "写作", value: "writing" },
];

function renderSelect({ disabled = false }: { disabled?: boolean } = {}) {
  const onChange = vi.fn();
  render(
    <I18N_CONTEXT.Provider
      value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
    >
      <RoomSkillMultiSelect
        ariaLabel="Room 技能"
        disabled={disabled}
        emptyText="没有结果"
        errorText={null}
        isLoading={false}
        loadingText="加载中"
        onChange={onChange}
        onQueryChange={vi.fn()}
        options={options}
        placeholder="未选择"
        query=""
        searchPlaceholder="搜索技能"
        value={["research", "writing"]}
      />
    </I18N_CONTEXT.Provider>,
  );
  return onChange;
}

describe("RoomSkillMultiSelect", () => {
  it("keeps menu and removal as sibling native buttons", async () => {
    const user = userEvent.setup();
    const onChange = renderSelect();
    const trigger = screen.getByRole("button", { name: "Room 技能" });
    const removeResearch = screen.getByRole("button", { name: "移除 研究" });

    expect(trigger.contains(removeResearch)).toBe(false);
    await user.click(removeResearch);
    expect(onChange).toHaveBeenCalledWith(["writing"]);
    await user.click(trigger);
    expect(screen.getByRole("listbox", { name: "Room 技能" })).toBeTruthy();
  });

  it("blocks both opening and removal when the field is disabled", async () => {
    const user = userEvent.setup();
    const onChange = renderSelect({ disabled: true });
    const trigger = screen.getByRole("button", { name: "Room 技能" }) as HTMLButtonElement;
    const removeResearch = screen.getByRole("button", {
      name: "移除 研究",
    }) as HTMLButtonElement;

    expect(trigger.disabled).toBe(true);
    expect(removeResearch.disabled).toBe(true);
    await user.click(removeResearch);
    expect(onChange).not.toHaveBeenCalled();
    expect(screen.queryByRole("listbox")).toBeNull();
  });
});
