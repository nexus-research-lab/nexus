// INPUT: Room Skill 资源读取失败和已有表单草稿。
// OUTPUT: 证明表单保留选择器，并把失败映射到共享行内提示。
// POS: Room Skill 表单反馈合同；资源竞态由 use-room-skill-options 测试负责。

import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { RoomSkillsSelector } from "./room-skills-selector";

describe("RoomSkillsSelector", () => {
  it("maps a catalog read failure to the shared inline notice", () => {
    render(
      <I18N_CONTEXT.Provider
        value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
      >
        <RoomSkillsSelector
          disabled={false}
          error="network"
          isLoading={false}
          onChange={vi.fn()}
          onQueryChange={vi.fn()}
          options={[]}
          query=""
          value={["existing-skill"]}
        />
      </I18N_CONTEXT.Provider>,
    );

    const notice = screen.getByRole("status");
    expect(notice.getAttribute("data-inline-notice-tone")).toBe("danger");
    expect(notice.textContent).toContain("room.skills_load_error");
    expect(notice.textContent).toContain("room.skills_load_error_impact");
    const trigger = screen.getByRole("button", {
      name: "room.skills_label",
    }) as HTMLButtonElement;
    expect(trigger.disabled).toBe(false);
  });
});
