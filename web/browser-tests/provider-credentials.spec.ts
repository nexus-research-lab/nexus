// INPUT: Real settings routes with isolated Provider and preference responses.
// OUTPUT: Credential lifecycle, empty default selection and responsive page evidence.
// POS: Browser integration regression; every API request is mocked or blocked.
import { expect, test } from "@playwright/test";
import { appShellRead, APP_SHELL_INIT_SCRIPT } from "./native-ui-app-fixtures.mjs";

test("Provider credentials preserve models and cleared keys leave defaults empty", async ({ page, context }, info) => {
  const text = (zh: string, en: string) => info.project.metadata.locale === "zh" ? zh : en;
  const model = { id: "model", provider_id: "provider", model_id: "qa-model", display_name: "QA Model",
    category: "chat", enabled: true, is_default: true, capabilities_auto: {}, capabilities_override: {}, provider_options: {} };
  let record = { id: "provider", provider: "qa-provider", preset_key: "custom", visibility: "private", provider_kind: "llm",
    api_format: "responses", display_name: "QA Provider", base_url: "https://example.test/v1", models_path: "/models",
    enabled: true, can_manage: true, configuration_version: 1, auth_token_masked: "test-****",
    usage_count: 0, used_by_agents: [], last_test_status: "", last_test_error: "", agent_runtime_supported: true, models: [model] };
  const commands: Record<string, unknown>[] = [];
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await context.addInitScript(APP_SHELL_INIT_SCRIPT);
  await context.routeWebSocket("**/nexus/v1/chat/ws", (socket) => socket.onMessage((raw) => {
    if (JSON.parse(raw.toString()).type === "ping") socket.send(JSON.stringify({ event_type: "pong" }));
  }));
  await context.route("**/*", async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    if (path === "/nexus/v1/settings/providers/qa-provider" && request.method() === "PUT") {
      const payload = request.postDataJSON();
      commands.push(payload);
      record = { ...record, ...payload, configuration_version: record.configuration_version + 1,
        auth_token_masked: payload.auth_token === "" ? "" : record.auth_token_masked };
      return route.fulfill({ json: { data: record } });
    }
    if (request.method() === "GET") {
      if (path === "/nexus/v1/settings/providers") return route.fulfill({ json: { data: [record] } });
      if (path === "/nexus/v1/settings/provider-presets") return route.fulfill({ json: { data: [
        { preset_key: "custom", provider_kind: "llm", endpoint_mode: "custom", display_name: "Custom",
          description: "", key_url: "", default_api_format: "responses",
          formats: [{ api_format: "responses", base_url: "", models_path: "/models" }] },
      ] } });
      if (path === "/nexus/v1/settings/preferences") return route.fulfill({ json: { data: {
        version: 1, agent_runtime_kind: "nxs", default_agent_options: { provider: "qa-provider", model: "qa-model" },
      } } });
      if (path === "/nexus/v1/settings/providers/options") {
        const ready = record.enabled && Boolean(record.auth_token_masked);
        return route.fulfill({ json: { data: {
          default_provider: ready ? record.provider : null, default_model: ready ? "qa-model" : null,
          default_selection: ready ? { provider: record.provider, model: "qa-model" } : null,
          default_image_provider: null, default_image_model: null, default_image_selection: null,
          items: ready ? [{ provider: record.provider, display_name: record.display_name, models: [model] }] : [],
          background_items: [], image_items: [], vision_items: [],
        } } });
      }
    }
    const fixture = appShellRead(request.method(), path);
    if (fixture) return route.fulfill({ json: fixture });
    if (["fetch", "xhr"].includes(request.resourceType()) || request.method() !== "GET"
      || !["127.0.0.1", "localhost"].includes(new URL(request.url()).hostname)) return route.abort();
    return route.continue();
  });
  async function open(section: string) {
    const params = new URLSearchParams({ section, theme: String(info.project.metadata.theme), locale: String(info.project.metadata.locale) });
    await page.goto(`/app.html?desktop_route=${encodeURIComponent(`/settings?${params}`)}`);
  }
  await open("default-models");
  const defaultModel = () => page.getByRole("button", { name: text("默认对话模型", "Default chat model"), exact: true });
  await expect(defaultModel()).toContainText("QA Provider / QA Model");
  await open("providers");
  await page.getByRole("button", { name: "QA Provider", exact: true }).click();
  const toggle = page.getByRole("switch").first();
  await expect(toggle).toBeChecked();
  await toggle.click();
  await expect(toggle).not.toBeChecked();
  await expect(toggle).toBeEnabled();
  expect(commands[0]).toMatchObject({ enabled: false });
  expect(commands[0]).not.toHaveProperty("auth_token");
  await toggle.click();
  await expect(toggle).toBeChecked();
  await expect(toggle).toBeEnabled();
  await page.getByRole("button", { name: text("更换密钥", "Replace key"), exact: true }).click();
  const input = page.getByLabel(text("API 密钥", "API Key"), { exact: false });
  await input.fill("replacement-test-key");
  await page.getByRole("button", { name: text("保存", "Save"), exact: true }).click();
  await expect(page.getByRole("button", { name: text("更换密钥", "Replace key"), exact: true })).toBeVisible();
  expect(commands[2]).toMatchObject({ auth_token: "replacement-test-key", enabled: true });
  const clear = page.getByRole("button", { name: text("清除密钥", "Clear key"), exact: true });
  await clear.click();
  expect(commands).toHaveLength(3);
  await page.getByRole("dialog").getByRole("button", { name: text("清除密钥", "Clear key"), exact: true }).click();
  await expect(toggle).not.toBeChecked();
  await expect(toggle).toBeEnabled();
  expect(commands[3]).toMatchObject({ auth_token: "", enabled: false });
  await expect(page.getByText("QA Model", { exact: true }).first()).toBeVisible();
  await info.attach("provider-key-cleared", { body: await page.screenshot(), contentType: "image/png" });
  await open("default-models");
  await expect(defaultModel()).toBeDisabled();
  await expect(defaultModel()).toContainText(text("先启用一个模型", "Enable a model first"));
  expect(await page.locator(".settings-content-body").evaluate((element) => element.scrollWidth - element.clientWidth)).toBeLessThanOrEqual(1);
  await info.attach("empty-default-models", { body: await page.screenshot(), contentType: "image/png" });
  expect(errors).toEqual([]);
});
