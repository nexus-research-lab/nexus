// INPUT: 项目目录读取、创建请求与刷新事件。
// OUTPUT: 读取与写入共用互斥边界，未知写结果仍允许只读恢复。
// POS: Project Admin 请求顺序回归。
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { useProjectAdmin } from "./use-project-admin";

const api = vi.hoisted(() => ({ get: vi.fn(), create: vi.fn(), update: vi.fn() }));
vi.mock("@/lib/api/account/project-api", () => ({ getProjectsApi: api.get, createProjectApi: api.create, updateProjectMemberApi: api.update }));
beforeEach(() => { vi.resetAllMocks(); api.get.mockResolvedValue([]); });
afterEach(cleanup);

it("项目刷新与写入串行，旧目录不能在创建后覆盖新结果", async () => {
  let finishRead!: (projects: unknown[]) => void;
  api.get.mockImplementationOnce(() => new Promise((resolve) => { finishRead = resolve; }));
  const { result } = renderHook(() => useProjectAdmin({ canManageMembers: true }), { wrapper: I18nProvider });
  act(() => result.current.setNewProjectId("research"));
  await act(async () => { await result.current.createProject(); await result.current.refreshProjects(); });
  expect(api.create).not.toHaveBeenCalled();
  expect(api.get).toHaveBeenCalledTimes(1);
  await act(async () => finishRead([]));
  let finishCreate!: (project: unknown) => void;
  api.create.mockImplementationOnce(() => new Promise((resolve) => { finishCreate = resolve; }));
  act(() => { void result.current.createProject(); });
  await act(async () => { await result.current.refreshProjects(); });
  expect(api.get).toHaveBeenCalledTimes(1);
  await act(async () => finishCreate({ project_id: "research", root: "/research", members: {} }));
  expect(result.current.viewModel.projects.map((project) => project.project_id)).toEqual(["research"]);
});

it("未知写入后读取失败不解锁，后续成功刷新才恢复提交", async () => {
  const { result } = renderHook(() => useProjectAdmin({ canManageMembers: true }), { wrapper: I18nProvider });
  await waitFor(() => expect(result.current.viewModel.loading).toBe(false));
  act(() => result.current.setNewProjectId("research"));
  api.create.mockRejectedValueOnce(new Error("offline"));
  await act(async () => { await result.current.createProject(); });
  expect(result.current.viewModel.mutationsBlocked).toBe(true);
  api.get.mockRejectedValueOnce(new Error("offline"));
  await act(async () => { await result.current.refreshProjects(); });
  expect(result.current.viewModel.mutationsBlocked).toBe(true);
  await act(async () => { await result.current.createProject(); });
  expect(api.create).toHaveBeenCalledTimes(1);
  await act(async () => { await result.current.refreshProjects(); });
  expect(result.current.viewModel.mutationsBlocked).toBe(false);
});
