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
  const { buildSkillUpdateCheckNotice } = await server.ssrLoadModule(
    "/src/features/capability/skills/controller/skill-update-check-model.ts",
  );

  assert.deepEqual(
    buildSkillUpdateCheckNotice(0, [], true),
    { message: "已是最新版本", status: "current" },
  );
  assert.deepEqual(
    buildSkillUpdateCheckNotice(2, [], true),
    { message: "发现 2 个可更新", status: "updates" },
  );

  const failure = buildSkillUpdateCheckNotice(0, [{
    error: "此 Skill 记录的远端分支已不存在（deleted-branch），因此无法检查更新；请删除该 Skill 后从有效分支重新导入",
    skill_name: "skill-update-probe",
  }], true);
  assert.equal(failure?.status, "failure");
  assert.match(failure?.message ?? "", /skill-update-probe 检查失败/);
  assert.match(failure?.message ?? "", /远端分支已不存在/);
  assert.match(failure?.message ?? "", /重新导入/);
  assert.doesNotMatch(failure?.message ?? "", /暂无可更新/);
});

test("部分失败保留首条具体原因和其余失败数量", async () => {
  const { buildSkillUpdateCheckNotice } = await server.ssrLoadModule(
    "/src/features/capability/skills/controller/skill-update-check-model.ts",
  );
  const notice = buildSkillUpdateCheckNotice(1, [
    { error: "远端分支已删除", skill_name: "first-skill" },
    { error: "来源不可访问", skill_name: "second-skill" },
  ], true);

  assert.equal(notice?.status, "updates");
  assert.match(notice?.message ?? "", /发现 1 个可更新/);
  assert.match(notice?.message ?? "", /first-skill 检查失败：远端分支已删除/);
  assert.match(notice?.message ?? "", /另有 1 个 Skill 检查失败/);
});

test("目录高亮直接使用结构化失败状态而不是匹配提示文案", async () => {
  const { buildSkillsUpdateModel } = await server.ssrLoadModule(
    "/src/features/capability/skills/catalog/skills-catalog-model.ts",
  );
  const model = buildSkillsUpdateModel({
    checkingUpdates: false,
    checkUpdateNotice: {
      message: "来源已经失效，请重新导入",
      status: "failure",
    },
    lastUpdateCheckedAt: Date.now(),
    updateCount: 0,
  });

  assert.equal(model?.status, "failure");
  assert.equal(model?.statusLabel, "来源已经失效，请重新导入");
});

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
