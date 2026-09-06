// INPUT: 来源编辑器的认证选择、Token 草稿、保存命令与等待态。
// OUTPUT: 证明嵌套弹窗名称隔离，公共互斥选择保留认证分支、已存凭证留空语义和精确保存载荷。
// POS: 来源管理的真实弹窗交互测试；不模拟远端来源验证或持久化。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import type { ExternalSkillSourceInfo } from "@/types/capability/skill";

import { SkillSourceManagerDialog } from "./skill-source-manager-dialog";

const SOURCE: ExternalSkillSourceInfo = {
  auth_type: "bearer",
  credential_configured: true,
  deletable: true,
  enabled: true,
  kind: "private_registry",
  managed_by: "user",
  name: "Team skills",
  sort_order: 0,
  source_id: "team-source",
  trust: "private",
  url: "https://skills.example.com/index.json",
};

function view(onSave: ReturnType<typeof vi.fn>, sources: ExternalSkillSourceInfo[] = [], loading = false) {
  return (
    <I18N_CONTEXT.Provider value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}>
      <SkillSourceManagerDialog isOpen loading={loading} onClose={vi.fn()} onDelete={vi.fn()}
        onSave={onSave} onToggle={vi.fn()} sources={sources} />
    </I18N_CONTEXT.Provider>
  );
}

describe("Skill source authentication", () => {
  it("switches through shared pressed options and preserves the draft and exact save command", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn().mockResolvedValue(false);
    render(view(onSave));
    const manager = screen.getByRole("dialog", { name: "capability.skill_sources_title" });
    await user.click(screen.getByRole("button", { name: "capability.skill_source_add" }));
    const editor = screen.getByRole("dialog", { name: "capability.skill_source_add_title" });
    expect(editor.getAttribute("aria-labelledby")).not.toBe(manager.getAttribute("aria-labelledby"));
    await user.type(screen.getByRole("textbox", { name: "capability.skill_source_name" }), "Team skills");
    await user.type(screen.getByRole("textbox", { name: "capability.skill_source_url" }), SOURCE.url);
    const none = screen.getByRole("button", { name: "capability.skill_source_auth_none" });
    const bearer = screen.getByRole("button", { name: "capability.skill_source_auth_bearer" });
    expect(screen.getAllByRole("group", { name: "capability.skill_source_auth_type" })).toHaveLength(1);
    expect(none.closest("label")).toBeNull();
    expect(none.getAttribute("aria-pressed")).toBe("true");
    await user.click(bearer);
    expect(bearer.getAttribute("aria-pressed")).toBe("true");
    expect(none.getAttribute("aria-pressed")).toBe("false");
    await user.type(screen.getByLabelText(/^capability.skill_source_token/), "example-token");
    await user.click(none);
    expect(screen.queryByLabelText(/^capability.skill_source_token/)).toBeNull();
    await user.click(bearer);
    expect((screen.getByLabelText(/^capability.skill_source_token/) as HTMLInputElement).value).toBe("example-token");
    expect(onSave).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "capability.skill_source_validate_and_add" }));
    expect(onSave).toHaveBeenCalledExactlyOnceWith(null, {
      authType: "bearer", name: "Team skills", token: "example-token", url: SOURCE.url,
    });
  });

  it("keeps stored credentials blank and disables selection while saving", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn().mockResolvedValue(false);
    const { rerender } = render(view(onSave, [SOURCE]));
    await user.click(screen.getByRole("button", { name: "capability.skill_source_edit" }));
    expect(screen.getByRole("dialog", { name: "capability.skill_source_edit_title" })).toBeTruthy();
    const token = screen.getByLabelText(/^capability.skill_source_token/) as HTMLInputElement;
    expect(token.value).toBe("");
    expect(token.required).toBe(false);
    expect(document.getElementById(token.getAttribute("aria-describedby")!)?.textContent)
      .toBe("capability.skill_source_token_keep");
    await user.click(screen.getByRole("button", { name: "capability.skill_source_validate_and_save" }));
    expect(onSave).toHaveBeenCalledExactlyOnceWith(SOURCE, {
      authType: "bearer", name: SOURCE.name, token: "", url: SOURCE.url,
    });
    rerender(view(onSave, [SOURCE], true));
    const none = screen.getByRole("button", { name: "capability.skill_source_auth_none" });
    expect((none as HTMLButtonElement).disabled).toBe(true);
    await user.click(none);
    expect(screen.getByRole("button", { name: "capability.skill_source_auth_bearer" }).getAttribute("aria-pressed")).toBe("true");
    expect(onSave).toHaveBeenCalledTimes(1);
  });
});
