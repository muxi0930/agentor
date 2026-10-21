import type { Item, Thread, TokenUsage, Turn, TurnError } from "../../../bindings/agentor/internal/codex/models";

export type ConversationState = {
  thread: Thread; turns: Turn[]; cursor: string | null; activeTurnId: string;
  readOnly?: boolean;
  tokenUsage?: TokenUsage; turnDiffs?: Record<string, string>; transientError?: { message: string; willRetry: boolean; turnId?: string };
};

export function inputStatus(loading: boolean, conversation?: ConversationState): string {
  if (loading) return "正在加载会话，暂时无法输入，请稍候…";
  if (!conversation) return "会话尚未成功读取，暂时无法输入，请重新打开会话。";
  if (conversation.readOnly) return "已在另一个应用中打开";
  if (conversation.activeTurnId === "starting") return "当前任务正在启动，暂时无法追加指令，请稍候…";
  return "";
}
export type Notification = {
  threadId?: string; turnId?: string; itemId?: string; delta?: string;
  turn?: Turn; item?: IncomingItem; thread?: Thread; summaryIndex?: number; contentIndex?: number; threadName?: string;
  tokenUsage?: TokenUsage; diff?: string; error?: TurnError; willRetry?: boolean;
};
type IncomingItem = Omit<Item, "content"> & { content: Item["content"] | string[] };

export function mergeItem(previous: Item | undefined, incoming: IncomingItem): Item {
  const item = { ...previous, ...incoming } as Item;
  if (item.type === "reasoning") {
    item.content = null;
    for (const key of ["summary", "reasoningContent"] as const) {
      const parts = [...(previous?.[key] || [])];
      const incomingParts = key === "reasoningContent" ? incoming.reasoningContent || incoming.content as string[] | null : incoming.summary;
      for (const [index, text] of (incomingParts || []).entries()) {
        if (typeof text === "string" && text.trim() && text.trim().length >= (parts[index]?.trim().length || 0)) parts[index] = text;
      }
      item[key] = parts;
    }
  }
  return item;
}

function mergeItems(previous: Item[], incoming: Item[]): Item[] {
  const items = [...previous];
  for (const item of incoming) {
    const index = items.findIndex(existing => existing.id === item.id || (!!item.clientId && existing.clientId === item.clientId));
    if (index < 0) items.push(mergeItem(undefined, item)); else items[index] = mergeItem(items[index], item);
  }
  return items;
}

export function mergeTurn(turns: Turn[], next: Turn): Turn[] {
  const found = turns.find(turn => turn.id === next.id);
  if (!found) return [...turns, { ...next, items: mergeItems([], next.items || []) }];
  return turns.map(turn => turn.id === next.id
    ? { ...turn, ...next, items: mergeItems(turn.items || [], next.items || []) } : turn);
}

export function applyNotification(state: ConversationState, method: string, params: Notification): ConversationState {
  let next = { ...state, turns: [...state.turns] };
  if (method === "thread/tokenUsage/updated" && params.tokenUsage) next.tokenUsage = params.tokenUsage;
  else if (method === "turn/diff/updated" && params.turnId && params.diff !== undefined) next.turnDiffs = { ...next.turnDiffs, [params.turnId]: params.diff };
  else if (method === "error" && params.error) next.transientError = { message: params.error.message, willRetry: !!params.willRetry, turnId: params.turnId };
  else if (params.turn && method === "turn/started") {
    next.turns = mergeTurn(next.turns, params.turn); next.activeTurnId = params.turn.id;
    next.transientError = undefined;
  } else if (params.turn && method === "turn/completed") {
    next.turns = mergeTurn(next.turns, params.turn);
    if (next.activeTurnId === params.turn.id) next.activeTurnId = "";
    if (next.transientError?.turnId === params.turn.id) next.transientError = undefined;
  } else if (params.turnId && (params.item || params.delta !== undefined)) {
    let turn = next.turns.find(turn => turn.id === params.turnId);
    if (!turn) { turn = { id: params.turnId, status: "inProgress", items: [], error: null }; next.turns.push(turn); }
    const items = [...(turn.items || [])];
    const itemId = params.item?.id || params.itemId;
    const item = items.find(item => item.id === itemId || (!!params.item?.clientId && item.clientId === params.item.clientId));
    if (params.item) {
      const finalItem = mergeItem(item, { ...params.item, status: params.item.status || (method === "item/started" ? "inProgress" : "completed") });
      if (item) items.splice(items.indexOf(item), 1, finalItem); else items.push(finalItem);
    } else if (itemId && params.delta !== undefined) {
      const type = method === "item/commandExecution/outputDelta" ? "commandExecution" : method.startsWith("item/reasoning/") ? "reasoning" : "agentMessage";
      const updated = item ? { ...item } : { id: itemId, clientId: "", type, text: "", summary: [], content: [], command: "", aggregatedOutput: "", status: "inProgress", phase: "", exitCode: null, reasoningContent: [] };
      if (type === "commandExecution") updated.aggregatedOutput = (updated.aggregatedOutput || "") + params.delta;
      else if (type === "reasoning") {
        const key = method === "item/reasoning/textDelta" ? "reasoningContent" : "summary";
        const parts = [...(updated[key] || [])];
        const index = (key === "reasoningContent" ? params.contentIndex : params.summaryIndex) ?? 0;
        parts[index] = (parts[index] || "") + params.delta;
        updated[key] = parts;
      } else updated.text = (updated.text || "") + params.delta;
      if (item) items.splice(items.indexOf(item), 1, updated); else items.push(updated);
    }
    next.turns = next.turns.map(candidate => candidate.id === params.turnId ? { ...candidate, items } : candidate);
  }
  return next;
}
