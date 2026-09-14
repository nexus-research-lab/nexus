// INPUT: 真实 Custom MCP 表单、双语标签、动态行增删和脱敏配置快照。
// OUTPUT: 证明字段/错误身份不会随行号串位，保存沿用原参数、秘密 null 与传输边界。
// POS: 配置表单行为回归；只捕获 onSave 输入，不访问 MCP 或后端。

import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT, type I18nContextValue } from "@/shared/i18n/i18n-context";
import { MESSAGES, type Locale } from "@/shared/i18n/messages";
import type { CustomMCPServer, CustomMCPServerType } from "@/types/capability/connector";

import { CustomMCPDialog } from "./custom-mcp-dialog";

function localization(locale: Locale): I18nContextValue {
  return {
    locale,
    setLocale: () => undefined,
    t: (key, params = {}) => Object.entries(params).reduce(
      (text, [name, value]) => text.replaceAll(`{${name}}`, String(value)),
      MESSAGES[locale][key],
    ),
  };
}

function serverFor(type: CustomMCPServerType): CustomMCPServer {
  return {
    configuration_state: "ready",
    connector_id: "custom-mcp:example",
    enabled: true,
    name: "example",
    type,
    ...(type === "stdio"
      ? { command: "node", args: ["server.js"] }
      : { auth_type: "bearer", bearer_token: null, url: "https://example.com/mcp" }),
  };
}

function openDialog(server?: CustomMCPServer, locale: Locale = "en", busy = false) {
  const i18n = localization(locale);
  const onSave = vi.fn().mockResolvedValue(true);
  const onClose = vi.fn();
  render(<I18N_CONTEXT.Provider value={i18n}>
    <CustomMCPDialog busy={busy} onClose={onClose} onSave={onSave} server={server} />
  </I18N_CONTEXT.Provider>);
  const dialog = screen.getByRole("dialog");
  return { dialog, onClose, onSave, t: i18n.t, user: userEvent.setup() };
}

describe("CustomMCPDialog", () => {
  it.each((["en", "zh"] as const).flatMap((locale) => (
    (["stdio", "http", "sse"] as const).map((type) => ({ locale, type }))
  )))("pairs fields and exposes one choice group in $locale / $type", async ({ locale, type }) => {
    const { dialog, t, user } = openDialog(serverFor(type), locale);
    for (const label of dialog.querySelectorAll<HTMLLabelElement>("label[for]")) {
      const control = label.control;
      expect(control).not.toBeNull();
      expect(control!.closest('[role="dialog"]')).toBe(dialog);
      await user.click(label);
      expect(document.activeElement).toBe(control);
    }
    expect(within(dialog).getAllByRole("group", { name: t("capability.custom_mcp_transport") })).toHaveLength(1);
    if (type !== "stdio") {
      expect(within(dialog).getAllByRole("group", { name: t("capability.custom_mcp_auth") })).toHaveLength(1);
      const bearer = within(dialog).getByLabelText(t("capability.custom_mcp_bearer_token"), { exact: false });
      const description = document.getElementById(bearer.getAttribute("aria-describedby")!);
      expect(description?.textContent).toBe(t("capability.custom_mcp_bearer_hint"));
    }
  });

  it("keeps an argument's node, selection and value when a preceding equal-valued row is removed", async () => {
    const { dialog, onSave, t, user } = openDialog({ ...serverFor("stdio"), args: ["same", "same", "third"] });
    const rowName = (index: number) => t("capability.custom_mcp_row_label", { group: t("capability.custom_mcp_arguments"), index });
    const second = within(dialog).getByRole("textbox", { name: rowName(2) }) as HTMLInputElement;
    const id = second.id;
    act(() => { second.focus(); second.setSelectionRange(1, 3); });
    // Dispatch a different row's removal without first moving focus from the surviving input.
    fireEvent.click(within(dialog).getByRole("button", { name: t("capability.custom_mcp_remove_row", { row: rowName(1) }) }));
    expect(within(dialog).getByRole("textbox", { name: rowName(1) })).toBe(second);
    expect(second.id).toBe(id);
    expect(document.activeElement).toBe(second);
    expect([second.selectionStart, second.selectionEnd]).toEqual([1, 3]);
    await user.clear(second);
    await user.type(second, "edited");
    await user.click(within(dialog).getByRole("button", { name: t("capability.custom_mcp_add_argument") }));
    await user.type(within(dialog).getByRole("textbox", { name: rowName(3) }), "  final argument  ");

    const transport = within(dialog).getByRole("group", { name: t("capability.custom_mcp_transport") });
    await user.click(within(transport).getByRole("button", { name: "HTTP" }));
    await user.click(within(transport).getByRole("button", { name: "STDIO" }));
    await user.click(within(dialog).getByRole("button", { name: t("common.save") }));
    expect(onSave).toHaveBeenCalledOnce();
    expect(onSave).toHaveBeenCalledWith({ name: "example", type: "stdio", command: "node", args: ["edited", "third", "  final argument  "], env: {} });
  });

  it.each(["env", "headers"] as const)("isolates %s rows and preserves untouched masked values on save", async (kind) => {
    const source = { ...serverFor(kind === "env" ? "stdio" : "http"), [kind]: { ALPHA: null, BETA: null, GAMMA: null } };
    if (kind === "headers") source.auth_type = "headers";
    const { dialog, onSave, t, user } = openDialog(source);
    const group = t(kind === "env" ? "capability.custom_mcp_environment" : "capability.custom_mcp_headers");
    const rowName = (index: number) => t("capability.custom_mcp_row_label", { group, index });
    const inputName = (index: number, field: "key" | "value") => `${rowName(index)} ${t(`capability.custom_mcp_${field}`)}`;
    const second = within(dialog).getByLabelText(inputName(2, "value"), { exact: true }) as HTMLInputElement;
    const id = second.id;
    expect(second.value).toBe("");
    expect(second.required).toBe(false);
    act(() => second.focus());
    fireEvent.click(within(dialog).getByRole("button", { name: t("capability.custom_mcp_remove_row", { row: rowName(1) }) }));
    expect(within(dialog).getByLabelText(inputName(1, "value"), { exact: true })).toBe(second);
    expect(second.id).toBe(id);
    expect(document.activeElement).toBe(second);
    await user.type(second, "replacement");
    expect(second.required).toBe(true);
    await user.click(within(dialog).getByRole("button", { name: t(kind === "env" ? "capability.custom_mcp_add_environment" : "capability.custom_mcp_add_header") }));
    await user.type(within(dialog).getByLabelText(inputName(3, "key"), { exact: true }), "NEXT");
    await user.type(within(dialog).getByLabelText(inputName(3, "value"), { exact: true }), " value with spaces ");
    await user.click(within(dialog).getByRole("button", { name: t("common.save") }));
    const secrets = { BETA: "replacement", GAMMA: null, NEXT: " value with spaces " };
    expect(onSave).toHaveBeenCalledOnce();
    expect(onSave).toHaveBeenCalledWith(kind === "env"
      ? { name: "example", type: "stdio", command: "node", args: ["server.js"], env: secrets }
      : { name: "example", type: "http", url: "https://example.com/mcp", auth_type: "headers", bearer_token: undefined, headers: secrets });
  });

  it("keeps a native error on its exact secret input after deleting a preceding row", async () => {
    const { dialog, onSave, t, user } = openDialog({ ...serverFor("stdio"), env: { ALPHA: null, BETA: null } });
    const rowName = (index: number) => t("capability.custom_mcp_row_label", { group: t("capability.custom_mcp_environment"), index });
    await user.click(within(dialog).getByRole("button", { name: t("capability.custom_mcp_add_environment") }));
    await user.type(within(dialog).getByLabelText(`${rowName(3)} Key`, { exact: true }), "GAMMA");
    const invalid = within(dialog).getByLabelText(`${rowName(3)} Value`, { exact: true });
    await user.click(within(dialog).getByRole("button", { name: t("common.save") }));
    expect(onSave).not.toHaveBeenCalled();
    expect(invalid.getAttribute("aria-invalid")).toBe("true");
    const errorId = invalid.getAttribute("aria-errormessage");
    expect(document.getElementById(errorId!)).not.toBeNull();
    fireEvent.click(within(dialog).getByRole("button", { name: t("capability.custom_mcp_remove_row", { row: rowName(1) }) }));
    expect(within(dialog).getByLabelText(`${rowName(2)} Value`, { exact: true })).toBe(invalid);
    expect(invalid.getAttribute("aria-errormessage")).toBe(errorId);
    expect(invalid.getAttribute("aria-invalid")).toBe("true");
    expect(within(dialog).getByLabelText(`${rowName(1)} Value`, { exact: true }).getAttribute("aria-invalid")).toBeNull();
    await user.type(invalid, "new-value");
    expect(invalid.getAttribute("aria-invalid")).toBeNull();
    await user.click(within(dialog).getByRole("button", { name: t("common.save") }));
    expect(onSave).toHaveBeenCalledWith({ name: "example", type: "stdio", command: "node", args: ["server.js"], env: { BETA: null, GAMMA: "new-value" } });
  });

  it.each(["http", "sse"] as const)("keeps or replaces the masked bearer for %s and omits hidden auth fields", async (type) => {
    const { dialog, onSave, t, user } = openDialog(serverFor(type));
    const save = within(dialog).getByRole("button", { name: t("common.save") });
    const expected = { name: "example", type, url: "https://example.com/mcp", auth_type: "bearer", bearer_token: null, headers: undefined };
    await user.click(save);
    expect(onSave).toHaveBeenLastCalledWith(expected);
    await user.type(within(dialog).getByLabelText(t("capability.custom_mcp_bearer_token"), { exact: false }), "  new-token  ");
    await user.click(save);
    expect(onSave).toHaveBeenLastCalledWith({ ...expected, bearer_token: "new-token" });
    const auth = within(dialog).getByRole("group", { name: t("capability.custom_mcp_auth") });
    await user.click(within(auth).getByRole("button", { name: t("capability.custom_mcp_auth_none") }));
    await user.click(save);
    expect(onSave).toHaveBeenLastCalledWith({ ...expected, auth_type: "none", bearer_token: undefined });
  });

  it("binds labels and dialog titles to the right instance when two dialogs are mounted", () => {
    render(<I18N_CONTEXT.Provider value={localization("en")}>
      <CustomMCPDialog busy={false} onClose={vi.fn()} onSave={vi.fn()} server={serverFor("http")} />
      <CustomMCPDialog busy={false} onClose={vi.fn()} onSave={vi.fn()} server={serverFor("http")} />
    </I18N_CONTEXT.Provider>);
    const dialogs = document.querySelectorAll('[role="dialog"]');
    expect(dialogs).toHaveLength(2);
    const ids = [...document.querySelectorAll("[id]")].map((element) => element.id);
    expect(new Set(ids).size).toBe(ids.length);
    for (const dialog of dialogs) {
      expect(dialog.contains(document.getElementById(dialog.getAttribute("aria-labelledby")!))).toBe(true);
      for (const label of dialog.querySelectorAll<HTMLLabelElement>("label[for]")) {
        expect(label.control?.closest('[role="dialog"]')).toBe(dialog);
      }
    }
  });

  it("retains save and dismissal locks while a command is busy", async () => {
    const { dialog, onClose, onSave, t, user } = openDialog(serverFor("stdio"), "en", true);
    const save = within(dialog).getByRole("button", { name: t("common.saving") }) as HTMLButtonElement;
    const cancel = within(dialog).getByRole("button", { name: t("common.cancel") }) as HTMLButtonElement;
    expect(save.disabled).toBe(true);
    expect(cancel.disabled).toBe(true);
    await user.click(save);
    await user.click(cancel);
    await user.keyboard("{Escape}");
    expect(onSave).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
  });
});
