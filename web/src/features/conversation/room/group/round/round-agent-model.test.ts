import { describe, expect, it } from "vitest";

import { buildRoomAgentRoundEntries } from "./round-agent-model";

describe("Room Agent round ordering", () => {
  it("keeps the first execution observation as the display order", () => {
    const entries = buildRoomAgentRoundEntries(
      [],
      [],
      [],
      [
        {
          agent_id: "agent-a",
          agent_round_id: "round-a",
          display_order: 1,
          first_seen_at: 1,
          phase: "active",
          round_id: "root",
          status: "streaming",
        },
        {
          agent_id: "agent-b",
          agent_round_id: "round-b",
          display_order: 2,
          first_seen_at: 2,
          phase: "active",
          round_id: "root",
          status: "streaming",
        },
        {
          agent_id: "agent-a",
          agent_round_id: "round-a",
          display_order: 99,
          first_seen_at: 1,
          phase: "active",
          round_id: "root",
          status: "streaming",
        },
      ],
    );

    expect(entries.map((entry) => entry.agent_id)).toEqual([
      "agent-a",
      "agent-b",
    ]);
  });
});
