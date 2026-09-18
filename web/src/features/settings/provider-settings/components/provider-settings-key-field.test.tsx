// INPUT: Masked credentials, replacement drafts and explicit user actions.
// OUTPUT: Save/cancel/confirmed-clear boundaries and identity/permission isolation.
// POS: Credential editor regression; no real keys or Provider requests.
import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState, type ComponentProps } from "react";
import { describe, expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import type { ProviderConfigRecord } from "@/types/capability/provider";
import { ProviderSettingsKeyField } from "./provider-settings-key-field";

type Props = ComponentProps<typeof ProviderSettingsKeyField>;
const record = { id: "p1", configuration_version: 1, auth_token_masked: "test-****" } as ProviderConfigRecord;
function props(overrides: Partial<Props> = {}): Props {
  return { record, value: "", disabled: false, pending: false, isEditing: true,
    providerTitle: "Example", onChange: vi.fn(), onSave: vi.fn(), onClear: vi.fn(), ...overrides };
}
function Harness(input: Props) {
  const [value, setValue] = useState(input.value);
  return <I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => key }}>
    <ProviderSettingsKeyField {...input} value={value} onChange={(next) => {
      setValue(next); input.onChange(next);
    }} />
  </I18N_CONTEXT.Provider>;
}
const keyInput = () => screen.getByLabelText("settings.providers.api_key") as HTMLInputElement;

describe("Provider key editor", () => {
  it("saves replacement explicitly, while cancel clears only the draft", async () => {
    const user = userEvent.setup();
    const input = props();
    const { rerender } = render(<Harness {...input} />);
    expect(keyInput().readOnly).toBe(true);
    expect(keyInput().value).toBe("");
    await user.click(screen.getByRole("button", { name: "settings.providers.replace_key" }));
    expect(document.activeElement).toBe(keyInput());
    expect(keyInput().readOnly).toBe(false);
    expect((screen.getByRole("button", { name: "common.save" }) as HTMLButtonElement).disabled).toBe(true);
    await user.type(keyInput(), "replacement-test-key");
    fireEvent.keyDown(keyInput(), { key: "Enter", isComposing: true });
    expect(input.onSave).not.toHaveBeenCalled();
    fireEvent.blur(keyInput());
    expect(input.onSave).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "common.cancel" }));
    expect(input.onChange).toHaveBeenLastCalledWith("");
    expect(input.onClear).not.toHaveBeenCalled();
    expect(keyInput().readOnly).toBe(true);
    await user.click(screen.getByRole("button", { name: "settings.providers.replace_key" }));
    await user.type(keyInput(), "replacement-test-key");
    await user.click(screen.getByRole("button", { name: "common.save" }));
    expect(input.onSave).toHaveBeenCalledOnce();
    rerender(<Harness {...input} record={{ ...record, configuration_version: 2 }} />);
    expect(keyInput().readOnly).toBe(true);
    expect(keyInput().value).toBe("");
  });

  it("requires confirmation and discards an open confirmation when the Provider changes", async () => {
    const user = userEvent.setup();
    const input = props();
    const { rerender } = render(<Harness {...input} />);
    await user.click(screen.getByRole("button", { name: "settings.providers.clear_key" }));
    expect(input.onClear).not.toHaveBeenCalled();
    await user.click(within(screen.getByRole("dialog")).getByRole("button", { name: "common.cancel" }));
    expect(input.onClear).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "settings.providers.clear_key" }));
    rerender(<Harness {...input} record={{ ...record, id: "p2" }} />);
    expect(screen.queryByRole("dialog")).toBeNull();
    await user.click(screen.getByRole("button", { name: "settings.providers.clear_key" }));
    await user.click(within(screen.getByRole("dialog")).getByRole("button", { name: "settings.providers.clear_key" }));
    expect(input.onClear).toHaveBeenCalledOnce();
  });

  it.each(["disabled", "pending"] as const)("blocks editing during %s", async (flag) => {
    const user = userEvent.setup();
    const input = props({ [flag]: true });
    render(<Harness {...input} />);
    expect(keyInput().disabled).toBe(true);
    for (const button of screen.getAllByRole("button")) {
      expect((button as HTMLButtonElement).disabled).toBe(true);
      await user.click(button);
    }
    expect(input.onChange).not.toHaveBeenCalled();
    expect(input.onClear).not.toHaveBeenCalled();
  });
});
