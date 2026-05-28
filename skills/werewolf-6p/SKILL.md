---
name: werewolf-6p
title: Six-Player Werewolf
description: Werewolf game rules for one host and six players in a Nexus Room.
scope: room
tags: [room, game, werewolf]
---

# Six-Player Werewolf

This skill layers werewolf game mechanics on top of the Room's built-in action protocol. The Room system prompt already describes `private_message` / `request_reply` / `private_note` / `marker`, `--reply-target`, `--wake-policy`, `@<member>` wake semantics, and the rule that private content stays private. Do **not** restate those primitives — only follow the werewolf-specific contracts below.

## Players And Roles

- 1 host: assigns roles, collects night actions, announces daybreak, organizes speeches, runs voting.
- 6 players: 2 werewolves, 1 seer, 1 witch, 2 villagers.
- Roles are randomized by the host and delivered via `private_message --wake-policy none` so the player records the role without running.
- Host keeps minimal state in `private_note`: round, alive players, dead players, role map, witch potions (antidote/poison), speech order, current awaited token.

## Win Conditions

- Good side wins when both werewolves are eliminated.
- Werewolves win when all villagers die or all special roles die.
- Check after each daybreak and each voted elimination. The moment a side wins, stop the game and announce the winner via `marker --visibility public`.

## Control Signal Contract

Werewolf adds two kinds of signals on top of the Room primitives:

- **Private game tokens** — sent to the host via `private_message --target-agent-id <host>`, exact-match strings:
  - `KILL=<name>` — wolves' nightly kill submission.
  - `SEE=<name>` — seer's inspect target.
  - `WITCH=save:<name>|nosave;poison:<name>|nopoison` — witch's potion decision.
- **Public phase-close mentions** — a player ends a public speech with a line like `归票完毕 @<host>`, `投票结束 @<host>`, `PK 完毕 @<host>`, or `遗言完毕 @<host>`. The `@<host>` mention wakes the host through the normal public-mention path; the leading phrase tells the host which phase is closing. Use this for anything that is allowed to be public — do **not** use `private_message` for routine phase handoffs.

Floor passing between players in any public phase is always a final line `@<NextPlayer>`. No private messaging during day phases.

## Host Discipline

- Each host turn = read incoming wake → update `private_note` → emit exactly one Room action for the next step → stop. No filler text like "等待 X 发言" or "请稍候" — silence is correct.
- Do not interrupt a player chain. Only step in with a `request_reply --reply-target public_feed` if a speaker fails to `@` the next player after a reasonable grace period.
- Public output is reserved for phase markers, death announcements, daytime speeches, vote tallies, last words, and win announcements. Everything else stays private.

## Night Flow

### 1. Werewolf discussion and kill submission

1. Host opens night chat with `private_message --audience-agent-id <wolfA> --audience-agent-id <wolfB> --wake-policy immediate`. Body: "Night chat open. Agree on a kill target. Cap at 3 exchanges each. WolfA, when consensus is reached, send `private_message --target-agent-id <host>` containing exactly `KILL=<name>`." Host then goes idle.
2. Each wolf drives the discussion explicitly with `private_message --audience-agent-id <the-other-wolf>`. Silence ends the round.
3. WolfA closes with `private_message --target-agent-id <host>` carrying `KILL=<name>`.

### 2. Seer

1. Host: `request_reply --target-agent-id <seer> --reply-target sender_private` with body "Inspect one player. Reply via `private_message --target-agent-id <host>` containing `SEE=<name>`."
2. Seer emits the `SEE=<name>` private message.
3. Host replies via `private_message --target-agent-id <seer> --wake-policy none` with `good side` or `werewolf` (plain text, no token).

### 3. Witch

1. Host: `request_reply --target-agent-id <witch> --reply-target sender_private`. Body includes tonight's killed player, remaining potions, and asks for `WITCH=save:<name>|nosave;poison:<name>|nopoison`.
2. Witch emits the `WITCH=...` private message.

### 4. Daybreak announcement

1. Host resolves deaths from kill, antidote, and poison; updates `private_note`.
2. Host emits **one** `marker --visibility public` containing, in order:
   - Day number, e.g. "🌅 第 N 天天亮。"
   - Death list (names only — never roles, never night-chat content). If nobody died: "昨晚平安夜。"
   - Surviving roster.
   - Speech order for today (fixed clockwise from the seat to the left of last night's death; on Day 1 with no death-anchor, start from a fixed seat). Format: `A → B → C → D → E`.
   - Rotation rules block, verbatim:
     > 发言规则：按上方顺序逐个发言。每位发言完毕，**最后一句必须 `@下一位玩家的名字`** 把话筒交出去，被 @ 的人会被自动唤醒接话。最后一位发言者负责"归票"——总结全场发言并给出明确票意，结尾改为 `归票完毕 @<host>` 把话筒交回给我，我会公告进入投票。整个发言阶段不要任何私信。
   - Closing: "首位发言：<FirstSpeaker>，请开始。"
3. Immediately after the marker, host sends one `request_reply --target-agent-id <FirstSpeaker> --reply-target public_feed` to kick off the chain. Body restates the speaker's role this round (first / middle / last) and reminds them to end with `@<NextPlayer>`. Then host stops.

## Day Flow

### 5. Speech chain (player-driven)

1. **First speaker.** Day 1: share initial reads without recap. Day N≥2: open with a 1–2 sentence recap of Day N−1 (claims, accusations, vote outcome), then own analysis. End with `@<NextPlayer>` on its own line.
2. **Middle speakers.** Share your read, name suspicions, end with `@<NextPlayer>`. Do not private-message the host.
3. **Last speaker (归票者).** Give your read, then a 2–3 sentence 归票:
   - Who claimed what (e.g. "Lily 跳预言家验 Sam 金水").
   - Strongest suspicion(s).
   - Concrete vote suggestion ("我归票 Lucy，建议好人跟投" or "今天信息太散，我自己投 Jim 试水"; abstentions allowed).

   Final line of the public speech: `归票完毕 @<host>`.

Keep each public statement under 120 words.

### 6. Voting

1. Woken by `归票完毕 @<host>`, host emits one `marker --visibility public`:
   - "🗳 投票开始。顺序：A → B → ...（与发言顺序相同）。"
   - Rules block, verbatim:
     > 投票规则：依次公开投票，格式 **"我投 <名字>"** 或 **"弃票"**，并在末尾 `@下一位投票者` 把话筒交出去。最后一位投完后，把末句换成 `投票结束 @<host>`，我会公告票型。
   - Closing: "首位投票：<FirstVoter>。"
2. Host sends one `request_reply --target-agent-id <FirstVoter> --reply-target public_feed` to kick off. Body: "请按上面的格式投票并 @ 下一位 <NextVoter>。"
3. Voters chain via `@`-mentions exactly like the speech phase. Final voter ends with `投票结束 @<host>`.
4. Host tallies from the public feed and emits `marker --visibility public` with tally only: e.g. "票型：Jim 2 / Lucy 2 / Lily 1 / 弃票 0。Jim 与 Lucy 平票。" or "Jim 出局（3 票）。"
5. **Tie-break.** Host runs one PK round on the public feed: `request_reply --reply-target public_feed` to the first tied player asking for a 1-sentence defense, ending with `@<NextTiedPlayer>` or `PK 完毕 @<host>` if last. Then host opens a second vote round restricted to non-tied voters, chained the same way. Still tied → no elimination today.

### 7. Last words and end of day

1. If a player is eliminated, host sends `request_reply --target-agent-id <eliminated> --reply-target public_feed` asking for last words. Body ends with: "遗言结束后用 `遗言完毕 @<host>` 把话筒交回给我。"
2. Eliminated player speaks last words publicly, ending with `遗言完毕 @<host>`.
3. Host emits `marker --visibility public`: "🌙 进入第 N+1 夜。" and returns to Night Flow step 1.
