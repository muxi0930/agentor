import { useCallback, useEffect, useRef, useState } from "react";
import { Events } from "@wailsio/runtime";
import * as Codex from "../../../bindings/agentor/internal/codex/service";
import * as Data from "../../../bindings/agentor/internal/appdata/service";
import * as Files from "../../../bindings/agentor/internal/artifact/service";
import type { Attachment } from "../../../bindings/agentor/internal/artifact/models";
import type { Settings } from "../../../bindings/agentor/internal/appdata/models";
import type { Conversation, Item, Model, Status, Thread } from "../../../bindings/agentor/internal/codex/models";
import { errorText } from "@/app/errors";
import { applyNotification, inputStatus, mergeItem, mergeTurn, type ConversationState, type Notification } from "./state";

function fromConversation(value: Conversation): ConversationState {
  return { thread: value.thread, turns: [...(value.turns.data || [])].reverse(), cursor: value.turns.nextCursor, activeTurnId: value.activeTurnId, readOnly: value.readOnly };
}

function cacheConversation(old: Record<string, ConversationState>, id: string, value: ConversationState): Record<string, ConversationState> {
  const next = Object.fromEntries(Object.entries(old).filter(([key]) => key !== id));
  next[id] = value;
  for (const key of Object.keys(next)) {
    if (Object.keys(next).length <= 8) break;
    if (key !== id && !next[key].activeTurnId) delete next[key];
  }
  return next;
}

export function useChat(settings: Settings, onSettings: (value: Settings) => void, status?: Status) {
  const [threads, setThreads] = useState<Thread[]>([]);
  const [listCursor, setListCursor] = useState<string | null>(null);
  const [models, setModels] = useState<Model[]>([]);
  const [selected, setSelected] = useState(settings.lastThreadId);
  const [conversations, setConversations] = useState<Record<string, ConversationState>>({});
  const [drafts, setDrafts] = useState<Record<string, string>>({});
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [sending, setSending] = useState<Record<string, boolean>>({});
  const [attachments, setAttachments] = useState<Record<string, Attachment[]>>({});
  const [draftAttachments, setDraftAttachments] = useState<Record<string, string[]>>({});
  const [uploading, setUploading] = useState<Record<string, number>>({});
  const selectionRequest = useRef(0);
  const statusRef = useRef(status);
  const historyHome = useRef("");
  statusRef.current = status;

  const loadAttachments = useCallback(async (threadId: string) => {
    const files = await Files.GetAttachments(threadId);
    setAttachments(old => ({ ...old, [threadId]: files || [] }));
  }, []);

  const refreshThreads = useCallback(async (cursor = "") => {
    const generation = statusRef.current?.generation;
    const result = await Codex.ListThreads(cursor);
    if (generation !== statusRef.current?.generation) return;
    const data = result.data || [];
    setThreads(old => cursor ? [...old, ...data.filter(thread => !old.some(existing => existing.id === thread.id))] : data);
    setListCursor(result.nextCursor);
  }, []);

  const openThread = useCallback(async (threadId: string) => {
    const request = ++selectionRequest.current;
    const generation = statusRef.current?.generation;
    setSelected(threadId); setLoading(true); setError("");
    void loadAttachments(threadId).catch(error => setError(errorText(error)));
    try {
      const value = await Codex.OpenThread(threadId);
      if (request !== selectionRequest.current || generation !== statusRef.current?.generation) return;
      setConversations(old => {
        const loaded = fromConversation(value);
        // 历史加载期间可能已收到新的增量内容
        const live = old[threadId];
        loaded.tokenUsage = value.tokenUsage || live?.tokenUsage;
        loaded.turnDiffs = live?.turnDiffs;
        if (live) for (const turn of live.turns) {
          const saved = loaded.turns.find(saved => saved.id === turn.id);
          if (!saved || (saved.status === "inProgress" && turn.status !== "inProgress")) loaded.turns = mergeTurn(loaded.turns, turn);
          else {
            const items = [...(saved.items || [])];
            for (const item of turn.items || []) {
              const index = items.findIndex(saved => saved.id === item.id || (!!item.clientId && item.clientId === saved.clientId));
              if (item.type === "reasoning") {
                if (index < 0) items.push({ ...item, status: saved.status === "inProgress" ? item.status : "completed" });
                else items[index] = mergeItem(item, items[index]);
              } else if (saved.status === "inProgress" && turn.status === "inProgress") {
                if (index < 0) items.push(item);
                else if ((item.text?.length || 0) > (items[index].text?.length || 0) || (item.aggregatedOutput?.length || 0) > (items[index].aggregatedOutput?.length || 0)) items[index] = item;
              }
            }
            loaded.turns = mergeTurn(loaded.turns, { ...saved, items });
          }
        }
        if (loaded.activeTurnId && loaded.turns.some(turn => turn.id === loaded.activeTurnId && turn.status !== "inProgress")) loaded.activeTurnId = "";
        return cacheConversation(old, threadId, loaded);
      });
      onSettings(await Data.SetLastThread(threadId));
    } catch (error) { if (request === selectionRequest.current) setError(errorText(error)); }
    finally { if (request === selectionRequest.current) setLoading(false); }
  }, [onSettings]);

  useEffect(() => {
    if (statusRef.current?.phase !== "ready") return;
    void refreshThreads().catch(error => setError(errorText(error)));
  }, [settings.readExternalThreads, refreshThreads]);

  useEffect(() => {
    if (status?.phase !== "ready") return;
    let cancelled = false;
    const changedHome = !!historyHome.current && historyHome.current !== status.home;
    historyHome.current = status.home;
    if (changedHome) {
      selectionRequest.current++;
      setSelected(""); setThreads([]); setConversations({}); setError(""); setLoading(false);
      void Data.SetLastThread("").then(onSettings).catch(error => { if (!cancelled) setError(errorText(error)); });
    }
    void refreshThreads().catch(error => { if (!cancelled) setError(errorText(error)); });
    void Codex.GetModels().then(value => { if (!cancelled) setModels(value || []); }).catch(error => { if (!cancelled) setError(errorText(error)); });
    if (selected && !changedHome) void openThread(selected);
    return () => { cancelled = true; };
  }, [status?.generation, status?.phase]);

  useEffect(() => {
    const pending: { generation: number; method: string; params: Notification }[] = [];
    let frame = 0;
    const flush = () => {
      if (frame) cancelAnimationFrame(frame);
      frame = 0;
      const batch = pending.splice(0);
      if (!batch.length) return;
      setConversations(old => {
        let next = old;
        for (const event of batch) {
          const id = event.params.threadId!;
          if (event.generation !== statusRef.current?.generation || statusRef.current.phase !== "ready" || !next[id]) continue;
          next = { ...next, [id]: applyNotification(next[id], event.method, event.params) };
        }
        return next;
      });
    };
    const off = Events.On("codex:event", event => {
      if (event.data.generation !== statusRef.current?.generation) return;
      const method = event.data.method;
      let params: Notification;
      try { params = JSON.parse(event.data.params); } catch { return; }
      if (method === "thread/name/updated") setThreads(old => old.map(thread => thread.id === params.threadId ? { ...thread, name: params.threadName || "" } : thread));
      if (params.threadId && (params.turn || params.item || params.delta !== undefined || params.tokenUsage || params.diff !== undefined || params.error)) {
        pending.push({ generation: event.data.generation, method, params });
        if (params.delta !== undefined) { if (!frame) frame = requestAnimationFrame(flush); } else flush();
      }
      if (["turn/completed", "thread/archived"].includes(method)) void refreshThreads().catch(error => setError(errorText(error)));
    });
    return () => { off(); if (frame) cancelAnimationFrame(frame); };
  }, [refreshThreads]);

  useEffect(() => {
    if (status?.phase !== "error" && status?.phase !== "stopped") return;
    setConversations(old => Object.fromEntries(Object.entries(old).map(([id, conversation]) => [id, {
      ...conversation, activeTurnId: "", turns: conversation.turns.map(turn => turn.status === "inProgress" ? { ...turn, status: "interrupted" } : turn),
    }])));
  }, [status?.phase]);

  async function createThread() {
    setLoading(true); setError("");
    try {
      const thread = await Codex.CreateThread(settings.projectDirectory, settings.model);
      setThreads(old => [thread, ...old.filter(item => item.id !== thread.id)]);
      setConversations(old => cacheConversation(old, thread.id, { thread, turns: [], cursor: null, activeTurnId: "" }));
      setSelected(thread.id);
      onSettings(await Data.SetLastThread(thread.id));
    } catch (error) { setError(errorText(error)); }
    finally { setLoading(false); }
  }

  async function clearSelection() {
    selectionRequest.current++;
    setSelected("");
    onSettings(await Data.SetLastThread(""));
  }

  async function send() {
    const threadId = selected, text = drafts[selected] || "";
    if (inputStatus(loading, conversations[threadId])) return;
    const fileIds = draftAttachments[threadId] || [];
    if ((!text.trim() && !fileIds.length) || sending[threadId] || uploading[threadId]) return;
    setSending(old => ({ ...old, [threadId]: true })); setError("");
    try {
      const messageId = crypto.randomUUID();
      const content = text.trim() ? [{ type: "text", text }] : [];
      const userItem: Item = { id: messageId, clientId: messageId, type: "userMessage", content, text: "", summary: [], command: "", aggregatedOutput: "", status: "completed", phase: "", exitCode: null, reasoningContent: [] };
      const activeTurnId = conversations[threadId]?.activeTurnId;
      if (activeTurnId) {
        await Codex.SteerTurn(threadId, text, messageId, fileIds);
        setConversations(old => {
          const conversation = old[threadId];
          if (!conversation) return old;
          return { ...old, [threadId]: { ...conversation, turns: conversation.turns.map(turn => turn.id === activeTurnId && !turn.items?.some(item => item.clientId === messageId) ? { ...turn, items: [...(turn.items || []), userItem] } : turn) } };
        });
      }
      else {
        const turn = await Codex.StartTurn(threadId, text, settings.model, settings.effort, messageId, fileIds);
        setConversations(old => {
          const conversation = old[threadId];
          if (!conversation) return old;
          const existing = conversation.turns.find(existing => existing.id === turn.id);
          const current = existing || turn;
          const items = current.items?.some(item => item.clientId === messageId) ? current.items : [userItem, ...(current.items || [])];
          return { ...old, [threadId]: { ...conversation, turns: mergeTurn(conversation.turns, { ...current, items }), activeTurnId: !existing && turn.status === "inProgress" ? turn.id : conversation.activeTurnId } };
        });
      }
      setDrafts(old => ({ ...old, [threadId]: old[threadId] === text ? "" : old[threadId] }));
      setDraftAttachments(old => ({ ...old, [threadId]: (old[threadId] || []).filter(id => !fileIds.includes(id)) }));
      void loadAttachments(threadId).catch(error => setError(errorText(error)));
    } catch (error) { setError(errorText(error)); }
    finally { setSending(old => ({ ...old, [threadId]: false })); }
  }

  function rememberFiles(threadId: string, files: Attachment[]) {
    setAttachments(old => ({ ...old, [threadId]: [...(old[threadId] || []), ...files] }));
    setDraftAttachments(old => ({ ...old, [threadId]: [...(old[threadId] || []), ...files.map(file => file.id)] }));
  }

  async function chooseFiles() {
    const id = selected;
    setUploading(old => ({ ...old, [id]: (old[id] || 0) + 1 })); setError("");
    try { rememberFiles(id, await Files.ChooseFiles(id) || []); }
    catch (error) { setError(errorText(error)); }
    finally { setUploading(old => ({ ...old, [id]: Math.max(0, (old[id] || 0) - 1) })); }
  }

  async function pasteImages(files: File[]) {
    const id = selected;
    setUploading(old => ({ ...old, [id]: (old[id] || 0) + 1 })); setError("");
    try {
      for (const file of files) {
        const encoded = await new Promise<string>((resolve, reject) => {
          const reader = new FileReader();
          reader.onload = () => resolve(String(reader.result).split(",")[1]);
          reader.onerror = () => reject(new Error(`无法读取图片 ${file.name}`));
          reader.readAsDataURL(file);
        });
        const item = await Files.SavePastedImage(id, file.name || "pasted-image.png", encoded);
        rememberFiles(id, [item]);
      }
    } catch (error) { setError(errorText(error)); }
    finally { setUploading(old => ({ ...old, [id]: Math.max(0, (old[id] || 0) - 1) })); }
  }

  async function loadEarlier() {
    const conversation = conversations[selected];
    if (!conversation?.cursor) return;
    const id = selected;
    setLoading(true);
    try {
      const page = await Codex.ListTurns(id, conversation.cursor);
      setConversations(old => ({ ...old, [id]: { ...old[id], cursor: page.nextCursor,
        turns: [...(page.data || [])].reverse().filter(turn => !old[id].turns.some(existing => existing.id === turn.id)).concat(old[id].turns) } }));
    } catch (error) { setError(errorText(error)); }
    finally { setLoading(false); }
  }

  return { threads, listCursor, models, selected, conversation: conversations[selected], loading, error, setError,
    attachments: attachments[selected] || [],
    draftFiles: (attachments[selected] || []).filter(file => draftAttachments[selected]?.includes(file.id)),
    removeDraftFile: (id: string) => {const threadId=selected;void Files.DiscardDraftAttachment(id).then(()=>{setDraftAttachments(old=>({...old,[threadId]:(old[threadId]||[]).filter(existing=>existing!==id)}));setAttachments(old=>({...old,[threadId]:(old[threadId]||[]).filter(file=>file.id!==id)}));}).catch(error=>setError(errorText(error)));},
    uploading: (uploading[selected] || 0) > 0, chooseFiles, pasteImages,
    draft: drafts[selected] || "", setDraft: (text: string) => setDrafts(old => ({ ...old, [selected]: text })),
    sending: sending[selected] || false, openThread, createThread, clearSelection, send, loadEarlier, refreshThreads };
}
