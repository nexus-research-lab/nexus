import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

import { createServer } from "vite";

const webRoot = fileURLToPath(new URL("..", import.meta.url));
const server = await createServer({
  configFile: false,
  logLevel: "silent",
  resolve: { alias: { "@": path.join(webRoot, "src") } },
  root: webRoot,
  server: { middlewareMode: true },
});

test.after(async () => {
  await server.close();
});

test("login failures block duplicate submission unless the server says not applied", async () => {
  const { buildLoginSubmitFailure } = await server.ssrLoadModule(
    "/src/pages/login/login-page-model.ts",
  );
  const { ApiRequestError, ApiTransportError } = await server.ssrLoadModule(
    "/src/lib/api/core/http-error.ts",
  );
  const { zhMessages } = await server.ssrLoadModule(
    "/src/shared/i18n/catalog/zh/index.ts",
  );
  const t = (key) => zhMessages[key];

  const unknown = buildLoginSubmitFailure(
    new ApiTransportError("连接中断", "network", "unknown"),
    t,
  );
  assert.equal(unknown.blocksSubmit, true);
  assert.equal(unknown.action, "check_status");
  assert.match(unknown.nextStep, /不要再次提交/);

  const rejected = buildLoginSubmitFailure(
    new ApiRequestError("密码错误", 401, {
      version: 1,
      code: "auth.invalid_credentials",
      category: "authentication",
      effect: "not_applied",
    }),
    t,
  );
  assert.equal(rejected.blocksSubmit, false);
  assert.equal(rejected.action, null);
  assert.match(rejected.impact, /没有生效/);
});

test("provider feedback distinguishes rejected, unknown, and committed writes", async () => {
  const { buildProviderErrorFeedback } = await server.ssrLoadModule(
    "/src/features/settings/provider-settings/model/provider-feedback-model.ts",
  );
  const { ApiRequestError, ApiTransportError } = await server.ssrLoadModule(
    "/src/lib/api/core/http-error.ts",
  );
  const { zhMessages } = await server.ssrLoadModule(
    "/src/shared/i18n/catalog/zh/index.ts",
  );
  const t = (key) => zhMessages[key];

  const rejected = buildProviderErrorFeedback(
    new ApiRequestError("配置不合法", 400, {
      version: 1,
      code: "provider.invalid",
      category: "validation",
      effect: "not_applied",
    }),
    "保存失败",
    "保存失败",
    t,
  );
  assert.equal(rejected.tone, "error");
  assert.equal(rejected.recoveryAction, undefined);
  assert.match(rejected.impact, /没有改变/);

  const unknown = buildProviderErrorFeedback(
    new ApiTransportError("响应中断", "response_interrupted", "unknown"),
    "保存失败",
    "保存失败",
    t,
  );
  assert.equal(unknown.tone, "warning");
  assert.equal(unknown.recoveryAction, "refresh");
  assert.equal(unknown.mutationEffect, "unknown");
  assert.match(unknown.title, /无法确认/);

  const committed = buildProviderErrorFeedback(
    new ApiRequestError("响应写回失败", 500, {
      version: 1,
      code: "provider.saved",
      category: "internal",
      effect: "committed",
    }),
    "保存失败",
    "保存失败",
    t,
  );
  assert.match(committed.title, /已保存/);
  assert.equal(committed.mutationEffect, "committed");
  assert.match(committed.nextStep, /不要重复/);
});

test("authorization, onboarding, custom MCP, and Skill detail keep actionable triads", async () => {
  const authorization = await read(
    "src/features/capability/channels/authorization/channel-authorization-dialog.tsx",
  );
  const onboarding = await read(
    "src/features/onboarding/provider-setup/provider-setup-dialog.tsx",
  );
  const customMcp = await read(
    "src/features/capability/connectors/custom/use-custom-mcp-servers.ts",
  );
  const skillDetail = await read(
    "src/features/capability/skills/detail/skill-detail-view.tsx",
  );
  const loginController = await read(
    "src/pages/login/use-login-page-controller.ts",
  );

  assert.match(authorization, /failure\.impact/);
  assert.match(authorization, /failure\.nextStep/);
  assert.match(authorization, /AuthorizationExpired/);
  assert.match(onboarding, /"persist_unknown"/);
  assert.match(onboarding, /"test_failed"/);
  assert.match(onboarding, /"default_not_applied"/);
  assert.match(onboarding, /provider_setup_persist_unknown_impact/);
  assert.match(onboarding, /provider_setup_test_failed_next/);
  assert.match(onboarding, /provider_setup_default_not_applied_next/);
  assert.match(customMcp, /projectMutationFailure\(error, fallbackMessage\)/);
  assert.match(customMcp, /state\.reload_check/);
  assert.match(skillDetail, /onRetry/);
  assert.match(skillDetail, /state\.read_failure_impact/);
  assert.doesNotMatch(
    loginController,
    /refreshStatus\(\)\s*\.then\(\(\) => setSubmitFailure\(null\)\)/,
  );
});

test("Skill Agent binding reads stay separate from conservative toggle outcomes", async () => {
  const {
    buildSkillAgentBindingsReadFailure,
    buildSkillAgentToggleFailure,
    buildSkillAgentToggleFollowupFailure,
  } = await server.ssrLoadModule(
    "/src/features/capability/skills/detail/skill-detail-model.ts",
  );
  const { ApiRequestError, ApiTransportError } = await server.ssrLoadModule(
    "/src/lib/api/core/http-error.ts",
  );
  const { zhMessages } = await server.ssrLoadModule(
    "/src/shared/i18n/catalog/zh/index.ts",
  );
  const t = (key) => zhMessages[key];

  const read = buildSkillAgentBindingsReadFailure(
    new Error("读取失败"),
    t,
  );
  assert.match(read.impact, /不会修改|没有修改/);
  assert.match(read.nextStep, /重试|再试/);

  const unknown = buildSkillAgentToggleFailure(
    new ApiTransportError("连接中断", "response_interrupted", "unknown"),
    "agent-a",
    t,
  );
  assert.equal(unknown.effect, "unknown");
  assert.equal(unknown.blocksRepeat, true);
  assert.match(unknown.nextStep, /不要自动重复/);

  const rejected = buildSkillAgentToggleFailure(
    new ApiRequestError("不允许修改", 409, {
      version: 1,
      code: "skill.binding_rejected",
      category: "conflict",
      effect: "not_applied",
    }),
    "agent-a",
    t,
  );
  assert.equal(rejected.effect, "not_applied");
  assert.equal(rejected.blocksRepeat, false);

  const followup = buildSkillAgentToggleFollowupFailure(
    new Error("目录刷新失败"),
    "agent-a",
    t,
  );
  assert.equal(followup.effect, "committed");

  const controller = await readFile(
    path.join(
      webRoot,
      "src/features/capability/skills/detail/use-skill-detail-controller.ts",
    ),
    "utf8",
  );
  const loadBindingsBody = controller.slice(
    controller.indexOf("const loadBindings"),
    controller.indexOf("const loadDetail"),
  );
  assert.doesNotMatch(loadBindingsBody, /setToggleFailures/);
  assert.match(controller, /existingFailure\?\.blocksRepeat/);
  assert.match(controller, /startNewToggleIntent/);
});

test("Agent Options Skill reads and exact toggles keep independent outcomes", async () => {
  const {
    buildAgentSkillMutationFailure,
    buildAgentSkillRefreshAfterMutationFailure,
    buildAgentSkillsReadFailure,
  } = await server.ssrLoadModule(
    "/src/features/agents/options/components/skills/agent-skills-model.ts",
  );
  const { ApiRequestError, ApiTransportError } = await server.ssrLoadModule(
    "/src/lib/api/core/http-error.ts",
  );
  const { zhMessages } = await server.ssrLoadModule(
    "/src/shared/i18n/catalog/zh/index.ts",
  );
  const t = (key) => zhMessages[key];
  const target = {
    agentId: "agent-a",
    desiredEnabled: true,
    skillName: "research",
  };

  const readFailure = buildAgentSkillsReadFailure(new Error("读取失败"), t);
  assert.match(readFailure.impact, /没有修改/);

  const unknown = buildAgentSkillMutationFailure(
    new ApiTransportError("连接中断", "response_interrupted", "unknown"),
    target,
    t,
  );
  assert.equal(unknown.effect, "unknown");
  assert.equal(unknown.blocksRepeat, true);
  assert.deepEqual(unknown.target, target);
  assert.match(unknown.nextStep, /不要自动重复/);

  const rejected = buildAgentSkillMutationFailure(
    new ApiRequestError("更改被拒绝", 409, {
      version: 1,
      code: "skill.toggle_rejected",
      category: "conflict",
      effect: "not_applied",
    }),
    target,
    t,
  );
  assert.equal(rejected.effect, "not_applied");
  assert.equal(rejected.blocksRepeat, false);

  const committed = buildAgentSkillRefreshAfterMutationFailure(
    new Error("刷新失败"),
    target,
    t,
  );
  assert.equal(committed.effect, "committed");
  assert.equal(committed.blocksRepeat, true);

  const controller = await read(
    "src/features/agents/options/components/skills/use-agent-skills-controller.ts",
  );
  const resource = await read(
    "src/features/agents/options/components/skills/use-agent-skills-resource.ts",
  );
  assert.match(controller, /applyCommittedSkill\(committedSkill\)/);
  assert.match(controller, /failures\[skill\.name\]\?\.blocksRepeat/);
  assert.match(resource, /runRefresh\("background", false\)/);
  assert.doesNotMatch(resource, /setActionFailures/);
});

test("Composer picker reads and Session-setting writes preserve separate recovery", async () => {
  const {
    buildComposerReadFailure,
    buildComposerSettingsMutationFailure,
    createComposerSettingsMutationIntent,
    isSameComposerSettingsIntent,
  } = await server.ssrLoadModule(
    "/src/features/conversation/shared/composer/controller/composer-settings-reliability.ts",
  );
  const { ApiRequestError, ApiTransportError } = await server.ssrLoadModule(
    "/src/lib/api/core/http-error.ts",
  );
  const { zhMessages } = await server.ssrLoadModule(
    "/src/shared/i18n/catalog/zh/index.ts",
  );
  const t = (key) => zhMessages[key];
  const current = {
    connector_ids: ["connector-a"],
    model: "model-a",
    permission_mode: "default",
    provider: "provider-a",
  };
  const sameIntent = createComposerSettingsMutationIntent(
    "session-a",
    "model",
    current,
  );
  const changedIntent = createComposerSettingsMutationIntent(
    "session-a",
    "model",
    { ...current, model: "model-b" },
  );
  assert.equal(isSameComposerSettingsIntent(sameIntent, sameIntent), true);
  assert.equal(isSameComposerSettingsIntent(sameIntent, changedIntent), false);

  const readFailure = buildComposerReadFailure(
    new Error("目录读取失败"),
    "models",
    "加载模型失败",
    t,
  );
  assert.match(readFailure.impact, /草稿|输入/);
  assert.match(readFailure.nextStep, /重新加载/);

  const unknown = buildComposerSettingsMutationFailure(
    new ApiTransportError("响应中断", "response_interrupted", "unknown"),
    sameIntent,
    t,
  );
  assert.equal(unknown.effect, "unknown");
  assert.equal(unknown.blocksRepeat, true);
  assert.match(unknown.nextStep, /不要自动重复/);

  const notApplied = buildComposerSettingsMutationFailure(
    new ApiRequestError("更改被拒绝", 409, {
      version: 1,
      code: "session.settings_rejected",
      category: "conflict",
      effect: "not_applied",
    }),
    sameIntent,
    t,
  );
  assert.equal(notApplied.effect, "not_applied");
  assert.equal(notApplied.blocksRepeat, false);

  const settingsController = await read(
    "src/features/conversation/shared/composer/controller/use-composer-session-settings.ts",
  );
  const slashController = await read(
    "src/features/conversation/shared/composer/use-composer-slash-command.ts",
  );
  const reliabilityView = await read(
    "src/features/conversation/shared/composer/components/footer/composer-session-settings-reliability.tsx",
  );
  assert.match(settingsController, /isSameComposerSettingsIntent/);
  assert.match(settingsController, /!savingSessionKeysRef\.current\.has\(sessionKey\)/);
  assert.match(settingsController, /!mutationFailuresRef\.current\[sessionKey\]\?\.blocksRepeat/);
  assert.doesNotMatch(settingsController, /cacheSettings\(sessionKey, previous\);\s*setSettingsError/);
  assert.doesNotMatch(slashController, /技能列表加载失败|模型列表加载失败/);
  assert.match(reliabilityView, /impact=\{failure\.impact\}/);
  assert.match(reliabilityView, /nextStep=\{failure\.nextStep\}/);
});

test("Scheduled history fallbacks and ordinary async notices remain polite and localized", async () => {
  const scheduled = await read(
    "src/features/capability/scheduled/history/scheduled-task-run-history-dialog.tsx",
  );
  const authorization = await read(
    "src/features/capability/channels/authorization/channel-authorization-dialog.tsx",
  );
  const onboarding = await read(
    "src/features/onboarding/provider-setup/provider-setup-dialog.tsx",
  );
  const browser = await read(
    "src/features/settings/browser/browser-settings-section.tsx",
  );

  assert.match(scheduled, /scheduled_history_feedback_fallback_impact/);
  assert.match(scheduled, /scheduled_history_feedback_fallback_next_step/);
  assert.doesNotMatch(scheduled, /role=\{actions\.feedback\.tone/);
  assert.doesNotMatch(scheduled, /已有任务和运行记录保持不变。/);
  assert.match(authorization, /aria-live="polite"[\s\S]*role="status"/);
  assert.match(onboarding, /aria-live="polite"[\s\S]*role="status"/);
  assert.match(browser, /setupError \|\| statusError[\s\S]*aria-live="polite"[\s\S]*role="status"/);
  assert.match(browser, /settings\.browser\.incompatible_impact/);
  assert.match(browser, /settings\.browser\.incompatible_next_step/);
  assert.match(browser, /settings\.browser\.status_failed_impact/);
  assert.match(browser, /settings\.browser\.status_failed_next_step/);
  assert.match(browser, /settings\.browser\.install_failed_impact/);
  assert.match(browser, /settings\.browser\.install_failed_next_step/);
  assert.doesNotMatch(browser, /getErrorMessage/);
  assert.doesNotMatch(browser, /role="alert"/);
});

test("secondary read and validation surfaces state impact and a concrete next step", async () => {
  const subagentList = await read(
    "src/features/conversation/shared/subagent/subagent-task-list.tsx",
  );
  const subagentResource = await read(
    "src/features/conversation/shared/subagent/use-subagent-tasks.ts",
  );
  const identityModel = await read(
    "src/features/agents/options/components/identity/identity-model-selector.tsx",
  );
  const providerResource = await read(
    "src/features/agents/options/editor/use-agent-provider-options.ts",
  );
  const roomSkills = await read(
    "src/features/conversation/room/members/skills/room-skills-selector.tsx",
  );
  const roomSkillsResource = await read(
    "src/features/conversation/room/members/skills/use-room-skill-options.ts",
  );
  const connectorDetail = await read(
    "src/features/capability/connectors/detail/connector-detail-content.tsx",
  );
  const scheduledForm = await read(
    "src/features/capability/scheduled/dialog/schedule/task-schedule-panel.tsx",
  );
  const executionSurface = await read(
    "src/features/conversation/shared/execution/execution-workgraph-surface.tsx",
  );

  assert.match(subagentList, /subagents\.list_load_failed_impact/);
  assert.match(subagentList, /subagents\.list_load_failed_next_step/);
  assert.match(subagentList, /aria-live="polite"[\s\S]*role="status"/);
  assert.doesNotMatch(subagentResource, /error instanceof Error \? error\.message/);

  assert.match(identityModel, /provider_load_failed_impact/);
  assert.match(identityModel, /provider_load_failed_next_step/);
  assert.match(identityModel, /aria-live="polite"[\s\S]*role="status"/);
  assert.doesNotMatch(providerResource, /error instanceof Error \? error\.message/);

  assert.match(roomSkills, /room\.skills_load_error_impact/);
  assert.match(roomSkills, /room\.skills_load_error_next_step/);
  assert.match(roomSkills, /aria-live="polite"[\s\S]*role="status"/);
  assert.doesNotMatch(roomSkillsResource, /error instanceof Error \? error\.message/);

  assert.match(connectorDetail, /connector_configuration_unavailable_impact/);
  assert.match(connectorDetail, /connector_configuration_unavailable_next_step/);
  assert.doesNotMatch(connectorDetail, /description=\{error\}/);

  assert.match(scheduledForm, /scheduled_dialog_invalid_impact/);
  assert.match(scheduledForm, /scheduled_dialog_invalid_next_step/);
  assert.doesNotMatch(scheduledForm, /<UiStateBlock/);

  assert.match(executionSurface, /mode === "current" && resource\.error/);
  assert.match(executionSurface, /execution\.surface_stale_impact/);
  assert.match(executionSurface, /execution\.surface_unavailable_impact/);
  assert.match(executionSurface, /execution\.surface_failure_next_step/);
  assert.doesNotMatch(executionSurface, /mode === "current" && resource\.error[\s\S]{0,200}<UiIconButton/);
});

function read(relativePath) {
  return readFile(path.join(webRoot, relativePath), "utf8");
}
