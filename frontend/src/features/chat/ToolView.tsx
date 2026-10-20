import { useState } from "react";
import { FileDiff, Image, Minimize2, Search, Wrench } from "lucide-react";
import type { Item } from "../../../bindings/agentor/internal/codex/models";
import * as Data from "../../../bindings/agentor/internal/appdata/service";
import { Trace, TraceCollapsible, TraceCollapsibleContent, TraceCollapsibleTrigger, TraceDescription, TraceIcon, TraceLine, TraceTitle } from "@/components/ui/trace";
import { Button } from "@/components/ui/button";
import { CodeView } from "./CodeView";
import { DiffView } from "./DiffView";

export function ToolView({ item, onError }: { item: Item; onError: (message: string) => void }) {
  const [open, setOpen] = useState(false);
  const file = item.type === "fileChange", search = item.type === "webSearch", image = item.type === "imageView", compact = item.type === "contextCompaction";
  const title = file ? `修改 ${item.changes?.length || 0} 个文件` : search ? `搜索：${item.query || "网页"}` : image ? `查看图片：${item.path || ""}` : compact ? "上下文压缩" : item.tool ? `${item.server ? item.server + "/" : ""}${item.tool}` : item.type;
  const details = () => item.details ? JSON.stringify(JSON.parse(item.details), null, 2) : JSON.stringify(item, null, 2);
  return <Trace><TraceCollapsible open={open} onOpenChange={setOpen}><TraceCollapsibleTrigger><TraceLine status={item.status === "inProgress" ? "running" : item.status === "failed" ? "error" : undefined}>
    <TraceIcon>{file ? <FileDiff /> : search ? <Search /> : image ? <Image /> : compact ? <Minimize2 /> : <Wrench />}</TraceIcon><TraceTitle>{title}</TraceTitle><TraceDescription>{item.status || "已记录"}{item.durationMs != null ? ` · ${(item.durationMs / 1000).toFixed(1)}s` : ""}</TraceDescription>
  </TraceLine></TraceCollapsibleTrigger><TraceCollapsibleContent>{open && <>
    {file ? <DiffView changes={item.changes} /> : compact ? <p className="text-sm">{item.status === "inProgress" ? "正在整理会话上下文…" : "已记录本次上下文压缩。"}</p> : <>
      {image && item.path && <Button variant="outline" size="sm" onClick={() => void Data.OpenFile(item.path!).catch(error => onError(String(error)))}>打开图片</Button>}
      <CodeView code={details()} language="工具结果" />
    </>}
  </>}</TraceCollapsibleContent></TraceCollapsible></Trace>;
}
