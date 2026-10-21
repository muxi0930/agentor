import assert from "node:assert/strict";
import test from "node:test";
import { applyNotification, inputStatus, mergeTurn, type ConversationState } from "../src/features/chat/state.ts";

test("occupied history stays readable while sending is blocked until reopened", () => {
  const state = { turns: [{ id: "history", status: "completed", items: [] }], activeTurnId: "", readOnly: true } as unknown as ConversationState;
  assert.equal(inputStatus(false, state), "已在另一个应用中打开");
  const updated = applyNotification(state, "thread/name/updated", {});
  assert.equal(updated.turns[0].id, "history");
  assert.equal(inputStatus(false, updated), "已在另一个应用中打开");
  assert.equal(inputStatus(false, { ...updated, readOnly: false }), "");
});
import type { Item, Turn } from "../bindings/agentor/internal/codex/models.ts";

const user = { id: "client-1", clientId: "client-1", type: "userMessage", content: [{ type: "text", text: "Color?" }] } as Item;
const agent = { id: "agent-1", clientId: "", type: "agentMessage", text: "Blue" } as Item;
const turn = (status: string, items: Item[]): Turn => ({ id: "turn-1", status, items, error: null });

test("会话读取期间替换输入，读取失败保持提示，成功后恢复交互", () => {
  const state = { turns: [], activeTurnId: "" } as unknown as ConversationState;
  assert.match(inputStatus(true, state), /正在加载/);
  assert.match(inputStatus(false), /尚未成功读取/);
  assert.equal(inputStatus(false, state), "");
});

test("启动中的会话暂不可追加，收到启动事件后恢复追加且完成后保持可交互", () => {
  let state = { turns: [], activeTurnId: "starting" } as unknown as ConversationState;
  assert.match(inputStatus(false, state), /正在启动/);
  state = applyNotification(state, "turn/started", { turn: turn("inProgress", []) });
  assert.equal(inputStatus(false, state), "");
  state = applyNotification(state, "turn/completed", { turn: turn("completed", []) });
  assert.equal(inputStatus(false, state), "");
});

test("a partial completed turn preserves the accepted user message and its attachment identity", () => {
  const initial = [turn("inProgress", [user])];
  const completed = mergeTurn(initial, turn("completed", [agent]));
  assert.deepEqual(completed[0].items?.map(item => item.id), ["client-1", "agent-1"]);
  assert.equal(completed[0].items?.[0].clientId, "client-1");
  assert.equal(completed[0].status, "completed");
  assert.equal(mergeTurn(completed, turn("completed", [agent]))[0].items?.length, 2);
});

test("the authoritative user item replaces its local receipt by client message id", () => {
  const authoritative = { ...user, id: "server-user-1" };
  const merged = mergeTurn([turn("inProgress", [user])], turn("completed", [authoritative, agent]));
  assert.deepEqual(merged[0].items?.map(item => item.id), ["server-user-1", "agent-1"]);
});

test("completed item text replaces streaming text without duplicating the user message", () => {
  let state = { turns: [turn("inProgress", [user])], activeTurnId: "turn-1" } as ConversationState;
  state = applyNotification(state, "item/agentMessage/delta", { turnId: "turn-1", itemId: "agent-1", delta: "Bl" });
  state = applyNotification(state, "item/completed", { turnId: "turn-1", item: agent });
  state = applyNotification(state, "turn/completed", { turn: turn("completed", [agent]) });
  assert.equal(state.activeTurnId, "");
  assert.equal(state.turns[0].items?.length, 2);
  assert.equal(state.turns[0].items?.[1].text, "Blue");
});

test("reasoning completion preserves the summary already received from streaming events", () => {
  let state = { turns: [turn("inProgress", [])], activeTurnId: "turn-1" } as ConversationState;
  state = applyNotification(state, "item/reasoning/summaryTextDelta", { turnId: "turn-1", itemId: "reason-1", summaryIndex: 0, delta: "检查现有调用链" });
  const completed = { id: "reason-1", type: "reasoning", summary: [], reasoningContent: [], status: "completed" } as unknown as Item;
  state = applyNotification(state, "item/completed", { turnId: "turn-1", item: completed });
  state = applyNotification(state, "turn/completed", { turn: turn("completed", [completed]) });
  assert.equal(state.turns[0].items?.[0].summary?.join("\n"), "检查现有调用链");
});

test("reasoning content and summaries keep their distinct upstream indexes", () => {
  let state = { turns: [turn("inProgress", [])], activeTurnId: "turn-1" } as ConversationState;
  state = applyNotification(state, "item/reasoning/summaryTextDelta", { turnId: "turn-1", itemId: "reason-1", summaryIndex: 1, delta: "概要" });
  state = applyNotification(state, "item/reasoning/textDelta", { turnId: "turn-1", itemId: "reason-1", contentIndex: 0, delta: "已提供的内容" });
  state = applyNotification(state, "item/reasoning/textDelta", { turnId: "turn-1", itemId: "reason-1", contentIndex: 1, delta: "第二段" });
  state = applyNotification(state, "item/reasoning/textDelta", { turnId: "turn-1", itemId: "reason-1", contentIndex: 1, delta: "内容" });
  const item = state.turns[0].items?.[0];
  assert.equal(item?.summary?.[1], "概要");
  assert.equal(item?.summary?.[0] || "", "");
  assert.deepEqual(item?.reasoningContent, ["已提供的内容", "第二段内容"]);
});

test("raw reasoning completion maps upstream content to the displayed reasoning content", () => {
  const state = applyNotification({ turns: [], activeTurnId: "turn-1" } as unknown as ConversationState, "item/completed", {
    turnId: "turn-1", item: { id: "reason-1", type: "reasoning", summary: [], content: ["上游提供的思考内容"] } as unknown as Item,
  });
  assert.deepEqual(state.turns[0].items?.[0].reasoningContent, ["上游提供的思考内容"]);
});

test("history and live reasoning snapshots preserve the fuller upstream content in either arrival order", () => {
  const live = { id: "reason-1", type: "reasoning", summary: ["已收到的完整概要"], reasoningContent: ["完整内容", "第二段"] } as unknown as Item;
  const history = { ...live, summary: ["已收到"], reasoningContent: [] };
  for (const [previous, incoming] of [[live, history], [history, live]]) {
    const merged = mergeTurn([turn("inProgress", [previous])], turn("inProgress", [incoming]));
    assert.deepEqual(merged[0].items?.[0].summary, live.summary);
    assert.deepEqual(merged[0].items?.[0].reasoningContent, live.reasoningContent);
  }
});

test("sparse reasoning segments survive merging a live snapshot into history", () => {
  let live = { turns: [turn("inProgress", [])], activeTurnId: "turn-1" } as ConversationState;
  live = applyNotification(live, "item/reasoning/summaryTextDelta", { turnId: "turn-1", itemId: "reason-1", summaryIndex: 1, delta: "第二段概要" });
  live = applyNotification(live, "item/reasoning/textDelta", { turnId: "turn-1", itemId: "reason-1", contentIndex: 2, delta: "第三段内容" });
  const history = { id: "reason-1", type: "reasoning", summary: [], reasoningContent: [] } as unknown as Item;
  const merged = mergeTurn([turn("inProgress", [history])], live.turns[0]);
  assert.equal(merged[0].items?.[0].summary?.[1], "第二段概要");
  assert.equal(merged[0].items?.[0].reasoningContent?.[2], "第三段内容");
});
