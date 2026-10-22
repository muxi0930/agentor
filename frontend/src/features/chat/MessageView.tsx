import { Tooltip, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import { useState } from "react";
import { Brain, Copy, Eye, FileText, SquareTerminal } from "lucide-react";
import type { Item } from "../../../bindings/agentor/internal/codex/models";
import type { Attachment } from "../../../bindings/agentor/internal/artifact/models";
import { Message, MessageContent, MessageHeader } from "@/components/ui/message";
import { Bubble, BubbleContent } from "@/components/ui/bubble";
import { Trace, TraceCollapsible, TraceCollapsibleContent, TraceCollapsibleTrigger, TraceDescription, TraceIcon, TraceLine, TraceTitle } from "@/components/ui/trace";
import { Button } from "@/components/ui/button";
import { CodeView } from "./CodeView";
import { MarkdownView } from "./MarkdownView";
import { AttachmentView } from "./AttachmentView";
import { ToolView } from "./ToolView";

export function MessageView({ item, files = [], streaming = false, onError }: { item: Item; files?: Attachment[]; streaming?: boolean; onError: (message: string) => void }) {
  const [source, setSource] = useState(false);
  const [expanded, setExpanded] = useState(false);
  if (item.type === "userMessage" || item.type === "agentMessage") {
    const user = item.type === "userMessage";
    const text = user ? (item.content || []).map(input => input.text || "").filter(Boolean).join("\n") : item.text;
    const display = !expanded && text.length > 16000 ? text.slice(0, 16000) : text;
    return <Message align={user ? "end" : "start"}>
      <MessageHeader className={`w-20 shrink-0 flex-col gap-1 px-0 pt-1 sm:in-data-[side-gutters]:absolute sm:in-data-[side-gutters]:top-0 ${user ? "sm:in-data-[side-gutters]:-right-22" : "sm:in-data-[side-gutters]:-left-22"}`}>
        <span className={user ? "w-6 self-start text-center" : "self-end text-right"}>{user ? "Me" : item.phase === "commentary" ? "进展 · Codex" : "Codex"}</span>
        <div className={`flex flex-col gap-0 opacity-0 group-hover/message:opacity-100 group-focus-within/message:opacity-100 ${user ? "items-start self-start" : "items-end self-end"}`}>
          <Tooltip><TooltipTrigger render={<Button variant="ghost" size="icon-xs" aria-label="复制消息" onClick={() => void navigator.clipboard.writeText(text).catch(error => onError(String(error)))}><Copy aria-hidden="true" /></Button>} /><TooltipContent>复制消息</TooltipContent></Tooltip>
          <Tooltip><TooltipTrigger render={<Button variant="ghost" size="icon-xs" aria-label={source ? "预览" : "原文"} aria-pressed={source} onClick={() => setSource(!source)}>{source ? <FileText aria-hidden="true" /> : <Eye aria-hidden="true" />}</Button>} /><TooltipContent>{source ? "预览" : "原文"}</TooltipContent></Tooltip>
        </div>
      </MessageHeader>
      <MessageContent className="flex-1">
      <Bubble variant={user ? "secondary" : "ghost"} align={user ? "end" : "start"}><BubbleContent className="max-w-full">
        {source ? <CodeView code={text} language="markdown" /> : user ? <p className="whitespace-pre-wrap">{display}</p> : <MarkdownView text={display || "…"} streaming={streaming} onError={onError} />}
        {!expanded && text.length > 16000 && <Button variant="outline" size="sm" className="mt-3" onClick={() => setExpanded(true)}>展开完整消息（{text.length.toLocaleString()} 字符）</Button>}
      </BubbleContent></Bubble>
      {user && files.length > 0 && <AttachmentView files={files} onError={onError} />}
    </MessageContent></Message>;
  }
  if (item.type === "reasoning" || item.type === "commandExecution") {
    const command = item.type === "commandExecution";
    const title = command ? item.command || "命令" : "思考过程";
    const characters = Array.from(title);
    const running = item.status === "inProgress" && (command || streaming);
    const reasoning = (item.summary || []).filter(part => part.trim()).join("\n") || (item.reasoningContent || []).filter(part => part.trim()).join("\n");
    if (!command && !reasoning) return null;
    return <Trace><TraceCollapsible><TraceCollapsibleTrigger><TraceLine status={running ? "running" : undefined}><TraceIcon>{command ? <SquareTerminal /> : <Brain />}</TraceIcon><Tooltip><TooltipTrigger render={<TraceTitle>{command && "执行 "}{characters.length > 30 ? characters.slice(0, 30).join("") + "...." : title}</TraceTitle>} /><TooltipContent>{command ? `执行 ${title}` : title}</TooltipContent></Tooltip>{command && <TraceDescription><span className={item.status === "failed" ? "text-[color-mix(in_oklch,var(--destructive)_50%,var(--background))]" : undefined}>{item.status}</span>{item.durationMs != null ? ` · ${(item.durationMs / 1000).toFixed(2)}s` : ""}</TraceDescription>}</TraceLine></TraceCollapsibleTrigger>
      <TraceCollapsibleContent>{command ? <><CodeView code={item.command || "执行命令"} language="完整命令" /><CodeView code={item.aggregatedOutput || "暂无输出"} language="终端输出" />{item.exitCode != null && <p className="text-xs">退出码 {item.exitCode}</p>}</> : <p className="whitespace-pre-wrap text-sm">{reasoning}</p>}</TraceCollapsibleContent>
    </TraceCollapsible></Trace>;
  }
  return <ToolView item={item} onError={onError} />;
}
