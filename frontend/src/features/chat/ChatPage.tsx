import { createPortal } from "react-dom";
import { useEffect, useMemo, useRef, useState } from "react";
import { Archive, ArrowUp, ChevronRight, Folder, FolderOpen, LoaderCircle, LockKeyhole, Paperclip, Pencil, Pin, Plus, RefreshCw, Square, Terminal } from "lucide-react";
import type { Settings } from "../../../bindings/agentor/internal/appdata/models";
import type { Status, Thread } from "../../../bindings/agentor/internal/codex/models";
import * as Codex from "../../../bindings/agentor/internal/codex/service";
import * as Data from "../../../bindings/agentor/internal/appdata/service";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Field, FieldLabel } from "@/components/ui/field";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { MessageScroller, MessageScrollerButton, MessageScrollerContent, MessageScrollerItem, MessageScrollerProvider, MessageScrollerViewport } from "@/components/ui/message-scroller";
import { MessageView } from "./MessageView";
import { AttachmentView } from "./AttachmentView";
import { DiffView } from "./DiffView";
import { UsageView } from "./UsageView";
import { PreviewContext } from "./HTMLView";
import { PermissionSelect, permissionLabels } from "@/features/accounts/PermissionSelect";
import { WorktreeDialog } from "./WorktreeDialog";
import { useChat } from "./useChat";
import { inputStatus } from "./state";
import { errorText } from "@/app/errors";

export function ChatPage({ settings, onSettings, status, openSettings, sidebarContainer, onActivate, focusThread }: {
  settings: Settings; onSettings: (settings: Settings) => void; status?: Status; openSettings: () => void; sidebarContainer: HTMLElement | null; onActivate: () => void; focusThread?: { threadId: string };
}) {
  const chat = useChat(settings, onSettings, status);
  const openedFocus = useRef<{ threadId: string }>(undefined);
  useEffect(() => {
    if (!focusThread || openedFocus.current === focusThread || status?.phase !== "ready") return;
    openedFocus.current = focusThread;
    void chat.openThread(focusThread.threadId);
  }, [focusThread, status?.phase, chat.openThread]);
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const [visible, setVisible] = useState<Record<string, number>>({});
  const [editing, setEditing] = useState<Thread | null>(null);
  const [name, setName] = useState("");
  const [saving, setSaving] = useState(false);
  const [project, setProject] = useState<string | null>(null);
  const [archiveProject, setArchiveProject] = useState<string | null>(null);
  const list = useRef<HTMLDivElement>(null);
  const [dragging, setDragging] = useState<{ cwd: string; id: string; kind: "project" | "thread" } | null>(null);
  const groups = useMemo(() => {
    const result = new Map<string, Thread[]>();
    for (const cwd of settings.projectOrder || []) result.set(cwd, []);
    if (settings.projectDirectory) result.set(settings.projectDirectory, []);
    for (const thread of chat.threads) result.set(thread.cwd, [...(result.get(thread.cwd) || []), thread]);
    const rank = (ids: string[] | null | undefined, id: string) => { const index = ids?.indexOf(id) ?? -1; return index < 0 ? Number.MAX_SAFE_INTEGER : index; };
    for (const [cwd, threads] of result) threads.sort((a, b) => Number(!!settings.pinnedThreads?.[b.id]) - Number(!!settings.pinnedThreads?.[a.id]) || rank(settings.chatOrder?.[cwd], a.id) - rank(settings.chatOrder?.[cwd], b.id));
    return [...result.entries()].filter(([cwd]) => !settings.hiddenProjects?.includes(cwd)).sort(([a], [b]) => rank(settings.projectOrder, a) - rank(settings.projectOrder, b));
  }, [chat.threads, settings]);
  const model = chat.models.find(model => model.model === settings.model);
  const effectiveModel = model || chat.models.find(model => model.model === chat.conversation?.thread.model) || chat.models.find(model => model.isDefault);
  const supportsImages = !!effectiveModel?.inputModalities?.includes("image");
  const invalidImages = !supportsImages && chat.draftFiles.some(file => file.mediaType.startsWith("image/"));
  const active = !!chat.conversation?.activeTurnId;
  const selectedThread = chat.threads.find(thread => thread.id === chat.selected) || chat.conversation?.thread;
  const title = selectedThread?.name || selectedThread?.preview || "新对话";
  const connected = status?.phase === "ready";
  const projectPermission = settings.projectPermissions?.[selectedThread?.cwd || settings.projectDirectory] || settings.permissionMode;
  const effectivePermission = settings.threadPermissions?.[chat.selected] || projectPermission;
  const unavailable = inputStatus(chat.loading, chat.conversation);

  async function run(action: () => Promise<void>) {
    chat.setError("");
    try { await action(); } catch (error) { chat.setError(errorText(error)); }
  }

  async function importProject() {
    const directory = await Data.ChooseDirectory();
    if (directory) {
      await Data.SetChatOrder("", [...new Set([...(settings.projectOrder || []), directory])]);
      onSettings(await Data.SetProjectDirectory(directory));
      onActivate();
    }
  }

  function reorder(cwd: string, id: string, before: string) {
    const ids = (cwd ? groups.find(([path]) => path === cwd)?.[1].map(thread => thread.id) : groups.map(([path]) => path)) || [];
    const next = ids.filter(value => value !== id);
    next.splice(Math.max(0, next.indexOf(before)), 0, id);
    void run(async () => onSettings(await Data.SetChatOrder(cwd, next)));
  }

  function moveByKey(event: React.KeyboardEvent, cwd: string, id: string) {
    if (!event.altKey || !["ArrowUp", "ArrowDown"].includes(event.key)) return;
    event.preventDefault(); event.stopPropagation();
    const ids = (cwd ? groups.find(([path]) => path === cwd)?.[1].map(thread => thread.id) : groups.map(([path]) => path)) || [];
    const index = ids.indexOf(id), other = event.key === "ArrowUp" ? ids[index - 1] : ids[index + 1];
    if (other && (!cwd || !!settings.pinnedThreads?.[id] === !!settings.pinnedThreads?.[other])) reorder(cwd, event.key === "ArrowUp" ? id : other, event.key === "ArrowUp" ? other : id);
  }

  const sidebar = sidebarContainer && settings.mode === "chat" ? createPortal(connected ? (
    <aside aria-label="会话列表" className="flex min-h-0 flex-1 flex-col">
      <div className="flex items-center gap-2 p-3"><Button className="flex-1" variant="outline" disabled={chat.loading || !settings.projectDirectory} onClick={() => { onActivate(); void chat.createThread(); }}><Plus />新建对话</Button>
        <Button variant="ghost" size="icon" aria-label="导入项目文件夹" title="导入项目文件夹" onClick={() => void run(importProject)}><FolderOpen /></Button><WorktreeDialog settings={settings} onSettings={onSettings} /><Button variant="ghost" size="icon" aria-label="刷新会话" onClick={() => void run(() => chat.refreshThreads())}><RefreshCw /></Button></div>
      {groups.length > 0 && <div className="px-3 pb-3"><Select value={settings.projectDirectory} disabled={chat.loading} onValueChange={value => { if (value) void run(async () => onSettings(await Data.SetProjectDirectory(value))); }} items={groups.map(([cwd]) => ({ value: cwd, label: settings.projectAliases?.[cwd] || cwd }))}><SelectTrigger aria-label="新会话所属项目" className="w-full min-w-0" title={settings.projectDirectory}><SelectValue placeholder="选择项目" /></SelectTrigger><SelectContent>{groups.map(([cwd]) => <SelectItem key={cwd} value={cwd}>{settings.projectAliases?.[cwd] || cwd}</SelectItem>)}</SelectContent></Select></div>}
      {!settings.projectDirectory && <div className="px-3 pb-3"><Button variant="outline" className="w-full" onClick={() => void run(importProject)}><FolderOpen />导入项目文件夹</Button><p className="pt-2 text-xs text-muted-foreground">先导入或选择项目，再新建会话。</p></div>}
      <div ref={list} className="mx-3 mb-3 min-h-0 flex-1 overflow-y-auto rounded-xl border border-sidebar-border px-2 pb-4" onDragOver={event => { if (!dragging || !list.current) return; event.preventDefault(); const box = list.current.getBoundingClientRect(); if (event.clientY < box.top + 45) list.current.scrollBy(0, -24); else if (event.clientY > box.bottom - 45) list.current.scrollBy(0, 24); }}>
        {groups.map(([cwd, threads]) => <Collapsible key={cwd} open={expanded[cwd] !== false} onOpenChange={open => {
          setExpanded(old => ({ ...old, [cwd]: open })); if (!open) setVisible(old => ({ ...old, [cwd]: 6 }));
        }}>
          <div className="mt-2 flex items-center" draggable onDragStart={event => { setDragging({ kind:"project", cwd:"", id:cwd }); event.dataTransfer.setData("text/plain", cwd); }} onDragEnd={() => setDragging(null)} onDrop={event => { event.preventDefault(); if (dragging?.kind === "project" && dragging.id !== cwd) reorder("", dragging.id, cwd); setDragging(null); }} onKeyDown={event => moveByKey(event, "", cwd)}><CollapsibleTrigger className="flex min-w-0 flex-1 items-center gap-2 rounded-md px-2 py-2 text-left text-xs font-medium text-muted-foreground hover:bg-muted" title={`${cwd} · 可拖动排序，或 Alt+方向键`}>
            <ChevronRight className={`size-3 shrink-0 transition-transform ${expanded[cwd] !== false ? "rotate-90" : ""}`} /><Folder className="size-3 shrink-0" /><span className="truncate">{settings.projectAliases?.[cwd] || cwd.split(/[\\/]/).filter(Boolean).slice(-1)[0] || cwd}</span><span className="ml-auto">{threads.length}</span>
          </CollapsibleTrigger><Button variant="ghost" size="icon-xs" aria-label="管理项目分组" onClick={() => { setProject(cwd); setName(settings.projectAliases?.[cwd] || ""); }}><Pencil /></Button></div>
          <CollapsibleContent>
            {threads.slice(0, visible[cwd] || 6).map(thread => <div key={thread.id} draggable onDragStart={event => { setDragging({kind:"thread",cwd,id:thread.id}); event.dataTransfer.setData("text/plain", thread.id); }} onDragEnd={() => setDragging(null)} onDrop={event => { event.preventDefault(); if (dragging?.kind === "thread" && dragging.cwd === cwd && dragging.id !== thread.id && !!settings.pinnedThreads?.[dragging.id] === !!settings.pinnedThreads?.[thread.id]) reorder(cwd, dragging.id, thread.id); setDragging(null); }} onKeyDown={event => moveByKey(event, cwd, thread.id)} className={`group/thread flex items-center gap-0.5 rounded-lg ${chat.selected === thread.id ? "bg-accent" : "hover:bg-muted"}`}>
              <Button variant="ghost" className="min-w-0 flex-1 justify-start px-2 font-normal" onClick={() => { onActivate(); void chat.openThread(thread.id); }} title={thread.name || thread.preview || thread.id}>
                {thread.status.type === "active" && <LoaderCircle className="size-3 shrink-0 animate-spin" />}<span className="truncate">{thread.name || thread.preview || "新对话"}</span>
              </Button>
              <Button variant="ghost" size="icon-xs" aria-label={settings.pinnedThreads?.[thread.id] ? "取消置顶会话" : "置顶会话"} className={`opacity-0 group-hover/thread:opacity-100 group-focus-within/thread:opacity-100 ${settings.pinnedThreads?.[thread.id] ? "text-primary" : ""}`} onClick={() => void run(async () => onSettings(await Data.ToggleThreadPin(thread.id)))}><Pin /></Button>
              <Button variant="ghost" size="icon-xs" aria-label="修改会话标题" className="opacity-0 group-hover/thread:opacity-100 group-focus-within/thread:opacity-100" onClick={() => { setEditing(thread); setName(thread.name || thread.preview || ""); }}><Pencil /></Button>
              <Button variant="ghost" size="icon-xs" aria-label="归档会话" className="mr-1 opacity-0 group-hover/thread:opacity-100 group-focus-within/thread:opacity-100" onClick={() => void run(async () => { await Codex.ArchiveThread(thread.id); if (chat.selected === thread.id) await chat.clearSelection(); await chat.refreshThreads(); })}><Archive /></Button>
            </div>)}
            {threads.length > (visible[cwd] || 6) && <Button variant="ghost" size="sm" className="ml-4 text-xs text-muted-foreground" onClick={() => setVisible(old => ({ ...old, [cwd]: (old[cwd] || 6) + 6 }))}>显示更多</Button>}
          </CollapsibleContent>
        </Collapsible>)}
        {!chat.threads.length && <p className="p-4 text-xs leading-6 text-muted-foreground">{settings.projectDirectory ? "项目已导入，可以新建会话。" : "暂无会话，请先导入项目文件夹。"}</p>}
        {chat.listCursor && <Button variant="ghost" className="mt-3 w-full" onClick={() => void run(() => chat.refreshThreads(chat.listCursor!))}>加载更多会话</Button>}
      </div>
    </aside>
  ) : <p className="p-4 text-sm text-muted-foreground">连接 Codex 后显示会话列表。</p>, sidebarContainer) : null;

  if (!connected) return <>{sidebar}<div className="m-auto w-full max-w-2xl p-10"><Empty className="py-16">
    <EmptyHeader><EmptyMedia variant="icon"><Terminal /></EmptyMedia><EmptyTitle className="text-2xl font-semibold tracking-tight">从一次对话开始</EmptyTitle><EmptyDescription className="mt-2 leading-7">连接 Codex，打开项目。<br />您的对话与工作资料将在这里逐步汇聚。</EmptyDescription></EmptyHeader>
    <EmptyContent><Button onClick={openSettings}>配置 Codex</Button>{status?.phase === "error" && <p className="text-sm text-destructive">{status.message}</p>}</EmptyContent>
  </Empty></div></>;

  return <div className="flex size-full min-h-0 min-w-0">
    {sidebar}

    <section className="flex min-h-0 min-w-0 flex-1 flex-col">
      <div className="flex h-13 shrink-0 items-center gap-2 border-b px-5"><p className="truncate text-sm font-medium">{chat.selected ? title : "对话"}</p>{chat.loading && <LoaderCircle aria-label="加载中" className="size-4 animate-spin text-muted-foreground" />}<span className="ml-auto shrink-0 text-xs text-muted-foreground">{active ? "正在执行" : "就绪"}</span></div>
      {chat.error && <div className="px-5 pt-4"><Alert variant="destructive"><AlertDescription className="flex items-start justify-between gap-3"><span className="break-words">{chat.error}</span><Button size="sm" variant="ghost" onClick={() => chat.setError("")}>关闭</Button></AlertDescription></Alert></div>}
      {chat.conversation?.transientError && <div className="px-5 pt-4"><Alert><AlertDescription>{chat.conversation.transientError.message}{chat.conversation.transientError.willRetry ? " · Codex 正在重试" : ""}</AlertDescription></Alert></div>}
      {chat.selected ? <>
        <div className="min-h-0 flex-1">
          <PreviewContext.Provider value={{ threadId: chat.selected, followup: text => { chat.setDraft(chat.draft ? `${chat.draft}\n\n${text}` : text); document.getElementById("chat-input")?.focus(); } }}><MessageScrollerProvider key={chat.selected}>
            <MessageScroller><MessageScrollerViewport><MessageScrollerContent className="mx-auto max-w-3xl px-6 py-6">
              {chat.conversation?.cursor && <Button variant="ghost" disabled={chat.loading} onClick={() => void chat.loadEarlier()}>加载更早的消息</Button>}
              {chat.conversation?.turns.map(turn => <MessageScrollerItem key={turn.id} className="space-y-5">
                {(turn.items || []).map(item => <MessageView key={item.id} item={item} streaming={turn.status === "inProgress"} files={chat.attachments.filter(file => file.messageId === item.clientId && !!item.clientId)} onError={chat.setError} />)}
                {turn.error && <Alert variant="destructive"><AlertDescription>{turn.error.message}</AlertDescription></Alert>}
                {turn.status === "interrupted" && <p className="text-xs text-muted-foreground">此次执行已停止或中断。</p>}
                {chat.conversation?.turnDiffs?.[turn.id] && !turn.items?.some(item => item.type === "fileChange") && <DiffView diff={chat.conversation.turnDiffs[turn.id]} />}
                {turn.status !== "inProgress" && <p className="text-[11px] text-muted-foreground">耗时 {turn.durationMs != null ? `${(turn.durationMs / 1000).toFixed(1)} 秒` : "未知"}</p>}
              </MessageScrollerItem>)}
              {!chat.conversation?.turns.length && !chat.loading && <Empty className="m-auto"><EmptyHeader><EmptyMedia variant="icon"><Terminal /></EmptyMedia><EmptyTitle>这次，要完成什么？</EmptyTitle><EmptyDescription>输入任务，Codex 将在当前项目中开始工作。</EmptyDescription></EmptyHeader></Empty>}
              {active && <p role="status" className="flex items-center gap-2 text-xs text-muted-foreground"><LoaderCircle className="size-3 animate-spin" />Codex 正在处理…</p>}
            </MessageScrollerContent></MessageScrollerViewport><MessageScrollerButton /></MessageScroller>
          </MessageScrollerProvider></PreviewContext.Provider>
        </div>
        {unavailable ? <div role="status" className="mx-auto w-full max-w-3xl shrink-0 px-5 pb-5 pt-2">{chat.conversation?.readOnly && !chat.loading ? <div className="flex items-center gap-4 rounded-2xl border bg-card p-5"><LockKeyhole aria-hidden="true" className="size-5 shrink-0" /><div className="min-w-0 space-y-1"><p className="text-sm font-medium">已在另一个应用中打开</p><p className="text-sm text-muted-foreground">请先在那边关闭会话，才能在这里继续。</p></div></div> : <Alert><AlertDescription>{unavailable}</AlertDescription></Alert>}</div> : <form className="mx-auto w-full max-w-3xl shrink-0 px-5 pb-5 pt-2" onSubmit={event => { event.preventDefault(); if (!invalidImages) void chat.send(); }}>
          <div className="rounded-2xl border bg-card p-3 shadow-sm focus-within:ring-2 focus-within:ring-ring/30">
            {chat.draftFiles.length > 0 && <div className="mb-3"><AttachmentView files={chat.draftFiles} onRemove={chat.removeDraftFile} onError={chat.setError} /></div>}
            <label htmlFor="chat-input" className="sr-only">{active ? "追加指令" : "发送消息"}</label>
            <Textarea id="chat-input" className="max-h-52 min-h-20 resize-none border-0 bg-transparent p-1 shadow-none focus-visible:ring-0" style={{ fieldSizing: "content" }} value={chat.draft} placeholder={active ? "追加指令，或等待当前任务完成…" : "描述您的任务…"} onChange={event => chat.setDraft(event.target.value)} onPaste={event => {
              const images = [...event.clipboardData.files].filter(file => file.type.startsWith("image/"));
              if (!images.length) return;
              event.preventDefault();
              if (!supportsImages) { chat.setError("请先选择支持图片输入的模型"); return; }
              void chat.pasteImages(images);
            }} onKeyDown={event => {
              // WebKit 可能先结束组合输入再触发确认回车，此时仍以 229 标记输入法按键
              if (event.nativeEvent.isComposing || event.nativeEvent.keyCode === 229) return;
              if (event.key === "Enter" && !event.shiftKey) { event.preventDefault(); if (!invalidImages) void chat.send(); }
            }} />
            {invalidImages && <p className="text-xs text-destructive">当前模型不支持图片，请选择支持图片的模型后再发送。</p>}
            <div className="mt-2 flex flex-wrap items-center gap-2">
              <Button type="button" variant="ghost" size="icon-sm" aria-label="添加图片或文件" disabled={chat.uploading} onClick={() => void chat.chooseFiles()}>{chat.uploading ? <LoaderCircle className="animate-spin" /> : <Paperclip />}</Button>
              <Select value={settings.model} onValueChange={selectedValue => { if (selectedValue === null) return; void run(async () => {
                const selected = chat.models.find(model => model.model === selectedValue);
                onSettings(await Data.SetModelSelection(selectedValue, selected?.defaultReasoningEffort || ""));
              }); }} items={[{ value: "", label: settings.reuseCLI?"CLI 默认模型":"账号默认模型" }, ...(chat.models.map(model => ({ value: model.model, label: model.displayName })))]}><SelectTrigger aria-label="选择模型" size="sm" className="max-w-40 min-w-0 gap-1 border-0 bg-transparent px-2 text-xs shadow-none [&_svg]:size-3"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="">{settings.reuseCLI?"CLI 默认模型":"账号默认模型"}</SelectItem>{chat.models.map(model => <SelectItem key={model.id} value={model.model}>{model.displayName}</SelectItem>)}</SelectContent></Select>
              {model && !!model.supportedReasoningEfforts?.length && <Select value={settings.effort || model.defaultReasoningEffort} onValueChange={selectedValue => { if (selectedValue === null) return; void run(async () => onSettings(await Data.SetModelSelection(settings.model, selectedValue))); }} items={(model.supportedReasoningEfforts || []).map(option => ({ value: option.reasoningEffort, label: option.reasoningEffort }))}><SelectTrigger aria-label="思考强度" size="sm" className="w-20 min-w-0 gap-1 border-0 bg-transparent px-2 text-xs shadow-none [&_svg]:size-3"><SelectValue /></SelectTrigger><SelectContent>
                {(model.supportedReasoningEfforts || []).map(option => <SelectItem key={option.reasoningEffort} value={option.reasoningEffort}>{option.reasoningEffort}</SelectItem>)}
              </SelectContent></Select>}
              <div className="max-w-48"><PermissionSelect className="max-w-44 min-w-0 gap-1 border-0 px-2 text-xs [&_svg]:size-3" size="sm" label="当前会话权限" disabled={active || chat.sending} value={settings.threadPermissions?.[chat.selected] || ""} inherit={permissionLabels[projectPermission]} onChange={mode => void run(async () => onSettings(await Data.SetPermission("thread", chat.selected, mode)))} /></div>
              <div className="ml-auto flex items-center gap-2">{active && <Button variant="outline" type="button" aria-label="停止当前任务" onClick={() => void run(() => Codex.StopTurn(chat.selected))}><Square className="size-3" />停止</Button>}
                <Button type="submit" size="icon" aria-label={active ? "追加指令" : "发送消息"} disabled={(!chat.draft.trim() && !chat.draftFiles.length) || chat.sending || chat.loading || chat.uploading || invalidImages}>{chat.sending ? <LoaderCircle className="animate-spin" /> : <ArrowUp />}</Button></div>
            </div>
          </div><div className="mt-2 flex flex-wrap items-center justify-center gap-x-3 gap-y-1 text-[11px] text-muted-foreground"><span>Enter 发送 · Shift+Enter 换行 · 执行权限：{permissionLabels[effectivePermission]}</span><UsageView usage={chat.conversation?.tokenUsage} />{chat.conversation?.turns.some(turn => turn.status === "inProgress" && turn.items?.some(item => item.type === "contextCompaction" && item.status === "inProgress")) && <span role="status">正在压缩上下文…</span>}</div>
        </form>}
      </> : <Empty className="m-auto"><EmptyHeader><EmptyMedia variant="icon"><Terminal /></EmptyMedia><EmptyTitle>开始一次新的对话</EmptyTitle><EmptyDescription>{settings.projectDirectory ? "在所选项目中新建会话，也可以从左侧打开历史。" : "先导入项目文件夹，再新建会话。导入仅登记路径，不复制或移动文件。"}</EmptyDescription></EmptyHeader><EmptyContent>{settings.projectDirectory ? <Button disabled={chat.loading} onClick={() => void chat.createThread()}><Plus />新建对话</Button> : <Button onClick={() => void run(importProject)}><FolderOpen />导入项目文件夹</Button>}</EmptyContent></Empty>}
    </section>
    <Dialog open={!!editing} onOpenChange={open => { if (!open) setEditing(null); }}><DialogContent>
      <DialogHeader><DialogTitle>修改会话标题</DialogTitle><DialogDescription>标题会保存到 Codex 会话。</DialogDescription></DialogHeader>
      <form onSubmit={event => { event.preventDefault(); if (!editing) return; setSaving(true); void run(async () => {
        await Codex.RenameThread(editing.id, name); setEditing(null); await chat.refreshThreads();
      }).finally(() => setSaving(false)); }}><Field><FieldLabel htmlFor="thread-name">标题</FieldLabel><Input id="thread-name" value={name} onChange={event => setName(event.target.value)} /></Field>
        <DialogFooter className="mt-5"><Button variant="outline" type="button" onClick={() => setEditing(null)}>取消</Button><Button type="submit" disabled={saving || !name.trim()}>保存</Button></DialogFooter>
      </form>
    </DialogContent></Dialog>
    <Dialog open={project !== null} onOpenChange={open => { if (!open) setProject(null); }}><DialogContent>
      <DialogHeader><DialogTitle>管理项目分组</DialogTitle><DialogDescription className="break-all">{project}</DialogDescription></DialogHeader>
      {project && <Field><FieldLabel>项目执行权限</FieldLabel><PermissionSelect value={settings.projectPermissions?.[project] || ""} inherit={permissionLabels[settings.permissionMode]} onChange={mode => void run(async () => onSettings(await Data.SetPermission("project", project, mode)))} /></Field>}
      <form onSubmit={event => { event.preventDefault(); if (!project) return; setSaving(true); void run(async () => {
        onSettings(await Data.SetProjectAlias(project, name.trim())); setProject(null);
      }).finally(() => setSaving(false)); }}><Field><FieldLabel htmlFor="project-alias">展示名</FieldLabel><Input id="project-alias" value={name} onChange={event => setName(event.target.value)} placeholder="留空使用目录名称" /></Field>
        <DialogFooter className="mt-5"><Button type="submit" disabled={saving}>保存展示名</Button></DialogFooter>
      </form>
      <div className="flex flex-wrap gap-2 border-t pt-4"><Button variant="outline" onClick={() => void run(async () => {
        if (project) onSettings(await Data.HideProject(project)); setProject(null);
      })}>从侧栏移除分组</Button><Button variant="outline" onClick={() => { setArchiveProject(project); setProject(null); }}>归档项目下的会话</Button></div>
      <p className="text-xs text-muted-foreground">移除分组后，可重新选择项目目录恢复显示。</p>
    </DialogContent></Dialog>
    <AlertDialog open={archiveProject !== null} onOpenChange={open => { if (!open) setArchiveProject(null); }}><AlertDialogContent>
      <AlertDialogHeader><AlertDialogTitle>归档此项目下的全部会话？</AlertDialogTitle><AlertDialogDescription className="break-all">此操作会归档 {archiveProject} 下属于 agentor 的会话，项目文件将保留。</AlertDialogDescription></AlertDialogHeader>
      <AlertDialogFooter><AlertDialogCancel>取消</AlertDialogCancel><AlertDialogAction onClick={() => void run(async () => {
        if (!archiveProject) return;
        await Codex.ArchiveProject(archiveProject);
        if (chat.conversation?.thread.cwd === archiveProject) await chat.clearSelection();
        await chat.refreshThreads(); setArchiveProject(null);
      })}>确认归档</AlertDialogAction></AlertDialogFooter>
    </AlertDialogContent></AlertDialog>
  </div>;
}
