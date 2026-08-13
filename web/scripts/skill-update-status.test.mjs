import assert from "node:assert/strict";
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

test("手动更新检查明确区分最新、可更新和来源失败", async () => {
  const [noticeModel, messagesModule] = await Promise.all([
    server.ssrLoadModule(
      "/src/features/capability/skills/controller/skill-update-check-model.ts",
    ),
    server.ssrLoadModule("/src/shared/i18n/messages.ts"),
  ]);
  const { buildSkillUpdateCheckNotice, formatSkillUpdateCheckNotice } = noticeModel;
  const t = createTranslate(messagesModule.MESSAGES.zh);

  const current = buildSkillUpdateCheckNotice(0, [], true);
  assert.equal(current?.status, "current");
  assert.equal(formatSkillUpdateCheckNotice(current, t), "已是最新版本");
  const updates = buildSkillUpdateCheckNotice(2, [], true);
  assert.equal(updates?.status, "updates");
  assert.equal(formatSkillUpdateCheckNotice(updates, t), "发现 2 个可更新");

  const failure = buildSkillUpdateCheckNotice(0, [{
    error: "此 Skill 记录的远端分支已不存在（deleted-branch），因此无法检查更新；请删除该 Skill 后从有效分支重新导入",
    skill_name: "skill-update-probe",
  }], true);
  assert.equal(failure?.status, "failure");
  const message = formatSkillUpdateCheckNotice(failure, t);
  assert.match(message, /skill-update-probe 检查失败/);
  assert.match(message, /远端分支已不存在/);
  assert.match(message, /重新导入/);
  assert.doesNotMatch(message, /暂无可更新/);
});

test("部分失败保留首条具体原因和其余失败数量", async () => {
  const [noticeModel, messagesModule] = await Promise.all([
    server.ssrLoadModule(
      "/src/features/capability/skills/controller/skill-update-check-model.ts",
    ),
    server.ssrLoadModule("/src/shared/i18n/messages.ts"),
  ]);
  const { buildSkillUpdateCheckNotice, formatSkillUpdateCheckNotice } = noticeModel;
  const notice = buildSkillUpdateCheckNotice(1, [
    { error: "远端分支已删除", skill_name: "first-skill" },
    { error: "来源不可访问", skill_name: "second-skill" },
  ], true);

  assert.equal(notice?.status, "updates");
  const message = formatSkillUpdateCheckNotice(
    notice,
    createTranslate(messagesModule.MESSAGES.zh),
  );
  assert.match(message, /发现 1 个可更新/);
  assert.match(message, /first-skill 检查失败：远端分支已删除/);
  assert.match(message, /另有 1 个 Skill 检查失败/);
});

test("目录高亮直接使用结构化失败状态而不是匹配提示文案", async () => {
  const [catalogModel, messagesModule] = await Promise.all([
    server.ssrLoadModule(
      "/src/features/capability/skills/catalog/skills-catalog-model.ts",
    ),
    server.ssrLoadModule("/src/shared/i18n/messages.ts"),
  ]);
  const { buildSkillsUpdateModel } = catalogModel;
  const t = createTranslate(messagesModule.MESSAGES.zh);
  const model = buildSkillsUpdateModel({
    checkingUpdates: false,
    checkUpdateNotice: {
      availableCount: 0,
      failure: {
        additionalCount: 0,
        reason: "来源已经失效，请重新导入",
        skillName: "probe",
      },
      status: "failure",
    },
    lastUpdateCheckedAt: Date.now(),
    updateCount: 0,
  }, { locale: "zh", t });

  assert.equal(model?.status, "failure");
  assert.match(model?.statusLabel ?? "", /来源已经失效，请重新导入/);
});

function createTranslate(messages) {
  return (key, params) => messages[key].replace(/\{(\w+)\}/g, (match, name) => (
    params?.[name] ?? match
  ));
}

test("工作循环元数据不暴露协议枚举或固定英文计数", async () => {
  const { buildLoopMetadataPresentation, getLoopTriggerLabel } =
    await server.ssrLoadModule(
      "/src/features/capability/loops/loop-presentation.ts",
    );
  const messages = {
    "capability.loops_installs": "安装 {count} 次",
    "capability.loops_trigger_event": "事件触发",
    "capability.loops_trigger_interval": "定时触发",
    "capability.loops_trigger_manual": "手动",
    "capability.loops_views": "浏览 {count} 次",
  };
  const translate = (key, params) =>
    (messages[key] ?? key).replace("{count}", String(params?.count ?? ""));

  assert.deepEqual(
    buildLoopMetadataPresentation(
      { installs: 1811, trigger_type: "manual", views: 1502 },
      "zh",
      translate,
    ),
    {
      installsLabel: "安装 1,811 次",
      triggerLabel: "手动",
      viewsLabel: "浏览 1,502 次",
    },
  );
  assert.equal(getLoopTriggerLabel("event", translate), "事件触发");
  assert.equal(getLoopTriggerLabel("interval", translate), "定时触发");
  assert.equal(getLoopTriggerLabel("custom", translate), "custom");
});

test("英文社区 Skill 结果、预览与来源状态保持同一语言", async () => {
  const [skillModel, resultsModel, messagesModule] = await Promise.all([
    server.ssrLoadModule(
      "/src/features/capability/skills/external/external-skill-model.ts",
    ),
    server.ssrLoadModule(
      "/src/features/capability/skills/external/external-results-model.ts",
    ),
    server.ssrLoadModule("/src/shared/i18n/messages.ts"),
  ]);
  const t = createTranslate(messagesModule.MESSAGES.en);
  const localization = { t };
  const item = {
    description: "",
    detail_url: "https://skills.sh/example/skills/pdf",
    git_branch: "",
    git_path: "",
    git_url: "",
    import_mode: "skills_sh",
    installs: 171000,
    name: "pdf",
    package_spec: "example/skills/pdf",
    raw_url: "",
    readme_markdown: "",
    skill_slug: "pdf",
    source: "example/skills",
    source_key: "skills-sh",
    source_kind: "skills_sh",
    source_name: "skills.sh",
    source_trust: "community",
    tags: [],
    title: "pdf",
    version: "example/skills/pdf",
  };

  const listItem = skillModel.buildExternalSkillListItemModel(
    item,
    new Map(),
    new Set(),
    localization,
  );
  assert.equal(listItem.description, "Search result from skills.sh");
  assert.equal(listItem.importState.label, "Available");
  assert.equal(listItem.installLabel, "171K installs");

  const preview = skillModel.buildExternalSkillPreviewModel(
    item,
    new Map(),
    new Set(),
    false,
    localization,
  );
  assert.match(preview.markdown, /does not provide an in-app preview/);
  assert.doesNotMatch(preview.markdown, /暂不提供|打开原始页面/);

  const results = resultsModel.buildExternalResultsModel({
    activeSourceKey: "hermes",
    items: [],
    loading: false,
    localization,
    statuses: [],
    submittedQuery: "pdf",
    sources: [{
      enabled: false,
      kind: "hermes_index",
      name: "Hermes Skills Index",
      sort_order: 1,
      source_id: "hermes",
      trust: "community",
      url: "https://example.com/hermes.json",
    }],
  });
  assert.equal(
    resultsModel.sourceGroupSummaryLabel(results.groups[0], localization),
    "Disabled",
  );
  assert.match(
    resultsModel.sourceGroupEmptyMessage(results.groups[0], localization),
    /This source is disabled/,
  );
});

test("单次定时任务使用无歧义的年在前日期", async () => {
  const { formatDatetimeDisplay } = await server.ssrLoadModule(
    "/src/features/capability/scheduled/pickers/picker-formatters.ts",
  );

  assert.equal(
    formatDatetimeDisplay("2026-08-02", "20", "29", "09"),
    "2026/08/02 下午 08:29:09",
  );
});

test("英文定时任务的模板、日期和校验提示保持同一语言", async () => {
  const [formatters, boardModel, submitModel, messagesModule] =
    await Promise.all([
      server.ssrLoadModule(
        "/src/features/capability/scheduled/pickers/picker-formatters.ts",
      ),
      server.ssrLoadModule(
        "/src/features/capability/scheduled/board/scheduled-task-board-model.ts",
      ),
      server.ssrLoadModule(
        "/src/features/capability/scheduled/dialog/form/task-form-submit.ts",
      ),
      server.ssrLoadModule("/src/shared/i18n/messages.ts"),
    ]);
  const translate = (key) => messagesModule.MESSAGES.en[key] ?? key;

  assert.equal(
    formatters.formatDatetimeDisplay(
      "2026-08-02",
      "20",
      "29",
      "09",
      "en",
    ),
    "2026/08/02 08:29:09 PM",
  );

  const suggestions = boardModel.buildScheduledTaskSuggestions(translate);
  assert.equal(suggestions[0].title, "Daily work brief");
  assert.equal(suggestions[0].preset.taskName, "Daily work brief");
  assert.match(suggestions[0].preset.instruction, /highest-priority work/);

  assert.equal(
    submitModel.getTaskDialogValidationError({
      form: { taskName: "" },
    }, translate),
    "Enter a task name",
  );
});

test("情绪能力默认关闭，用户开启后写回偏好", async () => {
  const [runtimeOptions, preferencesModel] = await Promise.all([
    server.ssrLoadModule("/src/config/runtime-options.ts"),
    server.ssrLoadModule(
      "/src/features/settings/general/model/settings-preferences-model.ts",
    ),
  ]);
  const original = runtimeOptions.getUserPreferences();
  const enabled = { ...original, emotion_enabled: true };

  assert.equal(original.emotion_enabled, false);
  try {
    runtimeOptions.setUserPreferences(enabled);
    assert.equal(runtimeOptions.getUserPreferences().emotion_enabled, true);
    assert.equal(
      preferencesModel.buildPreferencesUpdatePayload(enabled).emotion_enabled,
      true,
    );
  } finally {
    runtimeOptions.setUserPreferences(original);
  }
});

test("定时任务恢复运行后不把上一段权限错误当成当前异常", async () => {
  const boardModel = await server.ssrLoadModule(
    "/src/features/capability/scheduled/board/scheduled-task-board-model.ts",
  );
  const task = {
    agent_id: "agent-1",
    configuration_version: 1,
    delivery: { mode: "none" },
    enabled: true,
    execution_kind: "agent",
    failure_streak: 1,
    instruction: "读取飞书文档",
    job_id: "task-1",
    last_error: "任务需要使用 mcp__nexus_connectors__feishu_docx_read。",
    last_run_status: "awaiting_approval",
    name: "飞书权限回归测试",
    permission_state: "ready",
    running: true,
    running_started_at: Date.now(),
    schedule: {
      interval_seconds: 3600,
      kind: "every",
      timezone: "Asia/Shanghai",
    },
    session_target: { kind: "isolated", wake_mode: "next-heartbeat" },
    source: {
      context_id: "agent-1",
      context_label: "Kevin",
      context_type: "agent",
      kind: "agent",
    },
  };
  const pending = {
    isDeleting: false,
    isPermissionPending: false,
    isRunning: false,
    isToggling: false,
  };

  const resumed = boardModel.getScheduledTaskCardPresentation(task, pending);
  assert.equal(resumed.columnId, "running");
  assert.equal(resumed.lastError, null);

  const failed = boardModel.getScheduledTaskCardPresentation({
    ...task,
    last_run_status: "failed",
    running: false,
    running_started_at: null,
  }, pending);
  assert.equal(failed.columnId, "attention");
  assert.match(failed.lastError ?? "", /feishu_docx_read/);

  const awaitingApproval = boardModel.getScheduledTaskCardPresentation({
    ...task,
    failure_streak: 0,
    last_error: null,
    pending_permission_request: {
      capability: {
        connector_id: "feishu-docx",
        effect: "read",
        tool_name: "mcp__nexus_connectors__feishu_docx_read",
      },
      created_at: "2026-08-10T08:30:00Z",
      job_id: "task-1",
      kind: "tool",
      policy_revision: 1,
      request_id: "permission-1",
      resume_safe: true,
      status: "pending",
      updated_at: "2026-08-10T08:30:00Z",
    },
    permission_state: "awaiting_approval",
    running: false,
    running_started_at: null,
  }, pending);
  assert.equal(awaitingApproval.permission?.title, "飞书文档读取需要确认");
  assert.match(awaitingApproval.timingSummary, /^请求于 /);
  assert.notEqual(
    awaitingApproval.timingSummary,
    awaitingApproval.permission?.title,
  );
});

test("删除绑定会话后定时任务进入需处理并锁定执行和启用", async () => {
  const boardModel = await server.ssrLoadModule(
    "/src/features/capability/scheduled/board/scheduled-task-board-model.ts",
  );
  const task = {
    agent_id: "agent-1",
    configuration_version: 1,
    delivery: { mode: "last", session_key: "deleted-delivery" },
    enabled: false,
    execution_kind: "agent",
    failure_streak: 0,
    instruction: "发送工作简报",
    job_id: "task-rebind",
    name: "工作简报",
    permission_state: "ready",
    running: false,
    schedule: {
      interval_seconds: 3600,
      kind: "every",
      timezone: "Asia/Shanghai",
    },
    session_binding_issues: ["execution", "delivery"],
    session_binding_state: "rebind_required",
    session_target: { bound_session_key: "deleted-execution", kind: "bound" },
    source: { kind: "agent" },
  };
  const presentation = boardModel.getScheduledTaskCardPresentation(task, {
    isDeleting: false,
    isPermissionPending: false,
    isRunning: false,
    isToggling: false,
  });

  assert.equal(presentation.columnId, "attention");
  assert.equal(presentation.binding?.title, "需要重新绑定会话");
  assert.match(presentation.binding?.description ?? "", /执行会话和结果投递会话/);
  assert.equal(presentation.runAction.disabled, true);
  assert.equal(presentation.toggleAction.disabled, true);
  assert.equal(presentation.toggleAction.label, "等待重新绑定");
  assert.equal(presentation.timingSummary, "任务已暂停 · 等待重新绑定");
});

test("桌面数据根示例跟随平台", async () => {
  const [model, messagesModule] = await Promise.all([
    server.ssrLoadModule(
      "/src/features/settings/general/model/workspace-settings-model.ts",
    ),
    server.ssrLoadModule("/src/shared/i18n/messages.ts"),
  ]);

  const macStateRootKey = model.getStateRootPlaceholderKey("macos");
  const windowsStateRootKey = model.getStateRootPlaceholderKey("windows");
  assert.equal(
    messagesModule.MESSAGES.en[macStateRootKey],
    "e.g. /Volumes/NexusData",
  );
  assert.equal(
    messagesModule.MESSAGES.en[windowsStateRootKey],
    "e.g. D:\\NexusData",
  );
});

test("桌面数据根只有发生变化时才允许迁移", async () => {
  const model = await server.ssrLoadModule(
    "/src/features/settings/general/model/workspace-settings-model.ts",
  );
  const applied = model.buildStateRootSettingsSnapshot({
    current_path: "/Users/you/.nexus",
  });
  const samePath = model.replaceWorkspaceDraft(
    applied,
    "/Users/you/.nexus",
  );
  assert.equal(model.canSaveWorkspaceSettings(samePath, false), false);
  const changedPath = model.replaceWorkspaceDraft(
    applied,
    "/Volumes/NexusData",
  );
  assert.equal(model.canSaveWorkspaceSettings(changedPath, false), true);
  const emptyPath = model.replaceWorkspaceDraft(applied, "  ");
  assert.equal(model.canSaveWorkspaceSettings(emptyPath, false), false);
});
