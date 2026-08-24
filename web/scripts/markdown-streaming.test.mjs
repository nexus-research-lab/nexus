import assert from "node:assert/strict";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import ReactMarkdown from "react-markdown";

import { createServer } from "vite";

const webRoot = fileURLToPath(new URL("..", import.meta.url));
const server = await createServer({
  configFile: false,
  logLevel: "silent",
  resolve: { alias: { "@": path.join(webRoot, "src") } },
  root: webRoot,
  server: { middlewareMode: true },
});

test.after(async () => {
  await server.close();
});

test("并行 Markdown 流共用一个帧提交且空闲后停止调度", async () => {
  const { StreamFrameScheduler } = await server.ssrLoadModule(
    "/src/shared/ui/markdown/streaming/stream-frame-scheduler.ts",
  );
  const frames = [];
  const cancelled = [];
  const scheduler = new StreamFrameScheduler({
    cancel: (frameId) => cancelled.push(frameId),
    request: (callback) => {
      frames.push(callback);
      return frames.length;
    },
  });
  const commits = [];
  const consumptions = [];
  const unsubscribeFirst = scheduler.subscribe((timestamp, grant) => {
    commits.push(`first:${timestamp}`);
    consumptions.push({ grant, timestamp });
    return grant;
  });
  const unsubscribeSecond = scheduler.subscribe((timestamp, grant) => {
    commits.push(`second:${timestamp}`);
    consumptions.push({ grant, timestamp });
    return grant;
  });

  assert.equal(frames.length, 1, "all streams must attach to one RAF");
  frames[0](16);
  assert.deepEqual(
    commits,
    ["first:16"],
    "one display frame must commit at most one busy stream",
  );
  assert.equal(
    consumptions
      .filter((entry) => entry.timestamp === 16)
      .reduce((total, entry) => total + entry.grant, 0),
    4,
    "the default aggregate reveal cap must stay at 4 graphemes",
  );
  assert.equal(frames.length, 2, "active streams schedule one shared next frame");

  frames[1](24);
  assert.deepEqual(
    commits,
    ["first:16"],
    "presentation commits are capped at 30Hz even on a faster display",
  );
  assert.equal(frames.length, 3);
  frames[2](50);
  assert.deepEqual(commits, ["first:16", "second:50"]);
  assert.equal(frames.length, 4);

  unsubscribeFirst();
  unsubscribeSecond();
  assert.deepEqual(
    cancelled,
    [4],
    "the last idle stream must cancel the outstanding shared frame",
  );
});

test("共享帧字符额度固定且按轮转顺序公平分配", async () => {
  const { StreamFrameScheduler } = await server.ssrLoadModule(
    "/src/shared/ui/markdown/streaming/stream-frame-scheduler.ts",
  );
  const frames = [];
  const scheduler = new StreamFrameScheduler({
    cancel: () => undefined,
    request: (callback) => {
      frames.push(callback);
      return frames.length;
    },
  }, 5);
  const consumptions = [];
  const subscribe = (name) => scheduler.subscribe((timestamp, grant) => {
    consumptions.push({ grant, name, timestamp });
    return grant;
  });
  const unsubscribeFirst = subscribe("first");
  const unsubscribeSecond = subscribe("second");
  const unsubscribeThird = subscribe("third");

  frames[0](16);
  frames[1](50);
  frames[2](84);

  for (const timestamp of [16, 50, 84]) {
    assert.equal(
      consumptions
        .filter((entry) => entry.timestamp === timestamp)
        .reduce((total, entry) => total + entry.grant, 0),
      5,
      "parallel streams must share one fixed aggregate reveal cap",
    );
  }
  const totals = Object.fromEntries(
    ["first", "second", "third"].map((name) => [
      name,
      consumptions
        .filter((entry) => entry.name === name)
        .reduce((total, entry) => total + entry.grant, 0),
    ]),
  );
  assert.deepEqual(
    totals,
    { first: 5, second: 5, third: 5 },
    "the extra share must rotate instead of favoring insertion order",
  );

  unsubscribeFirst();
  unsubscribeSecond();
  unsubscribeThird();
});

test("并行流多于帧额度时仍不会饥饿", async () => {
  const { StreamFrameScheduler } = await server.ssrLoadModule(
    "/src/shared/ui/markdown/streaming/stream-frame-scheduler.ts",
  );
  const frames = [];
  const scheduler = new StreamFrameScheduler({
    cancel: () => undefined,
    request: (callback) => {
      frames.push(callback);
      return frames.length;
    },
  }, 2);
  const served = [];
  const unsubscribers = ["a", "b", "c", "d"].map((name) => (
    scheduler.subscribe((_timestamp, grant) => {
      served.push(name);
      return grant;
    })
  ));

  frames[0](16);
  frames[1](50);
  frames[2](84);
  frames[3](118);

  assert.deepEqual(
    new Set(served),
    new Set(["a", "b", "c", "d"]),
    "round-robin must serve every stream within bounded frames",
  );
  for (const unsubscribe of unsubscribers) {
    unsubscribe();
  }
});

test("流退出和加入后轮转仍按订阅者身份连续", async () => {
  const { StreamFrameScheduler } = await server.ssrLoadModule(
    "/src/shared/ui/markdown/streaming/stream-frame-scheduler.ts",
  );
  const frames = [];
  const scheduler = new StreamFrameScheduler({
    cancel: () => undefined,
    request: (callback) => {
      frames.push(callback);
      return frames.length;
    },
  }, 1);
  const served = [];
  const subscribe = (name) => scheduler.subscribe((_timestamp, grant) => {
    served.push(name);
    return grant;
  });
  const unsubscribeA = subscribe("a");
  const unsubscribeB = subscribe("b");
  const unsubscribeC = subscribe("c");

  frames[0](16);
  unsubscribeA();
  frames[1](50);
  const unsubscribeD = subscribe("d");
  frames[2](84);
  frames[3](118);

  assert.deepEqual(
    served,
    ["a", "b", "c", "d"],
    "removing an earlier Set entry must not move the fair cursor",
  );
  unsubscribeB();
  unsubscribeC();
  unsubscribeD();
});

test("流回报未用额度后同帧预算可交给后续流", async () => {
  const { StreamFrameScheduler } = await server.ssrLoadModule(
    "/src/shared/ui/markdown/streaming/stream-frame-scheduler.ts",
  );
  const frames = [];
  const scheduler = new StreamFrameScheduler({
    cancel: () => undefined,
    request: (callback) => {
      frames.push(callback);
      return frames.length;
    },
  }, 5);
  const grants = [];
  const unsubscribeIdle = scheduler.subscribe((_timestamp, grant) => {
    grants.push(["idle", grant]);
    return 0;
  });
  const unsubscribeBusy = scheduler.subscribe((_timestamp, grant) => {
    grants.push(["busy", grant]);
    return grant;
  });

  frames[0](16);

  assert.deepEqual(grants, [["idle", 5], ["busy", 5]]);
  unsubscribeIdle();
  unsubscribeBusy();
});

test("大块 live backlog 平滑追赶且 terminal 在有界时间排空", async () => {
  const { AdaptiveStreamClock } = await server.ssrLoadModule(
    "/src/shared/ui/markdown/streaming/adaptive-stream-clock.ts",
  );
  const clock = new AdaptiveStreamClock(0);
  clock.observeAppend(0, 1_200);

  const liveFrame = clock.resolveFrame({
    backlog: 1_200,
    frameIntervalMs: 34,
    streaming: true,
    timestamp: 1_000,
  });
  assert.equal(liveFrame.phase, "rendering");
  assert.ok(
    liveFrame.cps >= 80 && liveFrame.cps <= 90 && liveFrame.revealCount > 0,
    "a large first snapshot must advance at a visible reading pace",
  );

  const terminalFrame = clock.resolveFrame({
    backlog: 1_200,
    frameIntervalMs: 34,
    streaming: false,
    timestamp: 1_034,
  });
  assert.equal(terminalFrame.phase, "flushing");
  assert.ok(
    terminalFrame.cps >= 110 && terminalFrame.cps <= 120
      && terminalFrame.revealCount > liveFrame.revealCount,
    "terminal drain must speed up gently without one immediate full-height jump",
  );
});

test("全局帧额度不足时流时钟保留未展示字符预算", async () => {
  const { AdaptiveStreamClock } = await server.ssrLoadModule(
    "/src/shared/ui/markdown/streaming/adaptive-stream-clock.ts",
  );
  const clock = new AdaptiveStreamClock(0);
  clock.observeAppend(0, 1_200);

  const cappedFrame = clock.resolveFrame({
    backlog: 1_200,
    frameIntervalMs: 34,
    maxRevealCount: 3,
    streaming: false,
    timestamp: 1_000,
  });
  assert.equal(cappedFrame.revealCount, 3);

  const nextFrame = clock.resolveFrame({
    backlog: 1_197,
    frameIntervalMs: 1,
    maxRevealCount: 100,
    streaming: false,
    timestamp: 1_001,
  });
  assert.equal(
    nextFrame.revealCount,
    1,
    "budget accumulated before the global cap must remain available",
  );
});

test("四条低速流的公平轮转等待不会被截成 50ms", async () => {
  const { AdaptiveStreamClock } = await server.ssrLoadModule(
    "/src/shared/ui/markdown/streaming/adaptive-stream-clock.ts",
  );
  const clocks = Array.from(
    { length: 4 },
    () => new AdaptiveStreamClock(0),
  );
  for (const clock of clocks) {
    clock.observeAppend(0, 10);
  }

  const frames = clocks.map((clock) => clock.resolveFrame({
    backlog: 10,
    frameIntervalMs: 134,
    maxRevealCount: 12,
    streaming: true,
    timestamp: 1_000,
  }));

  assert.deepEqual(frames.map((frame) => frame.cps), [18, 18, 18, 18]);
  assert.deepEqual(
    frames.map((frame) => frame.revealCount),
    [2, 2, 2, 2],
    "four streams at 30Hz must retain their full ~134ms fair wait",
  );
});

test("公平池等待信用不会预付给未来尚未到达的正文", async () => {
  const { AdaptiveStreamClock } = await server.ssrLoadModule(
    "/src/shared/ui/markdown/streaming/adaptive-stream-clock.ts",
  );
  const clock = new AdaptiveStreamClock(0);
  clock.observeAppend(0, 1_200);

  assert.equal(clock.resolveFrame({
    backlog: 1,
    frameIntervalMs: 10_000,
    maxRevealCount: 0,
    streaming: false,
    timestamp: 1_000,
  }).revealCount, 0);
  assert.equal(clock.resolveFrame({
    backlog: 1,
    frameIntervalMs: 1,
    maxRevealCount: 1,
    streaming: false,
    timestamp: 1_001,
  }).revealCount, 1);

  assert.ok(
    clock.resolveFrame({
      backlog: 100,
      frameIntervalMs: 1,
      maxRevealCount: 100,
      streaming: false,
      timestamp: 1_002,
    }).revealCount <= 1,
    "draining the old backlog must consume its retained credit before new text arrives",
  );
});

test("流式展示不拆分 emoji ZWJ 和组合字符", async () => {
  const {
    appendStreamingTextUnits,
    joinStreamingTextPrefix,
    splitStreamingTextUnits,
  } = await server.ssrLoadModule(
    "/src/shared/ui/markdown/streaming/stream-text-units.ts",
  );

  assert.deepEqual(
    splitStreamingTextUnits("中文👨‍👩‍👧‍👦e\u0301👍🏽"),
    ["中", "文", "👨‍👩‍👧‍👦", "e\u0301", "👍🏽"],
  );

  const emojiUnits = splitStreamingTextUnits("👩");
  const emojiAppend = appendStreamingTextUnits(emojiUnits, "\u200d💻");
  assert.deepEqual(emojiUnits, ["👩‍💻"]);
  assert.deepEqual(
    emojiAppend,
    { appendedCount: 0, replacedTrailingUnit: true },
  );

  const combiningUnits = splitStreamingTextUnits("a");
  const combiningAppend = appendStreamingTextUnits(combiningUnits, "\u0301");
  assert.deepEqual(combiningUnits, ["a\u0301"]);
  assert.deepEqual(
    combiningAppend,
    { appendedCount: 0, replacedTrailingUnit: true },
  );

  const largeSuffixUnits = splitStreamingTextUnits("a");
  const largeSuffixAppend = appendStreamingTextUnits(
    largeSuffixUnits,
    `\u0301${"后".repeat(1_000)}`,
  );
  assert.equal(joinStreamingTextPrefix(largeSuffixUnits, 1), "a\u0301");
  assert.equal(largeSuffixUnits.slice(1).join(""), "后".repeat(1_000));
  assert.deepEqual(
    largeSuffixAppend,
    { appendedCount: 1_000, replacedTrailingUnit: true },
    "atomic prefix repair must leave the large suffix in the reveal backlog",
  );
});

test("流式 Markdown 保持空行分隔的相邻有序列表项为一个语义块", async () => {
  const { splitStreamingMarkdownBlocks } = await server.ssrLoadModule(
    "/src/shared/ui/markdown/streaming/markdown-stream-blocks.ts",
  );
  const content = [
    "结果如下：",
    "",
    "1. **第一项**",
    "   摘要一",
    "",
    "2. **第二项**",
    "   摘要二",
    "",
    "3. **第三项**",
    "   摘要三",
  ].join("\n");

  const blocks = splitStreamingMarkdownBlocks(content);

  assert.equal(blocks.length, 2);
  assert.equal(blocks[0].content, "结果如下：\n\n");
  assert.equal(blocks[1].start_offset, "结果如下：\n\n".length);
  assert.match(blocks[1].content, /^1\./);
  assert.match(blocks[1].content, /\n2\./);
  assert.match(blocks[1].content, /\n3\./);
});

test("Markdown 有序列表透传非默认起始序号", async () => {
  const { createMarkdownComponents } = await server.ssrLoadModule(
    "/src/shared/ui/markdown/core/markdown-components.tsx",
  );
  const components = createMarkdownComponents(() => null);
  const html = renderToStaticMarkup(
    React.createElement(
      ReactMarkdown,
      { components },
      "4. 第四项",
    ),
  );

  assert.match(html, /<ol[^>]*start="4"/);
  assert.match(html, />第四项</);
});

test("增量 Markdown 不使用会重排既有行的 pretty wrapping", async () => {
  const { createMarkdownComponents } = await server.ssrLoadModule(
    "/src/shared/ui/markdown/core/markdown-components.tsx",
  );
  const components = createMarkdownComponents(() => null);
  const html = renderToStaticMarkup(
    React.createElement(
      ReactMarkdown,
      { components },
      "一段会继续追加的正文\n\n> 一段会继续追加的引用",
    ),
  );

  assert.doesNotMatch(html, /text-pretty|text-balance/);
  assert.match(html, /wrap-anywhere/);
});
