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
