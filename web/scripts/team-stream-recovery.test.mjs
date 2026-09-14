import assert from "node:assert/strict";
import test from "node:test";
import { fileURLToPath } from "node:url";

import { importLeafTypeScriptModule } from "./import-leaf-typescript-module.mjs";

const webRoot = fileURLToPath(new URL("..", import.meta.url));
const bootstrap = {
  conversation: { sync_stream_id: "stream-1", stream_epoch: "epoch-2" },
};

test("Team stream accepts current watermarks and explicit epoch resets", async () => {
  const { parseTeamStreamEvent } = await importLeafTypeScriptModule(
    webRoot,
    "src/features/team/team-stream-event.ts",
  );

  assert.deepEqual(parseTeamStreamEvent({
    type: "stream.updated",
    stream_id: "stream-1",
    stream_epoch: "epoch-2",
    high_water_seq: 9,
  }, bootstrap), {
    type: "stream.updated",
    stream_id: "stream-1",
    stream_epoch: "epoch-2",
    high_water_seq: 9,
  });
  assert.equal(parseTeamStreamEvent({
    type: "stream.updated",
    stream_id: "stream-1",
    stream_epoch: "epoch-1",
    high_water_seq: 9,
  }, bootstrap), null);
  assert.deepEqual(parseTeamStreamEvent({
    type: "stream.reset_required",
    stream_id: "stream-1",
    reason: "full_snapshot_required",
  }, bootstrap), {
    type: "stream.reset_required",
    stream_id: "stream-1",
    reason: "full_snapshot_required",
  });
  assert.equal(parseTeamStreamEvent({
    type: "stream.reset_required",
    stream_id: "stream-2",
    reason: "full_snapshot_required",
  }, bootstrap), null);
});
