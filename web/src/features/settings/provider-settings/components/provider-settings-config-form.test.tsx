// INPUT: Provider 创建/编辑/只读模式、字段与格式草稿、固定端点目录。
// OUTPUT: 证明多实例标签、原始变更/失焦回调、管理权限与固定端点的只读语义保持正确。
// POS: Provider 配置视图回归；布局断点由浏览器在真实容器中验证。

import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentProps, ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { ProviderSettingsConfigForm } from "./provider-settings-config-form";

type Props = ComponentProps<typeof ProviderSettingsConfigForm>;
const I18N = { locale: "en" as const, setLocale: () => undefined, t: (key: string) => key };
const FORMATS = [
  { api_format: "responses" as const, base_url: "https://example.com/responses", models_path: "/models" },
  { api_format: "chat_completions" as const, base_url: "https://example.com/chat", models_path: "/models" },
];

function props(overrides: Partial<Props> = {}): Props {
  return {
    builtinEndpointFormats: FORMATS,
    currentFormat: FORMATS[0],
    currentPreset: null,
    detailTitle: "Example",
    draft: { provider_kind: "llm", provider: "example", preset_key: "custom", api_format: "responses", display_name: "Example", auth_token: "", base_url: "https://example.com", models_path: "/models", enabled: true },
    formatOptions: [{ value: "responses", label: "Responses" }, { value: "chat_completions", label: "Chat Completions" }],
    isEditing: false,
    onApiFormatChange: vi.fn(), onAuthTokenChange: vi.fn(), onBaseUrlChange: vi.fn(), onFieldBlur: vi.fn(),
    onProviderDisplayNameChange: vi.fn(), onProviderKindChange: vi.fn(),
    providerKindOptions: [{ value: "llm", label: "Language model" }, { value: "image_generation", label: "Image generation" }],
    selectedCanManage: true, selectedRecord: null,
    showProviderShapeControls: true, showRuntimeFormatBadge: false, usesBuiltinEndpoint: false,
    ...overrides,
  };
}

function view(children: ReactNode) {
  return <I18N_CONTEXT.Provider value={I18N}>{children}</I18N_CONTEXT.Provider>;
}

describe("Provider configuration fields", () => {
  it("keeps each instance's labels on its own controls", async () => {
    const user = userEvent.setup();
    render(view(<>
      <section aria-label="First"><ProviderSettingsConfigForm {...props()} /></section>
      <section aria-label="Second"><ProviderSettingsConfigForm {...props()} /></section>
    </>));
    const ids = [...document.querySelectorAll("[id]")].map((element) => element.id);
    expect(new Set(ids).size).toBe(ids.length);
    for (const section of screen.getAllByRole("region")) {
      for (const label of section.querySelectorAll<HTMLLabelElement>("label[for]")) {
        expect(label.control?.closest("section")).toBe(section);
        await user.click(label);
        if (label.control?.getAttribute("aria-haspopup") === "listbox") {
          const popupId = label.control.getAttribute("aria-controls");
          const listbox = popupId ? document.getElementById(popupId) : null;
          expect(listbox?.getAttribute("role")).toBe("listbox");
          expect(document.activeElement?.closest("[role=listbox]")).toBe(listbox);
          expect(document.activeElement?.getAttribute("aria-selected")).toBe("true");
          await user.keyboard("{Escape}");
          expect(screen.queryByRole("listbox")).toBeNull();
        }
        expect(document.activeElement).toBe(label.control);
      }
    }
  });

  it("forwards raw text and exact selection changes while leaving blur commits to the owner", async () => {
    const state = props();
    const user = userEvent.setup();
    render(view(<ProviderSettingsConfigForm {...state} />));
    for (const [key, callback] of [
      ["provider_name", state.onProviderDisplayNameChange],
      ["api_key", state.onAuthTokenChange],
      ["base_url", state.onBaseUrlChange],
    ] as const) {
      const input = screen.getByLabelText(`settings.providers.${key}`, { exact: false });
      fireEvent.change(input, { target: { value: "  raw value  " } });
      expect(callback).toHaveBeenCalledWith("  raw value  ");
      expect(state.onFieldBlur).not.toHaveBeenCalled();
      fireEvent.blur(input);
      expect(state.onFieldBlur).toHaveBeenCalledOnce();
      vi.mocked(state.onFieldBlur).mockClear();
    }
    await user.click(screen.getByRole("button", { name: "settings.providers.kind" }));
    await user.click(screen.getByRole("option", { name: "Image generation" }));
    expect(state.onProviderKindChange).toHaveBeenCalledWith("image_generation");
    await user.click(screen.getByRole("button", { name: "settings.providers.api_format" }));
    await user.click(screen.getByRole("option", { name: "Chat Completions" }));
    expect(state.onApiFormatChange).toHaveBeenCalledWith("chat_completions");
  });

  it("presents fixed endpoints as a named read-only group and removes dangling labels", () => {
    const state = props({ usesBuiltinEndpoint: true, showProviderShapeControls: false, selectedCanManage: false });
    const { container } = render(view(<ProviderSettingsConfigForm {...state} />));
    const endpoints = screen.getByRole("group", { name: "settings.providers.base_url" });
    expect(within(endpoints).queryByRole("textbox")).toBeNull();
    expect(within(endpoints).queryByRole("button")).toBeNull();
    for (const format of FORMATS) {
      const row = within(endpoints).getByText(format.base_url).parentElement!;
      expect(row.getAttribute("tabindex")).toBeNull();
      expect(row.getAttribute("role")).toBeNull();
    }
    for (const label of container.querySelectorAll<HTMLLabelElement>("label[for]")) {
      expect(label.control).not.toBeNull();
    }
  });

  it("rebinds the endpoint label when switching between fixed and editable modes", () => {
    const state = props({ usesBuiltinEndpoint: true });
    const { rerender } = render(view(<ProviderSettingsConfigForm {...state} />));
    expect(screen.getByRole("group", { name: "settings.providers.base_url" })).toBeTruthy();
    rerender(view(<ProviderSettingsConfigForm {...state} usesBuiltinEndpoint={false} />));
    expect(screen.queryByRole("group", { name: "settings.providers.base_url" })).toBeNull();
    const input = screen.getByLabelText("settings.providers.base_url", { exact: false }) as HTMLInputElement;
    expect(input.required).toBe(true);
    expect(input.value).toBe(state.draft.base_url);
    fireEvent.blur(input);
    expect(state.onFieldBlur).toHaveBeenCalledOnce();
  });

  it("keeps management and single-option locks on the actual controls", async () => {
    const state = props({ selectedCanManage: false });
    const user = userEvent.setup();
    const { container, rerender } = render(view(<ProviderSettingsConfigForm {...state} />));
    for (const input of container.querySelectorAll<HTMLInputElement>("input")) expect(input.disabled).toBe(true);
    for (const button of screen.getAllByRole("button")) {
      expect((button as HTMLButtonElement).disabled).toBe(true);
      await user.click(button);
    }
    expect(state.onProviderKindChange).not.toHaveBeenCalled();
    expect(state.onApiFormatChange).not.toHaveBeenCalled();
    rerender(view(<ProviderSettingsConfigForm {...state} selectedCanManage
      providerKindOptions={state.providerKindOptions.slice(0, 1)} formatOptions={state.formatOptions.slice(0, 1)} />));
    for (const button of screen.getAllByRole("button")) expect((button as HTMLButtonElement).disabled).toBe(true);
  });

  it("keeps editing credentials blank and optional, with preset naming and kind locked", () => {
    const state = props({ isEditing: true });
    state.draft.preset_key = "builtin";
    render(view(<ProviderSettingsConfigForm {...state} />));
    const token = screen.getByLabelText("settings.providers.api_key", { exact: false }) as HTMLInputElement;
    expect(token.type).toBe("password");
    expect(token.value).toBe("");
    expect(token.required).toBe(false);
    expect((screen.getByLabelText("settings.providers.provider_name", { exact: false }) as HTMLInputElement).disabled).toBe(true);
    expect((screen.getByRole("button", { name: "settings.providers.kind" }) as HTMLButtonElement).disabled).toBe(true);
    expect((screen.getByRole("button", { name: "settings.providers.api_format" }) as HTMLButtonElement).disabled).toBe(false);
  });
});
