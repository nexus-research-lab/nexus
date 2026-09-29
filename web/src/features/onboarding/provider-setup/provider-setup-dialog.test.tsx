// INPUT: Provider 目录、语言变化、同轮重复提交与保存/测试/默认偏好响应。
// OUTPUT: 保留草稿、防止重复提交，并验证偏好基线落盘后完成默认模型选择。
// POS: Provider 初始化向导交互回归，无真实网络或密钥持久化。
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import { ProviderSetupDialog } from "./provider-setup-dialog";

const api = vi.hoisted(() => ({ presets: vi.fn(), providers: vi.fn(), fingerprint: vi.fn(), create: vi.fn(), test: vi.fn(), fetchModels: vi.fn(), preferences: vi.fn(), updatePreferences: vi.fn() }));
vi.mock("@/shared/auth/auth-context", () => ({ useAuth: () => ({ status: { authenticated: true } }) }));
vi.mock("@/shared/auth/auth-owner-identity", () => ({ resolveAuthOwnerScope: () => "test-owner" }));
vi.mock("@/config/desktop-runtime", () => ({ isDesktopRuntime: () => false, getDesktopRuntimeConfig: () => null }));
vi.mock("@/features/provider-imports/cc-switch/provider-ccswitch-dialog", () => ({ ProviderCCSwitchDialog: () => null }));
vi.mock("@/lib/api/settings/provider-api", () => ({
  listProviderPresetsApi: api.presets, listProviderConfigsApi: api.providers,
  createProviderConfigApi: api.create, fetchProviderModelsApi: api.fetchModels, testProviderConfigApi: vi.fn(), testProviderModelApi: api.test, updateProviderConfigApi: vi.fn(),
}));
vi.mock("./provider-setup-recovery", async (original) => ({
  ...await original<typeof import("./provider-setup-recovery")>(), fingerprintProviderSetup: api.fingerprint,
}));
vi.mock("@/lib/api/settings/preferences-api", () => ({ getUserPreferencesApi: api.preferences, updateUserPreferencesApi: api.updatePreferences }));
vi.mock("@/config/runtime-options", () => ({ getDefaultAgentRuntimeKind: () => "nxs", setUserPreferences: vi.fn() }));
vi.mock("@/hooks/capability/use-provider-availability", () => ({ invalidateProviderAvailability: vi.fn() }));
beforeEach(() => {
  localStorage.clear();
  vi.resetAllMocks();
  api.providers.mockResolvedValue([]);
  api.presets.mockResolvedValue([{ preset_key: "sample", provider_kind: "llm", endpoint_mode: "fixed", display_name: "Sample", description: "Sample provider", key_url: "", default_api_format: "anthropic_messages", formats: [{ api_format: "anthropic_messages", base_url: "https://example.test", models_path: "/models" }] }]);
  api.fingerprint.mockImplementation(() => new Promise(() => {}));
});
afterEach(cleanup);
function view(locale: "en" | "zh") {
  return <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t: (key) => MESSAGES[locale][key] }}><ProviderSetupDialog isOpen onClose={vi.fn()} onStart={vi.fn()} /></I18N_CONTEXT.Provider>;
}
async function enterCredentials() {
  await screen.findByText("Sample");
  await userEvent.click(screen.getByRole("button", { name: MESSAGES.en["onboarding.provider_setup_provider_continue"] }));
  return screen.getByLabelText(/API Key/i);
}
it("changing language preserves the credentials scene and the typed secret", async () => {
  const { rerender } = render(view("en"));
  const input = await enterCredentials();
  await userEvent.type(input, "draft-secret");
  rerender(view("zh"));
  expect(screen.getByDisplayValue("draft-secret")).toBe(input);
  expect(api.presets).toHaveBeenCalledTimes(1);
  expect(api.providers).toHaveBeenCalledTimes(1);
});
it("duplicate submit events create only one connection intent", async () => {
  render(view("en"));
  const input = await enterCredentials();
  await userEvent.type(input, "draft-secret");
  const form = input.closest("form")!;
  act(() => { fireEvent.submit(form); fireEvent.submit(form); });
  expect(api.fingerprint).toHaveBeenCalledTimes(1);
});

it("completes token setup and records the preference revision before selecting defaults", async () => {
  api.fingerprint.mockResolvedValue("a".repeat(64));
  api.create.mockResolvedValue({ configuration_version: 1, display_name: "Sample" });
  api.fetchModels.mockResolvedValue({ models: [] });
  api.providers.mockResolvedValueOnce([]).mockResolvedValue([{
    provider: "sample", can_manage: true, configuration_version: 2,
    models: [{ model_id: "first-model", display_name: "First model" }, { model_id: "sample-model", display_name: "Second model" }],
  }]);
  api.test.mockResolvedValue({ success: true, configuration_version: 2, model: "sample-model" });
  api.preferences.mockResolvedValue({ version: 7, default_agent_options: {} });
  api.updatePreferences.mockImplementation(async () => {
    const journal = JSON.parse(localStorage.getItem("nexus.provider_setup.journal.v1:10:test-owner")!);
    expect(journal.stage).toBe("default");
    expect(journal.outcome).toBe("unknown");
    expect(journal.preferencesBaselineVersion).toBe(7);
    expect(JSON.stringify(journal)).not.toContain("draft-secret");
    return { version: 8, default_agent_options: { provider: "sample", model: "sample-model" } };
  });
  render(view("en"));
  const input = await enterCredentials();
  await userEvent.type(input, "draft-secret");
  fireEvent.submit(input.closest("form")!);
  const modelInput = await screen.findByLabelText(/^Model ID/);
  expect(api.test).not.toHaveBeenCalled();
  expect(api.updatePreferences).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole("button", { name: "Choose a model" }));
  await userEvent.click(await screen.findByRole("option", { name: "Second model (sample-model)" }));
  expect((modelInput as HTMLInputElement).value).toBe("sample-model");
  await userEvent.click(screen.getByRole("button", { name: "Verify and use as default" }));
  await waitFor(() => expect(api.updatePreferences).toHaveBeenCalledOnce());
  expect(api.test).toHaveBeenCalledWith("sample", "sample-model", { expectedVersion: 2 });
  expect(api.create).toHaveBeenCalledWith(expect.objectContaining({ auth_token: "draft-secret" }));
  expect(api.updatePreferences).toHaveBeenCalledWith(expect.objectContaining({
    default_agent_options: { provider: "sample", model: "sample-model" },
    default_background_model_selection: { provider: "sample", model: "sample-model" },
  }), { expectedVersion: 7 });
  await screen.findByRole("button", { name: MESSAGES.en["onboarding.provider_setup_enter_chat"] });
  expect(localStorage.getItem("nexus.provider_setup.journal.v1:10:test-owner")).toBeNull();
});

it("falls back to manual selection after discovery fails and resumes without saving credentials again", async () => {
  api.fingerprint.mockResolvedValue("a".repeat(64));
  api.create.mockResolvedValue({ configuration_version: 1, display_name: "Sample" });
  api.fetchModels.mockRejectedValue(new Error("catalog unavailable"));
  api.providers.mockResolvedValueOnce([]).mockResolvedValue([{ provider: "sample", can_manage: true, configuration_version: 1, models: [] }]);
  const rendered = render(view("en"));
  const input = await enterCredentials();
  await userEvent.type(input, "draft-secret");
  fireEvent.submit(input.closest("form")!);
  await screen.findByText(MESSAGES.en["onboarding.provider_setup_model_list_failed"]);
  rendered.unmount();
  render(view("en"));
  const model = await screen.findByLabelText(/^Model ID/);
  expect(api.create).toHaveBeenCalledOnce();
  expect(api.fetchModels).toHaveBeenCalledOnce();
  api.test.mockResolvedValue({ success: true, configuration_version: 2, model: "manual-model" });
  api.preferences.mockResolvedValue({ version: 7, default_agent_options: {} });
  api.updatePreferences.mockResolvedValue({ version: 8, default_agent_options: {} });
  await userEvent.type(model, "manual-model");
  await userEvent.click(screen.getByRole("button", { name: "Verify and use as default" }));
  await screen.findByRole("button", { name: MESSAGES.en["onboarding.provider_setup_enter_chat"] });
  expect(api.test).toHaveBeenCalledWith("sample", "manual-model", { expectedVersion: 1 });
  expect(api.create).toHaveBeenCalledOnce();
});

it("keeps an uncertain model test locked and reconciles without replay", async () => {
  api.fingerprint.mockResolvedValue("a".repeat(64));
  api.create.mockResolvedValue({ configuration_version: 1, display_name: "Sample" });
  api.fetchModels.mockResolvedValue({ models: [] });
  api.providers.mockResolvedValueOnce([]).mockResolvedValue([{ provider: "sample", can_manage: true, configuration_version: 2, models: [] }]);
  api.test.mockRejectedValue(new Error("lost response"));
  render(view("en"));
  const input = await enterCredentials();
  await userEvent.type(input, "draft-secret");
  fireEvent.submit(input.closest("form")!);
  const model = await screen.findByLabelText(/^Model ID/);
  await userEvent.type(model, "exact-model");
  await userEvent.click(screen.getByRole("button", { name: "Verify and use as default" }));
  const reconcile = await screen.findByRole("button", { name: MESSAGES.en["onboarding.provider_setup_reconcile_action"] });
  expect((screen.getByLabelText(/^Model ID/) as HTMLInputElement).readOnly).toBe(true);
  await userEvent.click(reconcile);
  await waitFor(() => expect(api.providers).toHaveBeenCalledTimes(5));
  expect(api.test).toHaveBeenCalledOnce();
  expect(api.updatePreferences).not.toHaveBeenCalled();
});
