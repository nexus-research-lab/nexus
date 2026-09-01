---
name: avalon-8-10p
title: 8–10-Player Avalon
description: Eight-to-ten-player Avalon rules for a permanent Agent host, with the user choosing to join the randomized player pool or spectate.
scope: room
tags: [room, game, avalon]
---

# 8–10-Player Avalon

This skill layers a complete base game of Avalon on top of the Room communication kernel. The Room system prompt already defines public `@<member>` wakes and the built-in `nexus` Room tools; this skill defines only the game contract. The host Agent is always the moderator and never a player.

## Game Setup

Resolve participation and player count before assigning roles:

1. Exclude the host Agent from roles, leadership, teams, votes, and win-condition counts.
2. Ask the user exactly once whether to join as a player or spectate, using `AskUserQuestion`, unless the opening request already states that choice explicitly. Use the exact choices `参加本局` and `仅旁观`.
3. If the opening request explicitly names 8, 9, or 10 players, use that count. Reject any unsupported count instead of silently changing it.
4. Otherwise choose the largest supported count in the order `10, 9, 8` that the Room can fill. A participating user occupies one player seat; a spectating user occupies none.
5. Randomly select the required non-host Agent players. Extra Agents and a spectating user remain spectators and receive no role card or game wake.
6. If the requested count cannot be filled, stop before dealing roles and state the exact number of missing players. If no count was requested and fewer than eight seats can be filled, report how many players are missing for an eight-player game.
7. Use `用户玩家` as the participating user's stable public name. A participating user enters the same randomized role pool as every Agent player and can receive any role, become leader, join a quest, and vote.

Use exactly this role table:

| Players | Good | Evil |
| --- | --- | --- |
| 8 | Merlin, Percival, 3 Loyal Servants | Morgana, Assassin, Minion |
| 9 | Merlin, Percival, 4 Loyal Servants | Mordred, Morgana, Assassin |
| 10 | Merlin, Percival, 4 Loyal Servants | Mordred, Morgana, Oberon, Assassin |

Shuffle roles across all active players, then independently randomize the circular seating order and first leader. Announce only the player count, active roster, seating order, and first leader. Keep the participation answer, role mapping, and inactive candidate list private.

Lake Lady, Lancelot, and Excalibur are optional variants and are not enabled. Do not add them or substitute a different role distribution unless the user explicitly requests a variant before setup.

## Wake Plumbing

The game has three execution channels:

- **Public feed:** a normal public reply containing a non-code `@<member>` wakes that Agent. An Agent handoff has exactly one naked `@`; a handoff to the user has none.
- **Agent private channel:** use `nexus.send_message` with `destination=current_room` and `visibility=private` for Agent role cards, hidden votes, quest cards, assassination choices, and host state.
- **User private channel:** only the host uses `AskUserQuestion` for participation, the user's role card, and every hidden user choice. Player Agents never use it to contact the user.

`用户玩家` is not an Agent recipient or wake target. Never place it in `send_message.recipients`, `wake_targets`, or a naked public `@`. A user continues a public turn by posting a normal Room message and uses a real `@<next Agent>` only when an Agent must run next.

A user message counts as a proposal, speech, or other game action only while host state is waiting for that exact user turn. Spectator comments and out-of-turn messages never change game state.

For every ordered public chain, expose only the current handoff as a naked `@`. Write future names without `@` or inside code spans. Public actions must contain only the player's public persona; never leak role truth or private reasoning.

Collect hidden Agent choices one at a time with an explicit private handback:

```json
{
  "tool": "nexus.send_message",
  "arguments": {
    "destination": "current_room",
    "visibility": "private",
    "recipients": ["<player>"],
    "wake_policy": "immediate",
    "reply_route": {
      "mode": "private",
      "recipients": ["<host>"],
      "wake_policy": "immediate"
    },
    "content": "<legal choices and exact reply format>"
  }
}
```

The Agent player answers with only the requested plain-text final reply. It must not call a Room tool or send the answer twice. Queue the next hidden actor and output `<nexus_room_no_reply/>` until the collection is complete.

If the final hidden actor is an Agent, set `reply_route.next_reply_route.mode = "public"` on that Agent's original directed message. If the final hidden actor is the user and an Agent acts immediately before it, set that preceding Agent message's `next_reply_route` to public; the resumed host turn asks the user and publishes the result naturally. If the collection contains only the user, ask from the current public host turn.

Store minimal authoritative game state in a record-only private message to the host with `wake_policy="none"` and `reply_route.mode="none"`: player count, seating order, roles, current leader, quest number, proposal attempt, approved team, quest results, and rejection count.

## Role Knowledge

Deliver each Agent role as a record-only private message. Deliver a participating user's role through `AskUserQuestion` with one acknowledgement choice. Include only the knowledge that role is entitled to receive:

- Merlin sees every Evil player except Mordred. Merlin therefore sees Oberon when Oberon is present.
- Percival sees Merlin and Morgana as two unordered candidates and is not told which is which.
- Evil players other than Oberon know one another. They do not see Oberon as Evil.
- Oberon learns no Evil teammates, and no teammate learns Oberon's identity.
- Loyal Servants and the Minion receive no additional identities.

Never publish role knowledge before the game ends.

## Board And Win Conditions

All supported player counts use quest teams `3, 4, 4, 5, 5` for quests 1 through 5.

- Quests 1, 2, 3, and 5 fail with at least one `失败` card.
- Quest 4 requires at least two `失败` cards to fail; zero or one failure means the quest succeeds.
- Three failed quests immediately give Evil the win.
- Three successful quests start the assassination phase; Good has not won yet.
- Four consecutive rejected proposals make the fifth leader's finalized team execute the current quest without another team vote. Reset the proposal attempt and rejection count after every quest.

Do not replace the forced fifth proposal with an automatic Evil victory.

## Quest Round

Repeat these phases until one side reaches a terminal condition.

### 1. Preliminary Proposal And Discussion

1. The host publicly states the quest number, required team size, proposal attempt, and current leader.
2. The leader publicly proposes exactly that many distinct active players. The leader may include itself.
3. Run exactly one uninterrupted public speech lap. Start with the player immediately to the leader's right; the leader chooses before the lap whether to speak first or last. Follow the fixed circular seating order and allow no interjections.
4. Agent-to-Agent speeches end with exactly one `@<next Agent>`. Agent-to-user handoffs contain no naked `@`; user-to-Agent handoffs use one real `@<next Agent>`.
5. After the final speech, return to the leader. The leader publicly finalizes a legal team and may change any preliminary member.

The host rejects malformed proposals and asks the same leader to correct them; it never fills, removes, or substitutes a player silently.

### 2. Team Vote

On proposal attempts 1 through 4, every active player votes `同意` or `反对`; abstention is illegal. Collect all votes privately so no player sees an earlier choice, then reveal every named vote together in one public result.

A strict majority approves. A tie or a majority of `反对` rejects. On rejection, increment the proposal attempt, rotate leadership to the next active player in seating order, and repeat the same quest. On attempt 5, complete proposal and discussion normally, then skip the team vote and send the finalized team directly on the quest.

Use `AskUserQuestion` for a participating user's hidden team vote. Use private directed messages for Agent votes. Do not summarize partial vote totals while collecting.

### 3. Quest Cards

Collect one hidden card from every approved team member:

- Good players must submit `成功`.
- Evil players may submit either `成功` or `失败`.
- Non-team players submit nothing.

Use `AskUserQuestion` for a participating user's card and a private directed message for an Agent's card. Validate each choice against the player's role, shuffle their order before publishing only the aggregate number of `成功` and `失败` cards, and never identify who submitted a card.

Apply the quest's failure threshold, record the result, reset the rejection track, and rotate leadership to the next active player. Check the win condition before starting another proposal.

## Assassination

After the third successful quest, privately ask the Assassin alone to name one other active player. An Agent Assassin replies through a private directed message; a user Assassin chooses through `AskUserQuestion`.

- If Assassin identifies Merlin, Evil wins.
- Otherwise Good wins.

Publish the target, outcome, and complete role reveal together. Stop all game wakes after the final result.

## Public Handoffs And Privacy

- Public: roster, seating, leader, proposals, speeches, simultaneous named team-vote reveal, quest-card totals, quest results, score, assassination result, final roles.
- Private: participation answer, role map, role knowledge, team votes before reveal, quest cards, assassination choice before resolution, host state.
- A public transition to an Agent ends with exactly one naked `@<current actor>`. A transition to `用户玩家` has no naked `@` and waits for a normal user message.
- Never publish execution status such as who the host is waiting for. Never expose a partial hidden collection.
- No player is eliminated during Avalon. Every active player remains eligible to lead, speak, join teams, and vote until the game ends.

Alternative rejection tracks, early assassination, Lake Lady, Lancelot, and Excalibur remain disabled unless explicitly selected before roles are dealt. Never improvise a variant mid-game.
