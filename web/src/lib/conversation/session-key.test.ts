import { expect, it } from "vitest";

import {
  areEquivalentSessionKeys,
  buildRoomSharedSessionKey,
  getSessionKeyIdentity,
} from "./session-key";

it("keeps session identity stable across cached lookups", () => {
  const roomKey = buildRoomSharedSessionKey("conv-1");
  const agentKey = "agent:a1:ws:group:conv-1";

  for (let pass = 0; pass < 2; pass += 1) {
    expect(getSessionKeyIdentity(roomKey)).toBe("room:conv-1");
    expect(getSessionKeyIdentity(agentKey)).toBe(agentKey);
    expect(getSessionKeyIdentity("")).toBeNull();
    expect(getSessionKeyIdentity(null)).toBeNull();
    expect(areEquivalentSessionKeys(roomKey, " " + roomKey)).toBe(true);
    expect(areEquivalentSessionKeys(roomKey, agentKey)).toBe(false);
  }
});
