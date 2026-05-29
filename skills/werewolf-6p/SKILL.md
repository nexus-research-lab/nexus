---
name: werewolf-6p
title: Six-Player Werewolf
description: Werewolf game rules for one host and six players in a Nexus Room.
scope: room
tags: [room, game, werewolf]
---

# Six-Player Werewolf

This skill layers werewolf game rules on top of the Room communication kernel. The Room system prompt already documents public `@<member>` wake semantics, `nexusctl --json room message publish`, and `nexusctl --json room message send`; this skill only defines the game contract.

## Wake Plumbing

The game has two execution channels:

- **Public feed:** a normal public reply or `room message publish` containing a non-code `@<member>` wakes that member. Public phase control is public text with exactly one naked `@`.
- **Directed message:** use `room message send` for hidden information, hidden collection, and private state. A directed message can be record-only or can wake recipients.

For ordered public chains, the current handoff is the only naked `@`. Future recipients, examples, and format instructions use names without `@`, or code spans such as `` `@NextPlayer` ``. Do not write "please @Sam next" in the host announcement if Sam is not supposed to act now.

When a hidden handback should wake the host and the host's next natural final reply must be public, add `--reply-next-route public` to the original directed message. The handback stays private; the host's following final reply enters the public feed and any non-code `@` in that reply wakes normally.

Only use `room message publish` for an explicit proactive public broadcast. The normal night-to-day transition should use `--reply-next-route public` and a natural host final reply, not publish plus `<nexus_room_no_reply/>`.

Hidden collection must always name the handback route:

```bash
nexusctl --json room message send \
  --recipient-agent-id <player> \
  --wake-policy immediate \
  --reply-route private \
  --reply-recipient-agent-id <host> \
  --reply-wake-policy immediate \
  --content "<question>"
```

The player answers in plain text. Runtime projects that answer back to the host privately and wakes the host immediately. If the host's next final reply should be public, include `--reply-next-route public` on the original message; otherwise omit it and the host can continue with private tool calls plus `<nexus_room_no_reply/>`.

For small-group discussion, send one directed message to all group members, but still assign one member as the final collector:

```bash
nexusctl --json room message send \
  --recipient-agent-id <wolfA> \
  --recipient-agent-id <wolfB> \
  --wake-policy immediate \
  --reply-route private \
  --reply-recipient-agent-id <host> \
  --reply-wake-policy immediate \
  --content "你们是狼人。由 <wolfA> 汇总，最终只回复今晚击杀目标。"
```

Every recipient inherits the same `reply_route`, so the content must name one collector and instruct the others to output `<nexus_room_no_reply/>`. If two recipients both reply, the route fires twice and the host is woken twice.

Never "open a discussion and wait." The platform will not infer that a small group is done. A named player must hand the result back through `reply_route=private(... wake=immediate)`.

Host private state is also a directed message:

```bash
nexusctl --json room message send \
  --recipient-agent-id <host> \
  --wake-policy none \
  --reply-route none \
  --content "<round / alive / dead / roles / potion state>"
```

## Players And Roles

- 1 host: assigns roles, collects night actions, announces daybreak, organizes speeches, runs voting.
- 6 players: 2 werewolves, 1 seer, 1 witch, 2 villagers.
- Host randomizes roles and delivers each role by record-only directed message: `--recipient-agent-id <player> --wake-policy none --reply-route none`.
- Host keeps minimal private state by sending record-only directed messages to itself.

## Win Conditions

- Good side wins when both werewolves are eliminated.
- Werewolves win when all villagers die or all special roles die.
- Check after each daybreak and each voted elimination. The moment a side wins, announce publicly and stop.

## Night Flow

Close these steps in order. Every hidden decision uses a directed message with `--wake-policy immediate --reply-route private --reply-recipient-agent-id <host> --reply-wake-policy immediate`. Only the witch handback that leads directly to daybreak adds `--reply-next-route public`. The host always has exactly one expected private handback at a time, except the wolf step where the message may include both wolves but names one collector.

### 1. Werewolves

1. Host sends one directed message to both wolves, waking both. The reply_route (`private([host], wake=immediate)`) is shared by every recipient, so the content must name exactly one collector and tell the other wolf to stay silent. Content: "天黑了，你们是狼人。由 <wolfA> 汇总并回复今晚击杀目标名字（只回名字）。<wolfB> 只在本轮私下补充意见即可，最终不要回复主持人——请直接输出 `<nexus_room_no_reply/>`，否则主持人会收到两个目标。"
2. Collector wolf (`<wolfA>`) answers a target name in this turn. Do not answer "等队友确认"; that stalls.
3. **Non-collector wolf (`<wolfB>`) must output `<nexus_room_no_reply/>`.** If it answers a target too, the host is woken twice with conflicting kills.
4. The collector's reply wakes the host. Host records the kill and immediately proceeds to Seer.

### 2. Seer

1. Host sends a directed message to the seer: "今晚查验谁？只回名字。" The reply route wakes the host.
2. Host returns the result by record-only directed message to the seer with content `好人` or `狼人`.

### 3. Witch

Host sends a directed message to the witch. Content states tonight's killed player and remaining potions, then asks: "是否用解药救？是否用毒药毒谁？格式：救:<名字>|不救；毒:<名字>|不毒。" The reply route wakes the host and includes `--reply-next-route public`, because the host's next natural final reply is the daybreak announcement.

### 4. Daybreak

1. Host resolves deaths from kill / antidote / poison, updates private state, and checks win condition.
2. Host replies publicly with one daybreak announcement, containing:
   - Day number, e.g. "第 N 天天亮。"
   - Death list: names only, never roles or private content. If nobody died: "昨晚平安夜。"
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

Keep each public statement under 120 words. Do not use private messages during the speech phase.

Public speech is the entire final reply. Never prefix it with private reasoning, hidden role facts, drafts, or a separator such as `---`. A wolf may think privately as a wolf, but the final public reply must only contain what that public persona says.

### 6. Voting

1. Woken by `归票完毕 @<host>`, host replies publicly with one voting announcement:
   - "投票开始。顺序：A -> B -> ...（与发言顺序相同）。"
   - Rules: vote publicly with `"我投 <名字>"` or `"弃票"` and hand off to the next voter.
   - Final voter ends with `投票结束 @<host>`.
   - Final line: "首位投票 @<FirstVoter>，请按格式投票；结束时交给 <NextVoter>。"
   - The voting announcement contains exactly one naked `@`: `@<FirstVoter>`. Do not include a naked `@<NextVoter>` in instructions.
2. Voters chain via public `@`.
3. Host tallies from the public feed and replies publicly with the tally only, e.g. "票型：Jim 2 / Lucy 2 / Lily 1 / 弃票 0。Jim 与 Lucy 平票。"
4. Tie-break: run one public PK speech round with `@` chaining, then a fresh public voting round among non-tied voters. Still tied means no elimination today.

### 7. Last Words And End Of Day

1. If a player is eliminated, host replies publicly: "请发表遗言 @<eliminated>，结束用 `遗言完毕 @<host>` 交回给我。"
2. Eliminated player speaks publicly and ends with `遗言完毕 @<host>`.
3. Host checks win condition, replies publicly "进入第 N+1 夜。", then returns to Night Flow.

## Privacy

- **Private:** role assignments, wolf night chat, kill/seer/witch decisions, seer result, host state.
- **Public:** phase announcements, death lists, daytime speeches, vote tallies, last words, win announcement.
- The host never reveals a role, night decision, or private reply on the public feed.
- Players never reveal hidden role truth, private night actions, private prompts, or "I am secretly a wolf" reasoning in public. Only the host reveals final roles after the game has ended.
- Players do not declare the winner before the host announces the official result.
