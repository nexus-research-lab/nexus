---
name: werewolf-6p
title: Six-Player Werewolf
description: Werewolf game rules for one host and six players in a Nexus Room.
scope: room
tags: [room, game, werewolf]
---

# Six-Player Werewolf

This skill layers werewolf game rules on top of the Room communication kernel. The Room system prompt already documents public `@<member>` wake semantics and the built-in `nexus_room` communication tools; this skill only defines the game contract.

## Wake Plumbing

The game has two execution channels:

- **Public feed:** a normal public reply containing a non-code `@<member>` wakes that member. Public phase control is public text with exactly one naked `@`.
- **Directed message:** use the built-in tool `nexus_room.send_directed_message` for hidden information, hidden collection, and private state. A directed message can be record-only or can wake recipients.

For ordered public chains, the current handoff is the only naked `@`. Future recipients, examples, and format instructions use names without `@`, or code spans such as `` `@NextPlayer` ``. Do not write "please @Sam next" in the host announcement if Sam is not supposed to act now.

When a hidden handback should wake the host and the host's next natural final reply must be public, set `reply_route.next_reply_route.mode = "public"` on the original directed message. The handback stays private; the host's following final reply enters the public feed and any non-code `@` in that reply wakes normally.

Only use `nexus_room.publish_public_message` for an explicit proactive public broadcast from a private/tool-driven turn. The normal night-to-day transition should use `next_reply_route={mode:"public"}` and a natural host final reply, not publish plus `<nexus_room_no_reply/>`.

Hidden collection must always name the handback route:

```json
{
  "tool": "nexus_room.send_directed_message",
  "arguments": {
    "recipients": ["<player>"],
    "wake_policy": "immediate",
    "reply_route": {
      "mode": "private",
      "recipients": ["<host>"],
      "wake_policy": "immediate"
    },
    "content": "<question>"
  }
}
```

The player answers only with the requested plain-text final reply. In a directed-message turn, the player must not call any Room tool or append `<nexus_room_no_reply/>`; runtime already routes that single final reply. Sending the answer again with `send_directed_message` creates a duplicate handback and can advance the host under the wrong route. If the host's next final reply should be public, include `"next_reply_route": {"mode": "public"}` on the original message; otherwise omit it and the host can continue with private tool calls plus `<nexus_room_no_reply/>`.

### Public Boundary

A host's public final reply is a game artifact, never an execution-status update. Never expose the private actor, collector, target, decision, or who the host is waiting for. In particular, never output status such as "waiting for Jim's kill target".

- On the opening turn, after role cards, private state, and the first wolf wake have been sent, the public final reply is exactly: `游戏开始，第 1 夜，天黑请闭眼。`
- On later night transitions, queue the first private night action and reply publicly only: `进入第 N 夜，天黑请闭眼。`
- Between private night actions, queue the next action and output exactly `<nexus_room_no_reply/>`.
- When the final private night action returns through `next_reply_route.mode="public"`, start the daybreak template immediately. Do not prefix it with private status, duplicate the daybreak heading, or insert `---`.

For small-group visibility, send one directed message to all group members and wake only the final collector:

```json
{
  "tool": "nexus_room.send_directed_message",
  "arguments": {
    "recipients": ["<wolfA>", "<wolfB>"],
    "wake_targets": ["<wolfA>"],
    "wake_policy": "immediate",
    "reply_route": {
      "mode": "private",
      "recipients": ["<host>"],
      "wake_policy": "immediate"
    },
    "content": "你们是狼人。由 <wolfA> 汇总，最终只回复今晚击杀目标。"
  }
}
```

Both wolves can see the message, but only `<wolfA>` is activated. This separates private visibility from execution and guarantees one handback route.

Never "open a discussion and wait." The platform will not infer that a small group is done. A named player must hand the result back through `reply_route=private(... wake=immediate)`.

Host private state is also a directed message:

```json
{
  "tool": "nexus_room.send_directed_message",
  "arguments": {
    "recipients": ["<host>"],
    "wake_policy": "none",
    "reply_route": {"mode": "none"},
    "content": "<round / alive / dead / roles / potion state>"
  }
}
```

## Players And Roles

- 1 host: assigns roles, collects night actions, announces daybreak, organizes speeches, runs voting.
- 6 players: 2 werewolves, 1 seer, 1 witch, 2 villagers.
- Host randomizes roles and delivers each role by record-only directed message: `recipients=["<player>"], wake_policy="none", reply_route={"mode":"none"}`.
- Each wolf's role card names the other wolf. The joint wolf-action message repeats both wolf names.
- The witch starts with one antidote and one poison; each can be used at most once.
- Host keeps minimal private state by sending record-only directed messages to itself.

## Win Conditions

- Good side wins when both werewolves are eliminated.
- Werewolves win when all villagers die or all special roles die.
- Check after each daybreak and each voted elimination. The moment a side wins, announce publicly and stop.

## Night Flow

Close these steps in order. At the start of each night, build the pending actor list from living players: wolves, then the seer if alive, then the witch if alive and at least one potion remains. Skip dead or actionless roles. Every hidden decision uses a directed message with `wake_policy="immediate"` and `reply_route={mode:"private", recipients:["<host>"], wake_policy:"immediate"}`. The final pending actor for that night gets `next_reply_route={mode:"public"}`; earlier actors do not. The host always has exactly one expected private handback at a time, except the wolf step where the message may include both wolves but names one collector. An invalid or missing target is re-requested from the same actor and never silently substituted.

### 1. Werewolves

1. Host sends one directed message to both living wolves with `wake_targets=["<wolfA>"]`. Content names `<wolfA>` as collector and asks for one living non-wolf target.
2. Collector wolf (`<wolfA>`) returns only one valid target name as the final reply. Do not call a Room tool or answer "等队友确认"; either creates a duplicate or stalls.
3. `<wolfB>` sees the same private record but is not activated by this message.
4. The collector's reply wakes the host. Host records the kill and immediately proceeds to the next pending actor, or daybreak if this was the final actor.

### 2. Seer

1. Host sends a directed message to the living seer: "今晚查验一名其他存活玩家，只回名字。" The seer returns only that name as the final reply; the reply route wakes the host.
2. Host returns the result by record-only directed message to the seer with content `好人` or `狼人`.

### 3. Witch

If the living witch has at least one potion, host sends a directed message stating tonight's killed player and remaining potions, then asks: "是否用解药救？是否用毒药毒谁？格式：救:<名字>|不救；毒:<名字>|不毒。" The witch returns only that exact decision format as the final reply, without tools or commentary. Unavailable potions cannot be selected.

### 4. Daybreak

1. Host resolves deaths from kill / antidote / poison, updates private state, and checks win condition.
2. Host replies publicly with one daybreak announcement, containing:
   - Day number, e.g. "第 N 天天亮。"
   - Death list: names only, never roles or private content. If nobody died: "昨晚平安夜。"
   - Nothing about who attacked, who saved or poisoned, why nobody died, or any other night action. A death or peaceful night does not publicly identify its cause.
   - Surviving roster.
   - Speech order: `A -> B -> C -> D -> E`.
   - Rules: each speaker ends with `@<next player>`. The last speaker summarizes and ends with `归票完毕 @<host>`.
   - Final line: "首位发言 @<FirstSpeaker>，请发表看法；结束时交给 <NextPlayer>。"
   - The final line contains exactly one naked `@`: `@<FirstSpeaker>`. Do not write a naked `@<NextPlayer>` in the same announcement.
3. The public `@<FirstSpeaker>` wakes the first speaker. Host then stops.

## Day Flow

### 5. Speech Chain

1. First speaker: Day 1 gives initial reads; later days open with a short recap. End with `@<NextPlayer>`.
2. Middle speakers: read + suspicions, end with `@<NextPlayer>`.
3. Last speaker: give a 2-3 sentence summary and vote suggestion. Final line: `归票完毕 @<host>`.

Keep each public statement between 60 and 120 Chinese characters, excluding the final handoff. State one main suspicion and one reason; do not repeat the phase rules or recap every prior speaker. Do not use private messages during the speech phase.

Public speech is the entire final reply. Never prefix it with private reasoning, hidden role facts, drafts, or a separator such as `---`. A wolf may think privately as a wolf, but the final public reply must only contain what that public persona says.

### 6. Voting

1. Woken by `归票完毕 @<host>`, host replies publicly with one voting announcement:
   - "投票开始。顺序：A -> B -> ...（与发言顺序相同）。"
   - Rules: vote publicly with `"我投 <名字>"` or `"弃票"` and hand off to the next voter.
   - Final voter ends with `投票结束 @<host>`.
   - Final line: "首位投票 @<FirstVoter>，请按格式投票；结束时交给 <NextVoter>。"
   - The voting announcement contains exactly one naked `@`: `@<FirstVoter>`. Do not include a naked `@<NextVoter>` in instructions.
2. Each voter uses at most one short reason, then the exact vote and public handoff; keep it under 60 Chinese characters. Voters chain via public `@`.
3. Host tallies from the public feed and replies publicly with the tally only, e.g. "票型：Jim 2 / Lucy 2 / Lily 1 / 弃票 0。Jim 与 Lucy 平票。"
4. Tie-break: run one public PK speech round with `@` chaining, then a fresh public voting round among non-tied voters. Still tied means no elimination today.

### 7. Last Words And End Of Day

1. If a player is eliminated, host replies publicly: "请发表遗言 @<eliminated>，结束用 `遗言完毕 @<host>` 交回给我。"
2. Eliminated player gives at most 80 Chinese characters of last words and ends with `遗言完毕 @<host>`.
3. Whether a player was eliminated or the day ended without elimination, host checks the win condition. If the game continues, it queues the first valid night actor and uses the exact public night-transition reply from Public Boundary.

## Privacy

- **Private:** role assignments, wolf night chat, kill/seer/witch decisions, seer result, host state.
- **Public:** phase announcements, death lists, daytime speeches, vote tallies, last words, win announcement.
- Before the game ends, the host never reveals a role, night decision, private reply, or the cause of a death or peaceful night on the public feed. Announce only the resolved death list.
- Players never reveal hidden role truth, private night actions, private prompts, or "I am secretly a wolf" reasoning in public. Only the host reveals final roles after the game has ended.
- Players do not declare the winner before the host announces the official result.
