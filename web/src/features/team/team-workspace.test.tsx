// INPUT: 群共享目录与未确认上传。
// OUTPUT: 无 Agent 也能使用文件目录，重试不更换文件，离开页面中止请求。
// POS: 共享文件交互回归，不访问真实文件服务。
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { TeamWorkspace } from "./team-workspace";

const api = vi.hoisted(() => ({list: vi.fn(), upload: vi.fn(), download: vi.fn()}));
vi.mock("@/lib/api/conversation/team-files-api", () => ({listTeamFiles: api.list, uploadTeamFile: api.upload, downloadTeamFile: api.download}));
vi.mock("@/shared/i18n/i18n-context", () => ({useI18n: () => ({t: (key: string) => key})}));

it("keeps an unconfirmed file retryable through refresh and aborts on unmount", async () => {
  api.list.mockResolvedValue([]);
  api.upload.mockRejectedValueOnce(new Error("unknown")).mockImplementation(() => new Promise(() => {}));
  const view = render(<TeamWorkspace roomId="group" />);
  await screen.findByText("room.no_files");
  const file = new File(["shared"], "report.txt");
  fireEvent.change(screen.getByLabelText("room.workspace_action_upload", {selector: "input"}), {target:{files:[file]}});
  await screen.findByText("team.files_upload_error");
  fireEvent(window, new Event("focus"));
  await waitFor(() => expect(api.list).toHaveBeenCalledTimes(2));
  expect(screen.getByText("team.files_upload_error")).toBeTruthy();
  fireEvent.click(screen.getByRole("button", {name:"state.retry"}));
  expect(api.upload.mock.calls.map((call) => call.slice(0,2))).toEqual([["group",file],["group",file]]);
  const signal = api.upload.mock.lastCall![2] as AbortSignal;
  view.unmount(); expect(signal.aborted).toBe(true);
});
