// INPUT: Provider 目录、语言变化与同轮重复表单提交。
// OUTPUT: 语言切换保留向导草稿；一次连接意图只生成一次持久阶段指纹。
// POS: Provider 初始化向导交互回归，无真实网络或密钥持久化。
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import { ProviderSetupDialog } from "./provider-setup-dialog";

const api = vi.hoisted(() => ({ presets: vi.fn(), providers: vi.fn(), fingerprint: vi.fn() }));
vi.mock("@/shared/auth/auth-context", () => ({ useAuth: () => ({ status: { authenticated: true } }) }));
vi.mock("@/shared/auth/auth-owner-identity", () => ({ resolveAuthOwnerScope: () => "test-owner" }));
vi.mock("@/config/desktop-runtime", () => ({ isDesktopRuntime: () => false, getDesktopRuntimeConfig: () => null }));
vi.mock("@/features/provider-imports/cc-switch/provider-ccswitch-dialog", () => ({ ProviderCCSwitchDialog: () => null }));
vi.mock("@/lib/api/settings/provider-api", () => ({
  listProviderPresetsApi: api.presets, listProviderConfigsApi: api.providers,
  createProviderConfigApi: vi.fn(), testProviderConfigApi: vi.fn(), testProviderModelApi: vi.fn(), updateProviderConfigApi: vi.fn(),
}));
vi.mock("./provider-setup-recovery", async (original) => ({
  ...await original<typeof import("./provider-setup-recovery")>(), fingerprintProviderSetup: api.fingerprint,
}));
beforeEach(() => {
  vi.clearAllMocks();
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
