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

test("project mutations distinguish not-applied from an unknown outcome", async () => {
  const { buildProjectMutationFeedback } = await server.ssrLoadModule(
    "/src/features/settings/operations/project-admin/project-admin-model.ts",
  );
  const { ApiRequestError, ApiTransportError } = await server.ssrLoadModule(
    "/src/lib/api/core/http-error.ts",
  );
  const { zhMessages } = await server.ssrLoadModule(
    "/src/shared/i18n/catalog/zh/index.ts",
  );
  const t = (key) => zhMessages[key];

  const rejected = buildProjectMutationFeedback(
    t,
    "grant",
    new ApiRequestError("权限不允许", 403, {
      version: 1,
      code: "project.permission_denied",
      category: "authorization",
      effect: "not_applied",
    }),
  );
  assert.equal(rejected.title, "成员权限没有更新");
  assert.match(rejected.impact, /没有改变/);
  assert.equal(rejected.recoveryAction, undefined);

  const unknown = buildProjectMutationFeedback(
    t,
    "create",
    new ApiTransportError("连接已中断", "network", "unknown"),
  );
  assert.match(unknown.title, /无法确认/);
  assert.match(unknown.impact, /重复项目/);
  assert.match(unknown.nextStep, /不要再次创建/);
  assert.equal(unknown.recoveryAction, "refresh");

  const committed = buildProjectMutationFeedback(
    t,
    "grant",
    new ApiRequestError("响应写回失败", 500, {
      version: 1,
      code: "project.access_committed",
      category: "internal",
      effect: "committed",
    }),
  );
  assert.match(committed.title, /已更新/);
  assert.match(committed.impact, /已经保存在服务端/);
  assert.doesNotMatch(committed.impact, /可能已经保存|请求中断前/);
  assert.equal(committed.recoveryAction, "refresh");

  const accepted = buildProjectMutationFeedback(
    t,
    "create",
    new ApiRequestError("仍在处理", 202, {
      version: 1,
      code: "project.create_accepted",
      category: "unavailable",
      effect: "accepted",
    }),
  );
  assert.match(accepted.title, /正在处理/);
  assert.match(accepted.impact, /已经接收/);
  assert.match(accepted.nextStep, /不要再次创建/);
});

test("post-mutation refresh failure is a separate committed stage", async () => {
  const project = await read(
    "src/features/settings/operations/project-admin/use-project-admin.ts",
  );
  const sources = await read(
    "src/features/capability/skills/controller/use-external-skill-sources.ts",
  );
  const connectors = await read(
    "src/features/capability/connectors/controller/use-connector-commands.ts",
  );

  assert.match(
    project,
    /await updateProjectMemberApi[\s\S]*succeeded = true;[\s\S]*setProjects\(await getProjectsApi\(\)\)[\s\S]*grant-refresh-failed/,
  );
  assert.match(sources, /if \(await refresh\(\)\)[\s\S]*reportCommittedRefreshFailure/);
  assert.match(connectors, /const refreshed = await refreshConnector[\s\S]*connector_refresh_failed_title/);
  assert.match(connectors, /projectMutationFailure\(error, errorFallback\)/);
  assert.match(project, /mutationsBlockedRef\.current/);
  assert.match(sources, /mutationRunningRef\.current/);
  assert.match(connectors, /accepted:[\s\S]*committed:[\s\S]*not_applied:[\s\S]*unknown:/);
});

function read(relativePath) {
  return readFile(path.join(webRoot, relativePath), "utf8");
}
