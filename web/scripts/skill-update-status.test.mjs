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

test("技能来源切换下沉到搜索工具区并使用无外框文字筛选", async () => {
  const [directorySource, searchSource] = await Promise.all([
    "src/features/capability/skills/skills-directory.tsx",
    "src/features/capability/skills/skills-search-bar.tsx",
  ].map((file) => readFile(path.join(webRoot, file), "utf8")));

  assert.match(directorySource, /onChangeDiscoveryMode=\{setDiscoveryMode\}/);
  assert.match(searchSource, /data-tour-anchor=\{SKILLS_TOUR_ANCHORS\.modes\}/);
  assert.match(searchSource, /role="group"/);
  assert.match(searchSource, /aria-pressed=\{active\}/);
  assert.match(searchSource, /inline-flex h-8 w-full shrink-0 items-center gap-1/);
  assert.match(searchSource, /sm:max-w-\[520px\]/);
  assert.match(searchSource, /bg-\(--surface-interactive-active-background\)/);
});

test("能力目录移除重复 Surface 身份并把页面动作收进正文标题区", async () => {
  const capabilityFiles = [
    "src/features/capability/channels/channels-directory.tsx",
    "src/features/capability/channels/pairings-directory.tsx",
    "src/features/capability/connectors/connectors-directory.tsx",
    "src/features/capability/loops/loops-directory.tsx",
    "src/features/capability/scheduled/scheduled-tasks-directory.tsx",
    "src/features/capability/skills/skills-directory.tsx",
  ];
  const [layoutSource, headerSource, ...directorySources] = await Promise.all([
    "src/features/capability/shared/capability-page-layout.tsx",
    "src/shared/ui/layout/workspace-content-header.tsx",
    ...capabilityFiles,
  ].map((file) => readFile(path.join(webRoot, file), "utf8")));

  assert.match(layoutSource, /actions\?: ReactNode/);
  assert.match(layoutSource, /WORKSPACE_CONTENT_PAGE_CLASS_NAME/);
  assert.match(layoutSource, /WorkspaceContentHeader/);
  assert.doesNotMatch(layoutSource, /variant\?: "board"/);
  assert.match(headerSource, /sm:flex-row sm:items-start sm:justify-between/);
  assert.match(headerSource, /data-tour-anchor=\{headerAnchor\}/);
  directorySources.forEach((source, index) => {
    assert.doesNotMatch(
      source,
      /WorkspaceSurfaceHeader/,
      `${capabilityFiles[index]} 不应恢复重复的能力身份 Header`,
    );
  });
  assert.match(directorySources[0], /actions=\{\(/);
  assert.match(directorySources[1], /actions=\{\(/);
  assert.match(directorySources[4], /actions=\{\(/);
  assert.match(directorySources[5], /actions=\{\(/);
});

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

test("Git Skill 导入字段与可见标签建立可访问关联", async () => {
  const source = await readFile(
    path.join(
      webRoot,
      "src/features/capability/skills/import/skill-import-source.tsx",
    ),
    "utf8",
  );

  ["url", "branch", "path"].forEach((field) => {
    const id = `skill-import-git-${field}`;
    assert.match(source, new RegExp(`htmlFor="${id}"`));
    assert.match(source, new RegExp(`id="${id}"`));
  });
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

test("工作区路径示例跟随桌面平台", async () => {
  const [model, messagesModule] = await Promise.all([
    server.ssrLoadModule(
      "/src/features/settings/general/model/workspace-settings-model.ts",
    ),
    server.ssrLoadModule("/src/shared/i18n/messages.ts"),
  ]);

  const macKey = model.getWorkspacePathPlaceholderKey("macos");
  const windowsKey = model.getWorkspacePathPlaceholderKey("windows");
  assert.equal(
    messagesModule.MESSAGES.en[macKey],
    "e.g. /Users/you/Nexus/workspaces",
  );
  assert.equal(
    messagesModule.MESSAGES.en[windowsKey],
    "e.g. D:\\Nexus\\workspace",
  );
  assert.equal(
    model.getWorkspacePathPlaceholderKey("linux"),
    "settings.general.workspace_path_placeholder_posix",
  );
});
