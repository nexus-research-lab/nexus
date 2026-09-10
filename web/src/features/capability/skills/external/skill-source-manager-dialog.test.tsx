// INPUT: 来源编辑器的认证选择、Token 草稿、保存命令与等待态。
// OUTPUT: 证明弹窗/字段实例隔离、具名独立行动及认证分支、凭据留空、精确命令和 busy。
// POS: 来源管理的真实弹窗交互测试；不模拟远端来源验证或持久化。

import { render, screen, within } from "@testing-library/react";
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
  it("keeps fields and save commands isolated between two mounted editors", async () => {
    const user = userEvent.setup();
    const firstSave = vi.fn().mockResolvedValue(false);
    const secondSave = vi.fn().mockResolvedValue(false);
    render(view(firstSave, [SOURCE]));
    await user.click(screen.getByRole("button", { name: "capability.skill_source_edit" }));
    render(view(secondSave, [SOURCE]));
    await user.click(screen.getByRole("button", { name: "capability.skill_source_edit" }));
    for (const label of document.querySelectorAll<HTMLLabelElement>("form label[for]")) {
      expect(label.control).not.toBeNull();
      expect(label.control?.closest("form")).toBe(label.closest("form"));
    }
    const editors = screen.getAllByRole("dialog", { name: "capability.skill_source_edit_title" });
    expect(editors).toHaveLength(2);
    const editor = within(editors[1]);
    const name = editor.getByRole("textbox", { name: "capability.skill_source_name" });
    await user.clear(name);
    await user.type(name, "Second source");
    await user.click(editor.getByRole("button", { name: "capability.skill_source_validate_and_save" }));
    expect(secondSave).toHaveBeenCalledExactlyOnceWith(SOURCE, {
      authType: "bearer", name: "Second source", token: "", url: SOURCE.url,
    });
    expect(firstSave).not.toHaveBeenCalled();
  });

  it("names each source action group and describes its switch without row-wide commands", async () => {
    const user = userEvent.setup();
    const onToggle = vi.fn();
    const onDelete = vi.fn();
    const source = { ...SOURCE, last_error: "Private diagnostic text" };
    render(<I18N_CONTEXT.Provider value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}>
      <SkillSourceManagerDialog isOpen loading={false} onClose={vi.fn()} onDelete={onDelete}
        onSave={vi.fn()} onToggle={onToggle} sources={[source, { ...SOURCE, source_id: "built-in", name: "Built-in", deletable: false }]} />
    </I18N_CONTEXT.Provider>);
    const group = within(screen.getByRole("group", { name: SOURCE.name }));
    const toggle = group.getByRole("switch");
    const descriptions = toggle.getAttribute("aria-describedby")!.split(" ").map((id) => document.getElementById(id)?.textContent);
    expect(descriptions).toEqual([
      `capability.skill_source_private · ${SOURCE.url}`,
      "capability.skill_source_credential_configured", "capability.skills_external_source_failed_description",
    ]);
    expect(screen.queryByText("Private diagnostic text")).toBeNull();
    await user.click(screen.getByText(SOURCE.name));
    expect(onToggle).not.toHaveBeenCalled();
    await user.click(toggle);
    expect(onToggle).toHaveBeenCalledExactlyOnceWith(source, false);
    expect(within(screen.getByRole("group", { name: "Built-in" })).queryAllByRole("button")).toHaveLength(0);
    await user.click(group.getByRole("button", { name: "capability.skill_source_delete" }));
    expect(onDelete).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "common.delete" }));
    expect(onDelete).toHaveBeenCalledExactlyOnceWith(source);
  });

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
    expect(screen.getByRole("button", { name: "capability.skill_source_validate_and_save" }).getAttribute("aria-busy")).toBe("true");
    const none = screen.getByRole("button", { name: "capability.skill_source_auth_none" });
    expect((none as HTMLButtonElement).disabled).toBe(true);
    await user.click(none);
    expect(screen.getByRole("button", { name: "capability.skill_source_auth_bearer" }).getAttribute("aria-pressed")).toBe("true");
    expect(onSave).toHaveBeenCalledTimes(1);
  });
});
