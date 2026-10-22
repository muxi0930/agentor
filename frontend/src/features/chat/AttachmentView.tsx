import { Tooltip, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import { useEffect, useState } from "react";
import { Events } from "@wailsio/runtime";
import * as Lark from "../../../bindings/agentor/internal/lark/service";
import type { Source } from "../../../bindings/agentor/internal/tasks/models";
import { ExternalLink, FileText, FolderOpen, X } from "lucide-react";
import type { Attachment as AttachmentData } from "../../../bindings/agentor/internal/artifact/models";
import * as Files from "../../../bindings/agentor/internal/artifact/service";
import { Attachment, AttachmentContent, AttachmentDescription, AttachmentGroup, AttachmentMedia, AttachmentTitle } from "@/components/ui/attachment";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";

export function AttachmentView({ files, onRemove, onError }: { files: AttachmentData[]; onRemove?: (id: string) => void; onError: (message: string) => void }) {
  const [preview, setPreview] = useState<AttachmentData | null>(null);
  return <>
    <AttachmentGroup>{files.map(file => <Attachment key={file.id} size="sm" state={file.status === "failed" ? "error" : file.status === "pending" ? "uploading" : "done"}>
      <button type="button" className="flex min-w-0 items-center gap-2 rounded-lg text-left outline-none focus-visible:ring-2 focus-visible:ring-ring" onClick={() => {
        if (file.mediaType.startsWith("image/")) setPreview(file); else void Files.OpenAttachment(file.id).catch(error => onError(String(error)));
      }}><AttachmentMedia variant={file.mediaType.startsWith("image/") ? "image" : "icon"}>{file.mediaType.startsWith("image/") ? <img src={file.url} alt={file.name} width={40} height={40} loading="lazy" /> : <FileText />}</AttachmentMedia>
        <AttachmentContent><AttachmentTitle className="max-w-40">{file.name}</AttachmentTitle><AttachmentDescription>{file.status === "failed" ? file.error : `${Math.max(1, Math.round(file.size / 1024)).toLocaleString()} KB`}</AttachmentDescription></AttachmentContent>
      </button>
      {onRemove && <Button variant="ghost" type="button" size="icon-xs" aria-label={`移除附件 ${file.name}`} onClick={() => onRemove(file.id)}><X /></Button>}
      {file.kind==="artifact"&&file.status==="ready"&&<ShareArtifactButton file={file} onError={onError}/>}
    </Attachment>)}</AttachmentGroup>
    <Dialog open={!!preview} onOpenChange={open => { if (!open) setPreview(null); }}><DialogContent className="sm:max-w-4xl">
      <DialogHeader><DialogTitle>{preview?.name}</DialogTitle><DialogDescription>原始图片预览</DialogDescription></DialogHeader>
      {preview && <><img src={preview.url} alt={preview.name} className="mx-auto max-h-[65vh] max-w-full object-contain" />
        <div className="flex gap-2"><Button variant="outline" onClick={() => void Files.OpenAttachment(preview.id).catch(error => onError(String(error)))}><ExternalLink />用系统应用打开</Button><Button variant="ghost" onClick={() => void Files.LocateAttachment(preview.id).catch(error => onError(String(error)))}><FolderOpen />定位文件</Button></div></>}
    </DialogContent></Dialog>
  </>;
}

function ShareArtifactButton({file,onError}:{file:AttachmentData;onError:(text:string)=>void}){
  const [target,setTarget]=useState<Source>(),[reason,setReason]=useState(""),[busy,setBusy]=useState(false),[queued,setQueued]=useState(false);
  useEffect(()=>{let cancelled=false;async function refresh(){try{const value=await Lark.GetArtifactShareTarget(file.id);if(!cancelled){setTarget(value);setReason("");}}catch(error){if(!cancelled){setTarget(undefined);setReason(String(error));}}}void refresh();const off=Events.On("lark:changed",()=>void refresh());return()=>{cancelled=true;off();};},[file.id]);
  const recipient=target?.chatType==="group"?target.chatId:target?.senderId;
  return <div className="ml-2 flex max-w-48 flex-col gap-1"><Tooltip><TooltipTrigger render={<Button variant="outline" size="xs" disabled={busy||!target} aria-label={`分享 ${file.name} 至飞书 ${recipient||"未配对接收者"}`} onClick={()=>{setBusy(true);void Lark.ShareArtifact(file.id).then(()=>setQueued(true)).catch(error=>onError(String(error))).finally(()=>setBusy(false));}}>{queued?"已加入发送记录":"分享到飞书"}</Button>} /><TooltipContent>{reason||`发送至 ${recipient}`}</TooltipContent></Tooltip><Tooltip><TooltipTrigger render={<span className="truncate text-[10px] text-muted-foreground">{recipient||"配对后可分享"}</span>} /><TooltipContent>{recipient||reason}</TooltipContent></Tooltip></div>;
}
