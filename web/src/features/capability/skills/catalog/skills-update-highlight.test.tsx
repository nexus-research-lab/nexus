// INPUT: 可更新 Skill、更新检查状态与目录动作回调。
// OUTPUT: 证明更新摘要复用 Panel/ListRow/Typography，且行级与嵌套更新动作保持隔离。
// POS: Skill 目录更新摘要 DOM 合同；状态文案计算由 skills-catalog-model 测试负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import type { SkillInfo } from "@/types/capability/skill";

import { SkillsUpdateHighlight } from "./skills-update-highlight";

const UPDATE = {
  category_key: "productivity",
  category_name: "Productivity",
  deletable: true,
  description: "Keep the project documentation current.",
  enabled_for_agent: true,
  has_update: true,
  locked: false,
  name: "docs-maintainer",
  scope: "any",
  source_name: "Nexus",
  source_ref: "builtin/docs-maintainer",
  source_type: "builtin",
  tags: [],
  title: "Docs Maintainer",
  version: "1.2.0",
} satisfies SkillInfo;

describe("SkillsUpdateHighlight", () => {
  it("keeps the shared update row keyboard action separate from its update button", async () => {
    const user = userEvent.setup();
    const onCheckUpdates = vi.fn();
    const onOpenSkill = vi.fn();
    const onUpdateSkill = vi.fn();
    const { container } = render(
      <I18N_CONTEXT.Provider
        value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
      >
        <SkillsUpdateHighlight
          busySkillNames={new Set()}
          checkUpdateNotice={null}
          checkingUpdates={false}
          lastUpdateCheckedAt={null}
          onCheckUpdates={onCheckUpdates}
          onOpenSkill={onOpenSkill}
          onUpdateSkill={onUpdateSkill}
          updates={[UPDATE]}
        />
      </I18N_CONTEXT.Provider>,
    );

    expect(container.querySelector(".surface-radius-md")).toBeTruthy();
    expect(screen.getByRole("heading", {
      name: "capability.skills_updates_title",
    }).className).toContain("ui-type-section-title");
    expect(screen.getByText(UPDATE.description).className)
      .toContain("ui-type-metadata");

    await user.click(screen.getByRole("button", {
      name: "capability.skills_update",
    }));
    expect(onUpdateSkill).toHaveBeenCalledWith(UPDATE.name);
    expect(onOpenSkill).not.toHaveBeenCalled();

    const row = screen.getByRole("button", { name: UPDATE.title });
    row.focus();
    await user.keyboard("{Enter}");
    expect(onOpenSkill).toHaveBeenCalledWith(UPDATE.name);

    await user.click(screen.getByRole("button", {
      name: "capability.skills_recheck",
    }));
    expect(onCheckUpdates).toHaveBeenCalledOnce();
  });
});
