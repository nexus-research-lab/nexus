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
    isEditing: false, pending: false, onClearAuthToken: vi.fn(),
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
});
