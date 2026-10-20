import { useEffect, useState } from "react";
import { BookOpen, FileUp, Search, Trash2 } from "lucide-react";
import * as Work from "../../../bindings/agentor/internal/workspace/service";
import type { Document, Feature, KnowledgeContent, Project, SearchMatch, State } from "../../../bindings/agentor/internal/workspace/models";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { MarkdownView } from "@/features/chat/MarkdownView";
import { errorText } from "@/app/errors";

export function KnowledgePanel({project,feature,documents,onSaved,onError}:{project:Project;feature?:Feature;documents:Document[];onSaved:(state:State)=>void;onError:(message:string)=>void}){
  const [query,setQuery]=useState(""),[matches,setMatches]=useState<SearchMatch[]|null>(null);
  const [content,setContent]=useState<KnowledgeContent>();
  const [removing,setRemoving]=useState<Document>();
  const [busy,setBusy]=useState(false);
  useEffect(()=>{setQuery("");setMatches(null);setContent(undefined);},[project.id]);
  async function run(action:()=>Promise<void>){setBusy(true);try{await action();}catch(error){onError(errorText(error));}finally{setBusy(false);}}
  const visible=documents.filter(document=>document.projectId===project.id && (matches===null || matches.some(match=>match.documentId===document.id)));
  return <Card><CardHeader><CardTitle className="flex items-center gap-2 text-base"><BookOpen className="size-4"/>{feature?"功能知识":"项目知识"}</CardTitle><CardDescription>{feature?"项目共享文档与此功能的文档会一起进入步骤上下文。":"导入 Markdown 后可共享给整个项目，或分别绑定至功能。"}</CardDescription></CardHeader>
    <CardContent className="space-y-4">
      <div className="flex flex-wrap gap-2"><Button variant="outline" disabled={busy} onClick={()=>void run(async()=>onSaved(await Work.ImportMarkdown(project.id,feature?.id||"")))}><FileUp/>导入 Markdown</Button><form className="flex min-w-48 flex-1 gap-2" onSubmit={event=>{event.preventDefault();void run(async()=>setMatches(await Work.SearchKnowledge(project.id,"",query)||[]));}}><Input aria-label="搜索项目知识" placeholder="搜索标题与正文…" value={query} onChange={event=>setQuery(event.target.value)}/><Button variant="outline" size="icon" type="submit" disabled={busy} aria-label="搜索知识"><Search/></Button>{matches!==null&&<Button variant="ghost" type="button" onClick={()=>{setMatches(null);setQuery("");}}>清除</Button>}</form></div>
      {!visible.length&&<p className="py-5 text-sm text-muted-foreground">{matches===null?"尚未导入知识文档。":"没有匹配的文档。"}</p>}
      <div className="space-y-2">{visible.map(document=>{
        const inherited=!!feature&&!!project.documentIds?.includes(document.id);
        const bound=(feature?.documentIds||project.documentIds||[]).includes(document.id);
        return <div key={document.id} className="rounded-lg border p-3"><div className="flex flex-wrap items-center gap-2"><Button variant="link" className="h-auto min-w-0 flex-1 justify-start p-0 text-left" disabled={busy} onClick={()=>void run(async()=>setContent(await Work.ReadKnowledge(project.id,"",document.id)))}>{document.title}</Button>{inherited?<Badge variant="secondary">项目共享</Badge>:<Button variant={bound?"secondary":"outline"} size="xs" disabled={busy} onClick={()=>void run(async()=>onSaved(await Work.BindKnowledge(project.id,feature?.id||"",document.id,!bound)))}>{bound?"取消绑定":feature?"绑定至功能":"共享给项目"}</Button>}<Button size="icon-xs" variant="ghost" aria-label={`删除知识文档 ${document.title}`} onClick={()=>setRemoving(document)}><Trash2/></Button></div>{matches?.find(match=>match.documentId===document.id)?.snippet&&<p className="mt-2 text-xs text-muted-foreground">{matches.find(match=>match.documentId===document.id)!.snippet}</p>}</div>;
      })}</div>
    </CardContent>
    <Dialog open={!!content} onOpenChange={open=>{if(!open)setContent(undefined);}}><DialogContent className="max-h-[85dvh] overflow-y-auto sm:max-w-3xl"><DialogHeader><DialogTitle>{content?.document.title}</DialogTitle><DialogDescription className="break-all">{content?.document.sourcePath?`来源：${content.document.sourcePath}`:"Agent 创建的知识文档"}</DialogDescription></DialogHeader>{content&&<MarkdownView text={content.content} onError={onError}/>}</DialogContent></Dialog>
    <AlertDialog open={!!removing} onOpenChange={open=>{if(!open)setRemoving(undefined);}}><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>删除“{removing?.title}”？</AlertDialogTitle><AlertDialogDescription>删除 agentor 管理的副本及所有绑定，原始导入文件保留。</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel>取消</AlertDialogCancel><AlertDialogAction onClick={()=>{if(removing)void run(async()=>onSaved(await Work.DeleteKnowledge(project.id,removing.id)));setRemoving(undefined);}}>删除副本</AlertDialogAction></AlertDialogFooter></AlertDialogContent></AlertDialog>
  </Card>;
}
