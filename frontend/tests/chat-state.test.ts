import assert from "node:assert/strict";
import test from "node:test";
import { applyNotification, mergeTurn, type ConversationState } from "../src/features/chat/state.ts";
import type { Item, Turn } from "../bindings/agentor/internal/codex/models.ts";

const user = { id: "client-1", clientId: "client-1", type: "userMessage", content: [{ type: "text", text: "Color?" }] } as Item;
const agent = { id: "agent-1", clientId: "", type: "agentMessage", text: "Blue" } as Item;
const turn = (status: string, items: Item[]): Turn => ({ id: "turn-1", status, items, error: null });

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
