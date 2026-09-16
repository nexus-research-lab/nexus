// INPUT: 授权响应未知、持久待确认状态和后续精确回执。
// OUTPUT: 原意图锁定、撤销恢复和读取失败时的禁用行为。
// POS: 本机授权 UI 的事务边界回归。
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { MemoryRouter } from "react-router-dom";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import type { TeamNodeView } from "@/lib/api/conversation/team-node-api";
import { TeamNodeDialog } from "./team-node-dialog";

const api = vi.hoisted(() => ({ get: vi.fn(), authorize: vi.fn(), revoke: vi.fn() }));
vi.mock("@/lib/api/conversation/team-node-api", () => ({ getTeamNode: api.get, authorizeTeamNode: api.authorize, revokeTeamNode: api.revoke }));

it("freezes unknown grants, retries the stored intent and reconciles a lost revocation response", async () => {
  const user = userEvent.setup();
  const initial: TeamNodeView = {state: "disconnected", agent_ids: [], candidates: [{id: "agent", name: "Amy"}], execution_available: false};
  api.get.mockResolvedValue(initial);
  api.authorize.mockRejectedValue(new Error("response lost"));
  render(<I18N_CONTEXT.Provider value={{locale: "en", setLocale: vi.fn(), t: (key) => key}}><TeamNodeDialog onClose={vi.fn()} /></I18N_CONTEXT.Provider>);
  await user.click(await screen.findByRole("checkbox", {name: "Amy"}));
  const pending: TeamNodeView = {...initial, state: "pending", name: "Persisted device", agent_ids: ["agent"]};
  api.get.mockResolvedValue(pending);
  await user.click(screen.getByRole("button", {name: "team.node_authorize"}));
  await waitFor(() => expect(screen.getByRole("textbox", {name: "team.node_name"})).toHaveProperty("disabled", true));
  expect(screen.getByRole("textbox", {name: "team.node_name"})).toHaveProperty("value", "Persisted device");
  expect(screen.getByRole("checkbox", {name: "Amy"})).toHaveProperty("disabled", true);
  expect(api.authorize).toHaveBeenCalledExactlyOnceWith("Nexus", ["agent"]);
  expect(screen.getByText("team.node_runtime_pending")).toBeTruthy();

  api.authorize.mockResolvedValue({});
  api.get.mockResolvedValue({...pending, state: "authorized"});
  await user.click(screen.getByRole("button", {name: "team.node_retry"}));
  await waitFor(() => expect(api.authorize).toHaveBeenLastCalledWith("Persisted device", ["agent"]));
  const revoke = await screen.findByRole("button", {name: "team.node_revoke"});
  api.revoke.mockRejectedValue(new Error("response lost"));
  api.get.mockResolvedValue({...pending, state: "revoked"});
  await user.click(revoke);
  await screen.findByText("team.node_revoked");
  expect(api.revoke).toHaveBeenCalledTimes(1);

  api.get.mockRejectedValue(new Error("offline"));
  await user.click(screen.getByRole("button", {name: "team.node_refresh"}));
  await waitFor(() => expect(screen.getByRole("button", {name: "team.node_authorize"})).toHaveProperty("disabled", true));
  expect(screen.getByRole("alert")).toBeTruthy();
});

it("requires an explicit execution opt-in and links approvals to the online Thread", async () => {
  const user = userEvent.setup();
  const initial: TeamNodeView = {state: "authorized", name: "Laptop", agent_ids: ["agent"], candidates: [{id: "agent", name: "Amy"}], execution_available: true, execution_enabled: false, jobs: []};
  api.get.mockResolvedValue(initial);
  api.authorize.mockReset().mockResolvedValue({});
  const close = vi.fn();
  render(<MemoryRouter><I18N_CONTEXT.Provider value={{locale: "en", setLocale: vi.fn(), t: (key) => key}}><TeamNodeDialog onClose={close} /></I18N_CONTEXT.Provider></MemoryRouter>);
  const enable = await screen.findByRole("button", {name: "team.node_enable_execution"});
  expect(api.authorize).not.toHaveBeenCalled();
  api.get.mockResolvedValue({...initial, execution_enabled: true, jobs: [{id: "job", agent_id: "agent", state: "running", room_id: "local-room", conversation_id: "local-conversation", source_room_id: "online-room", round_id: "round"}]});
  await user.click(enable);
  await screen.findByText("team.node_execution_enabled");
  expect(api.authorize).toHaveBeenCalledExactlyOnceWith("Laptop", ["agent"], true);
  expect(screen.queryByRole("button", {name: "team.node_enable_execution"})).toBeNull();
  const link = screen.getByRole("link", {name: "team.node_open_execution"});
  expect(link.getAttribute("href")).toBe("/team?room_id=online-room&thread=job");
  await user.click(link);
  expect(close).toHaveBeenCalledTimes(1);
});
