---
name: werewolf-6p
title: Six-Player Werewolf
description: Six-player Werewolf rules for a permanent Agent host, with the user choosing to join the randomized player pool or spectate.
scope: room
tags: [room, game, werewolf]
---

# Six-Player Werewolf

This skill layers werewolf game rules on top of the Room communication kernel. The Room system prompt already documents public `@<member>` wake semantics and the built-in `nexus` Room tools; this skill only defines the game contract. The host Agent is always the moderator and never a player.

## Game Setup

Resolve participation before assigning any role:

1. Treat the Room host Agent as the permanent moderator. Exclude it from roles, targets, speeches, votes, deaths, and win-condition counts.
2. Ask the user exactly once whether to join as a player or spectate, using `AskUserQuestion`, unless the opening request already states that choice explicitly.
3. If the user spectates, randomly select six non-host Agent players. The user receives no role and takes no game turn.
4. If the user joins, use `用户玩家` as the stable public name and randomly select five non-host Agent players. Extra non-host Agents are spectators and must not receive role cards or game wakes.
5. Stop before dealing roles if the Room has fewer than six candidate Agent players for spectator mode or fewer than five for user-participation mode. State the exact missing count; never start a partial game.
6. Shuffle the six active players against `2 werewolves + 1 seer + 1 witch + 2 villagers`. A participating user enters the same randomized role pool as every Agent player; never hand-pick or restrict the user's role.

The user can only join as a player and can never replace the host. Use the exact participation choices `参加本局` and `仅旁观` so the result is unambiguous.

After the active roster and roles are fixed, announce only the public roster. Keep the participation answer, role mapping, and inactive candidate list out of the public feed.

## Wake Plumbing

The game has three execution channels:

- **Public feed:** a normal public reply containing a non-code `@<member>` wakes that Agent member. An Agent handoff has exactly one naked `@`; a handoff to the user has none.
- **Agent private channel:** use `nexus.send_message` with `destination=current_room` and `visibility=private` for Agent role cards, hidden collection, and host state. A private message can be record-only or can wake Agent recipients.
- **User private channel:** only the host uses `AskUserQuestion` for the participation choice, the user's role card, and every hidden user decision. It pauses the host turn until the user answers without publishing the prompt or answer to the Room feed. Player Agents never use it to contact the user.

`用户玩家` is not an Agent recipient or wake target. Never place it in `send_message.recipients`, `wake_targets`, or a naked public `@`. When a public chain reaches the user, the preceding Agent stops with no naked `@`; the user continues through a normal Room message and uses a real `@<next Agent>` when an Agent should run next.

A user message counts as a speech, vote, last word, or other game action only while the host state is waiting for that exact user turn. Spectator comments and out-of-turn messages never change game state.

For ordered public chains, the current handoff is the only naked `@`. Future recipients, examples, and format instructions use names without `@`, or code spans such as `` `@NextPlayer` ``. Do not write "please @Sam next" in the host announcement if Sam is not supposed to act now.

When a hidden handback should wake the host and the host's next natural final reply must be public, set `reply_route.next_reply_route.mode = "public"` on the original directed message. The handback stays private; the host's following final reply enters the public feed and any non-code `@` in that reply wakes normally.

Only use `nexus.send_message` with `destination=current_room` and `visibility=public` for an explicit proactive public broadcast from a private/tool-driven turn. This includes the special case where a night transition must become visible before the host blocks on `AskUserQuestion` for the first hidden actor. Otherwise use `next_reply_route={mode:"public"}` and a natural host final reply.

Hidden collection must always name the handback route:

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
    "content": "<question>"
  }
}
```

The Agent player answers only with the requested plain-text final reply. In a private-message turn, the player must not call any Room tool or append `<nexus_room_no_reply/>`; runtime already routes that single final reply. Sending the answer again with `send_message` creates a duplicate handback and can advance the host under the wrong route. If the host's next final reply should be public, include `"next_reply_route": {"mode": "public"}` on the original message; otherwise omit it and the host can continue with private tool calls plus `<nexus_room_no_reply/>`.

### Public Boundary

A host's public final reply is a game artifact, never an execution-status update. Never expose the private actor, collector, target, decision, or who the host is waiting for. In particular, never output status such as "waiting for Jim's kill target".

- On the opening turn, after participation, role cards, private state, and the first hidden action have been queued, the public night announcement is exactly: `游戏开始，第 1 夜，天黑请闭眼。`
- On later night transitions, the exact public announcement is: `进入第 N 夜，天黑请闭眼。`
- If the first hidden actor is an Agent, queue its directed message and use the night announcement as the host's natural public final reply.
- If the first hidden actor is the user, publish the night announcement exactly once with `send_message(destination=current_room, visibility=public)`, then call `AskUserQuestion` in the same host turn. Never reveal why the user is being asked.
- Between private night actions, queue the next action and output exactly `<nexus_room_no_reply/>`.
- When the final hidden action completes in a public-routed host turn, start the daybreak template immediately. Do not prefix it with private status, duplicate the daybreak heading, or insert `---`.

For small-group visibility, send one directed message to all group members and wake only the final collector:

```json
{
  "tool": "nexus.send_message",
  "arguments": {
    "destination": "current_room",
    "visibility": "private",
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
  "tool": "nexus.send_message",
  "arguments": {
    "destination": "current_room",
    "visibility": "private",
    "recipients": ["<host>"],
    "wake_policy": "none",
    "reply_route": {"mode": "none"},
    "content": "<round / alive / dead / roles / potion state>"
  }
}
```

## Players And Roles

- 1 permanent host Agent: assigns roles, collects night actions, announces daybreak, organizes speeches, and runs voting. The host is always the moderator and can never receive a role.
- Exactly 6 active players: 2 werewolves, 1 seer, 1 witch, 2 villagers.
- Deliver each Agent role by record-only directed message: `recipients=["<player>"], wake_policy="none", reply_route={"mode":"none"}`.
- If the user participates, deliver the user's role through `AskUserQuestion` with one acknowledgement option. Include the same role rules an Agent would receive; if the user is a wolf, name the Agent wolf teammate.
- Each wolf's role card names the other wolf. The joint wolf-action message repeats both wolf names.
- The witch starts with one antidote and one poison; each can be used at most once.
- Host keeps minimal private state by sending record-only directed messages to itself, including whether the user participates and which candidate Agents are inactive.

## Win Conditions

- Good side wins when both werewolves are eliminated.
- Werewolves win when all villagers die or all special roles die.
- Check after each daybreak and each voted elimination. The moment a side wins, announce publicly and stop.

## Night Flow

Close these steps in order. At the start of each night, build the pending actor list from living active players: wolves, then the seer if alive, then the witch if alive and at least one potion remains. Skip dead or actionless roles. Agent decisions use a directed message with `wake_policy="immediate"` and `reply_route={mode:"private", recipients:["<host>"], wake_policy:"immediate"}`; user decisions use `AskUserQuestion` with only legal living targets as options. The host always expects one hidden result at a time. An invalid or missing target is re-requested from the same actor and never silently substituted.

If the final pending actor is an Agent, give that Agent message `next_reply_route={mode:"public"}`. If the final pending actor is the user and an Agent acts immediately before it, give the preceding Agent message `next_reply_route={mode:"public"}`; the resumed host turn asks the user and then publishes daybreak naturally. If the user acts earlier, ask first, queue the next Agent action, and output `<nexus_room_no_reply/>`.

### 1. Werewolves

1. If both living wolves are Agents, send one directed message to both with one named collector, as in the small-group example above.
2. If the user and one Agent wolf are alive, ask the Agent wolf privately for one proposal, then show that proposal and all legal targets to the user through `AskUserQuestion`. The user chooses the final kill target. This keeps the user in the same role while using the host as the bridge between human and Agent private channels.
3. If the participating user is the only living wolf, ask the user directly for one living non-wolf target through `AskUserQuestion`.
4. If only one Agent wolf is alive, ask that Agent directly through a directed message.
5. Record exactly one valid target and immediately proceed to the next pending actor or daybreak.

### 2. Seer

1. For an Agent seer, send a directed message asking for one other living player, then return `好人` or `狼人` by record-only directed message.
2. For a user seer, use `AskUserQuestion` to choose one other living player, then use a second private acknowledgement question to reveal `好人` or `狼人`.

### 3. Witch

If the living witch has at least one potion, state tonight's killed player and the remaining potions. Ask an Agent witch through a directed message using the exact format `救:<名字>|不救；毒:<名字>|不毒`. Ask a user witch through `AskUserQuestion`, offering only currently legal save and poison choices. Unavailable potions cannot be selected.

### 4. Daybreak

1. Host resolves deaths from kill / antidote / poison, updates private state, and checks win condition.
2. Host replies publicly with one daybreak announcement, containing:
   - Day number, e.g. "第 N 天天亮。"
   - Death list: names only, never roles or private content. If nobody died: "昨晚平安夜。"
   - Nothing about who attacked, who saved or poisoned, why nobody died, or any other night action. A death or peaceful night does not publicly identify its cause.
   - Surviving roster.
   - Speech order contains every living active player and excludes the host and spectators.
   - An Agent hands to another Agent with `@<next Agent>`. An Agent handing to the user uses no naked `@`, names `用户玩家`, and shows the user's future Agent handoff only as code such as `` `@NextAgent` ``.
   - If the first speaker is an Agent, the final line contains exactly one naked `@`: `首位发言 @<FirstSpeaker>，请发表看法；结束时交给 <NextPlayer>。`
   - If the first speaker is the user, the final line contains no naked `@`: `首位发言是用户玩家，请发表看法；结束时按提示交给下一位。`
3. Host then stops. An Agent mention wakes the first Agent; a user-first announcement waits for the user's normal Room message.

## Day Flow

### 5. Speech Chain

1. Agent-to-Agent: end with `@<NextAgent>`.
2. Agent-to-user: use no naked `@`; end by telling `用户玩家` to speak and show the user's required next handoff in code formatting.
3. User turn: the user posts a normal public Room message and ends with a real `@<NextAgent>`, or `归票完毕 @<host>` when last.
4. If the user omits the handoff and host auto-reply is enabled, the host emits one minimal public handoff to the intended next Agent without rewriting the user's speech.

Keep each public statement between 60 and 120 Chinese characters, excluding the final handoff. State one main suspicion and one reason; do not repeat the phase rules or recap every prior speaker. Do not use private messages during the speech phase.

Public speech is the entire final reply. Never prefix it with private reasoning, hidden role facts, drafts, or a separator such as `---`. A wolf may think privately as a wolf, but the final public reply must only contain what that public persona says.

### 6. Voting

1. Woken by `归票完毕 @<host>`, host replies publicly with one voting announcement:
   - "投票开始。顺序：A -> B -> ...（与发言顺序相同）。"
   - Rules: vote publicly with `"我投 <名字>"` or `"弃票"` and hand off to the next voter.
   - Final voter ends with `投票结束 @<host>`.
   - If the first voter is an Agent, the final line contains exactly one naked `@`: `首位投票 @<FirstVoter>，请按格式投票；结束时交给 <NextVoter>。`
   - If the first voter is the user, the final line contains no naked `@`: `首位投票是用户玩家，请按格式投票；结束时按提示交给下一位。`
2. Each voter uses at most one short reason, then the exact vote and public handoff; keep it under 60 Chinese characters. Apply the same Agent-to-user and user-to-Agent handoff rules as the speech chain.
3. Host tallies from the public feed and replies publicly with the tally only, e.g. "票型：Jim 2 / Lucy 2 / Lily 1 / 弃票 0。Jim 与 Lucy 平票。"
4. Tie-break: run one public PK speech round with `@` chaining, then a fresh public voting round among non-tied voters. Still tied means no elimination today.

### 7. Last Words And End Of Day

1. If an Agent player is eliminated, host replies publicly: "请发表遗言 @<eliminated>，结束用 `遗言完毕 @<host>` 交回给我。"
2. If the user is eliminated, host uses no naked `@` and asks `用户玩家` to post at most 80 Chinese characters, ending with a real `遗言完毕 @<host>`.
3. Whether a player was eliminated or the day ended without elimination, host checks the win condition. If the game continues, it queues the first valid night actor and uses the exact public night-transition reply from Public Boundary.

## Privacy

- **Private:** participation answer, role assignments, wolf night coordination, kill/seer/witch decisions, seer result, host state.
- **Public:** phase announcements, death lists, daytime speeches, vote tallies, last words, win announcement.
- Before the game ends, the host never reveals a role, night decision, private reply, or the cause of a death or peaceful night on the public feed. Announce only the resolved death list.
- Players never reveal hidden role truth, private night actions, private prompts, or "I am secretly a wolf" reasoning in public. Only the host reveals final roles after the game has ended.
- Players do not declare the winner before the host announces the official result.
- A participating user's `AskUserQuestion` prompts and answers follow the same secrecy rules as Agent directed messages. Never quote or summarize them publicly.
