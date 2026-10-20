import { useState } from "react";
import { Brain, Copy, Terminal } from "lucide-react";
import type { Item } from "../../../bindings/agentor/internal/codex/models";
import type { Attachment } from "../../../bindings/agentor/internal/artifact/models";
import { Message, MessageContent, MessageHeader } from "@/components/ui/message";
import { Bubble, BubbleContent } from "@/components/ui/bubble";
import { Trace, TraceCollapsible, TraceCollapsibleContent, TraceCollapsibleTrigger, TraceDescription, TraceIcon, TraceLine, TraceTitle } from "@/components/ui/trace";
import { Button } from "@/components/ui/button";
import { CodeView } from "./CodeView";
import { MarkdownView } from "./MarkdownView";
import { AttachmentView } from "./AttachmentView";

export function MessageView({ item, files = [], streaming = false, onError }: { item: Item; files?: Attachment[]; streaming?: boolean; onError: (message: string) => void }) {
  const [source, setSource] = useState(false);
  const [expanded, setExpanded] = useState(false);
  if (item.type === "userMessage" || item.type === "agentMessage") {
    const user = item.type === "userMessage";
    const text = user ? (item.content || []).map(input => input.text || "").filter(Boolean).join("\n") : item.text;
    const display = !expanded && text.length > 16000 ? text.slice(0, 16000) : text;
    return <Message align={user ? "end" : "start"}><MessageContent>
      <MessageHeader className="gap-2">{user ? "您" : "Codex"}{item.phase === "commentary" ? " · 进展" : ""}<Button variant="ghost" size="icon-xs" aria-label="复制消息" onClick={() => void navigator.clipboard.writeText(text).catch(error => onError(String(error)))}><Copy /></Button><Button variant="ghost" size="xs" onClick={() => setSource(!source)}>{source ? "预览" : "原文"}</Button></MessageHeader>
      <Bubble variant={user ? "secondary" : "ghost"} align={user ? "end" : "start"}><BubbleContent className="max-w-full">
        {source ? <CodeView code={text} language="markdown" /> : user ? <p className="whitespace-pre-wrap">{display}</p> : <MarkdownView text={display || "…"} streaming={streaming} onError={onError} />}
        {!expanded && text.length > 16000 && <Button variant="outline" size="sm" className="mt-3" onClick={() => setExpanded(true)}>展开完整消息（{text.length.toLocaleString()} 字符）</Button>}
      </BubbleContent></Bubble>
      {user && files.length > 0 && <AttachmentView files={files} onError={onError} />}
    </MessageContent></Message>;
  }
  if (item.type === "reasoning" || item.type === "commandExecution") {
    const command = item.type === "commandExecution";
    return <Trace><TraceCollapsible><TraceCollapsibleTrigger><TraceLine status={item.status === "inProgress" ? "running" : item.status === "failed" ? "error" : undefined}><TraceIcon>{command ? <Terminal /> : <Brain />}</TraceIcon><TraceTitle>{command ? item.command || "执行命令" : "思考过程"}</TraceTitle>{command && <TraceDescription>{item.status}</TraceDescription>}</TraceLine></TraceCollapsibleTrigger>
      <TraceCollapsibleContent>{command ? <><CodeView code={item.aggregatedOutput || "暂无输出"} language="终端输出" />{item.exitCode != null && <p className="text-xs">退出码 {item.exitCode}</p>}</> : <p className="whitespace-pre-wrap text-sm">{(item.summary?.length ? item.summary : item.reasoningContent || []).join("\n") || "正在思考…"}</p>}</TraceCollapsibleContent>
    </TraceCollapsible></Trace>;
  }
  return <p className="rounded-lg border px-3 py-2 text-xs text-muted-foreground">{item.type} · {item.status || "已记录"}</p>;
}
