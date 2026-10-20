import { useState } from "react";
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
    </Attachment>)}</AttachmentGroup>
    <Dialog open={!!preview} onOpenChange={open => { if (!open) setPreview(null); }}><DialogContent className="sm:max-w-4xl">
      <DialogHeader><DialogTitle>{preview?.name}</DialogTitle><DialogDescription>原始图片预览</DialogDescription></DialogHeader>
      {preview && <><img src={preview.url} alt={preview.name} className="mx-auto max-h-[65vh] max-w-full object-contain" />
        <div className="flex gap-2"><Button variant="outline" onClick={() => void Files.OpenAttachment(preview.id).catch(error => onError(String(error)))}><ExternalLink />用系统应用打开</Button><Button variant="ghost" onClick={() => void Files.LocateAttachment(preview.id).catch(error => onError(String(error)))}><FolderOpen />定位文件</Button></div></>}
    </DialogContent></Dialog>
  </>;
}
