import {beforeEach, expect, it, vi} from "vitest";
import {getTeamDeliveryStatuses, getTeamRoom} from "./team-api";
const request = vi.hoisted(() => vi.fn());
vi.mock("@/lib/api/core/http", () => ({requestApi: request}));
beforeEach(() => request.mockReset());

it("成员续页沿用原始版本与世代，并保留群主优先排序", async () => {
  const member = (id: string, role: string) => ({member_id: id, member_type: "user", role, state: "active"});
  request.mockResolvedValueOnce({room: {membership_version: 9}, conversation: {stream_epoch: "epoch"}, members: [member("first", "member")], next_member_cursor: "user:first"});
  request.mockResolvedValueOnce({members: [member("owner", "owner")]});
  const result = await getTeamRoom("room");
  const query = new URL(request.mock.calls[1][0], "https://example.test").searchParams;
  expect(Object.fromEntries(query)).toEqual({after: "user:first", membership_version: "9", stream_epoch: "epoch"});
  expect(result.members.map((item) => item.member_id)).toEqual(["owner", "first"]);
  request.mockResolvedValueOnce({room: {membership_version: 9}, conversation: {stream_epoch: "epoch"}, members: [], next_member_cursor: "user:first"});
  request.mockRejectedValueOnce(new Error("membership_version_conflict"));
  await expect(getTeamRoom("room")).rejects.toThrow("membership_version_conflict");
});

it("历史投递查询去重并按 100 条分批，取消信号贯穿每批", async () => {
  request.mockResolvedValue([]);
  const ids = Array.from({length: 201}, (_, i) => `message-${i}`);
  const controller = new AbortController();
  await getTeamDeliveryStatuses("room", [...ids, ids[0]], controller.signal);
  expect(request).toHaveBeenCalledTimes(3);
  expect(request.mock.calls.map(([url]) => new URL(url, "https://example.test").searchParams.getAll("message_id").length)).toEqual([100, 100, 1]);
  for (const [, options] of request.mock.calls) expect(options.signal).toBe(controller.signal);
  request.mockClear();
  await getTeamDeliveryStatuses("room", []);
  expect(request).not.toHaveBeenCalled();
});
