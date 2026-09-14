// INPUT: 缺失/显式 ACL 能力与 owner 重置。
// OUTPUT: 能力只在显式开启时暴露，配置更新和重置同步撤回。
// POS: 项目权限前端门禁的响应式回归。
import { act, renderHook } from "@testing-library/react";
import { afterEach, expect, it } from "vitest";
import { applyRuntimeOptions, resetRuntimeOptionsForOwnerChange } from "@/config/runtime-options";
import { useProjectPermissionsEnabled } from "./use-project-permissions-enabled";

afterEach(resetRuntimeOptionsForOwnerChange);
it("缺失能力默认关闭，显式开启后随 owner 重置撤回", () => {
  const { result } = renderHook(useProjectPermissionsEnabled);
  expect(result.current).toBe(false);
  act(() => applyRuntimeOptions({ default_agent_id: "qa", project_permissions_enabled: true }));
  expect(result.current).toBe(true);
  act(() => applyRuntimeOptions({ default_agent_id: "qa" }));
  expect(result.current).toBe(false);
  act(() => applyRuntimeOptions({ default_agent_id: "qa", project_permissions_enabled: true }));
  act(resetRuntimeOptionsForOwnerChange);
  expect(result.current).toBe(false);
});
