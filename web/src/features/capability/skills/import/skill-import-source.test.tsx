// INPUT: Skill 导入模式、草稿、文件入口与模式切换动作。
// OUTPUT: 证明导入来源复用共享分段控件、Panel、Typography 和 Spinner。
// POS: Skill 导入来源 DOM 合同；实际文件读取与网络提交归 controller。

import { createRef } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { SkillImportSource } from "./skill-import-source";

function view(mode: "git" | "local", importing = false) {
  return (
    <I18N_CONTEXT.Provider
      value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
    >
      <SkillImportSource
        draft={{ branch: "", path: "", url: "" }}
        fileInputRef={createRef<HTMLInputElement>()}
        gitUrlInputRef={createRef<HTMLInputElement>()}
        importing={importing}
        mode={mode}
        onSelectMode={onSelectMode}
        setDraftField={vi.fn()}
      />
    </I18N_CONTEXT.Provider>
  );
}

const onSelectMode = vi.fn();

describe("SkillImportSource", () => {
  it("uses the shared segmented owner and dispatches mode changes", async () => {
    onSelectMode.mockClear();
    const user = userEvent.setup();
    render(view("git"));

    const modes = screen.getByRole("group", {
      name: "capability.skills_import_title",
    });
    expect(modes.className).toContain("segmented-control");
    expect(screen.getByRole("button", {
      name: "capability.skills_import_mode_git",
    }).getAttribute("aria-pressed")).toBe("true");

    await user.click(screen.getByRole("button", {
      name: "capability.skills_import_mode_local",
    }));
    expect(onSelectMode).toHaveBeenCalledWith("local");
  });

  it("renders local import through shared panel, typography, and loading recipes", () => {
    const { container } = render(view("local", true));

    expect(screen.getByText("capability.skills_import_zip_title").className)
      .toContain("ui-type-section-title");
    expect(container.querySelector("section.surface-radius-md")).toBeTruthy();
    expect(screen.getByRole("button", {
      name: /capability.skills_importing/,
    }).querySelector("svg")?.className.baseVal).toContain("motion-reduce:animate-none");
  });
});
