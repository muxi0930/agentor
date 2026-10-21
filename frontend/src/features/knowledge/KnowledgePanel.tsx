import { useEffect, useState } from "react";
import { BookOpen, FileUp, Search, Trash2 } from "lucide-react";
import * as Work from "../../../bindings/agentor/internal/workspace/service";
import type { Document, Feature, KnowledgeContent, Project, SearchMatch, State, Version } from "../../../bindings/agentor/internal/workspace/models";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { MarkdownView } from "@/features/chat/MarkdownView";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { errorText } from "@/app/errors";

export function KnowledgePanel({project,version,feature,documents,onSaved,onError}:{project:Project;version?:Version;feature?:Feature;documents:Document[];onSaved:(state:State)=>void;onError:(message:string)=>void}){
  const [query,setQuery]=useState(""),[matches,setMatches]=useState<SearchMatch[]|null>(null);
  const [content,setContent]=useState<KnowledgeContent>();
  const [removing,setRemoving]=useState<Document>();
  const [busy,setBusy]=useState(false);
  useEffect(()=>{setQuery("");setMatches(null);setContent(undefined);},[project.id,version?.id,feature?.id]);
  async function run(action:()=>Promise<void>){setBusy(true);try{await action();}catch(error){onError(errorText(error));}finally{setBusy(false);}}
  const nodeId=feature?.id||version?.id||"";
  const level=feature?"功能":version?"版本":"项目";
  const ownIds=(feature?feature.documentIds:version?version.documentIds:project.documentIds)||[];
  const inheritedFrom=(id:string)=>nodeId&&project.documentIds?.includes(id)?"项目":feature&&version?.documentIds?.includes(id)?"版本":"";
  const projectDocuments=documents.filter(document=>document.projectId===project.id);
  const selectedDocuments=projectDocuments.filter(document=>ownIds.includes(document.id)||inheritedFrom(document.id));
  const searchDocuments=matches===null?[]:projectDocuments.filter(document=>matches.some(match=>match.documentId===document.id));
  const knowledgeTable=(items:Document[],searchResults=false)=><Table aria-label={searchResults?"知识搜索结果":"已选择的知识"}>
    <TableHeader><TableRow><TableHead>名称</TableHead>{searchResults&&<TableHead>匹配内容</TableHead>}<TableHead>绑定状态</TableHead><TableHead className="text-right">操作</TableHead></TableRow></TableHeader>
    <TableBody>{items.map(document=>{
      const inherited=inheritedFrom(document.id);
      const bound=ownIds.includes(document.id);
      const snippet=searchResults?matches?.find(match=>match.documentId===document.id)?.snippet:undefined;
      return <TableRow key={document.id}>
        <TableCell><Button variant="link" className="h-auto max-w-56 justify-start whitespace-normal break-words p-0 text-left" disabled={busy} onClick={()=>void run(async()=>setContent(await Work.ReadKnowledge(project.id,"",document.id)))}>{document.title}</Button></TableCell>
        {searchResults&&<TableCell className="max-w-96 whitespace-normal break-words text-muted-foreground">{snippet||"—"}</TableCell>}
        <TableCell><Badge variant="outline" className={bound||inherited?"border-transparent bg-green-700 text-white":undefined}>{bound||inherited?"已绑定":"未绑定"}</Badge></TableCell>
        <TableCell><div className="flex justify-end gap-2">{(!inherited||bound)&&<Button className="bg-black text-white hover:bg-black/80" size="xs" disabled={busy} onClick={()=>void run(async()=>onSaved(await Work.BindKnowledge(project.id,nodeId,document.id,!bound)))}>{bound?"解绑":"绑定"}</Button>}<Button size="icon-xs" variant="ghost" disabled={busy} aria-label={`删除知识文档 ${document.title}`} onClick={()=>setRemoving(document)}><Trash2/></Button></div></TableCell>
      </TableRow>;
    })}{!items.length&&<TableRow><TableCell colSpan={searchResults?4:3} className="py-5 text-center text-muted-foreground">{searchResults?"没有匹配的文档。":"尚未选择知识文档。"}</TableCell></TableRow>}</TableBody>
  </Table>;
  return <Card><CardHeader><CardTitle className="flex items-center gap-2 text-base"><BookOpen className="size-4"/>{level}知识</CardTitle><CardDescription>{feature?"自动继承项目与所属版本的知识；本功能绑定仅供本功能的会话使用。":version?"自动继承项目知识；本版本绑定自动供下属功能使用。":"本项目绑定的知识自动供下属版本与功能使用。"} 执行时提供文档索引，按需读取正文。</CardDescription></CardHeader>
    <CardContent className="space-y-4">
      <div className="flex flex-wrap gap-2"><Button variant="outline" disabled={busy} onClick={()=>void run(async()=>onSaved(await Work.ImportMarkdown(project.id,nodeId)))}><FileUp/>导入 Markdown</Button><form className="flex min-w-48 flex-1 gap-2" onSubmit={event=>{event.preventDefault();const keyword=query.trim();if(!keyword){setMatches(null);return;}void run(async()=>setMatches(await Work.SearchKnowledge(project.id,"",keyword)||[]));}}><Input aria-label="搜索项目知识" placeholder="搜索标题与正文…" value={query} onChange={event=>{setQuery(event.target.value);setMatches(null);}}/><Button variant="outline" size="icon" type="submit" disabled={busy||!query.trim()} aria-label="搜索知识"><Search/></Button>{query&&<Button variant="ghost" type="button" onClick={()=>{setMatches(null);setQuery("");}}>清除</Button>}</form></div>
      <section className="space-y-2"><h3 className="text-sm font-medium">已选择的知识</h3>{knowledgeTable(selectedDocuments)}</section>
      {matches!==null&&query.trim()&&<section className="space-y-2"><h3 className="text-sm font-medium">搜索结果</h3>{knowledgeTable(searchDocuments,true)}</section>}
    </CardContent>
    <Dialog open={!!content} onOpenChange={open=>{if(!open)setContent(undefined);}}><DialogContent className="max-h-[85dvh] overflow-y-auto sm:max-w-3xl"><DialogHeader><DialogTitle>{content?.document.title}</DialogTitle><DialogDescription className="break-all">{content?.document.sourcePath?`来源：${content.document.sourcePath}`:"Agent 创建的知识文档"}</DialogDescription></DialogHeader>{content&&<MarkdownView text={content.content} onError={onError}/>}</DialogContent></Dialog>
    <AlertDialog open={!!removing} onOpenChange={open=>{if(!open)setRemoving(undefined);}}><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>删除“{removing?.title}”？</AlertDialogTitle><AlertDialogDescription>删除 agentor 管理的副本及所有绑定，原始导入文件保留。</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel>取消</AlertDialogCancel><AlertDialogAction onClick={()=>{if(removing)void run(async()=>onSaved(await Work.DeleteKnowledge(project.id,removing.id)));setRemoving(undefined);}}>删除副本</AlertDialogAction></AlertDialogFooter></AlertDialogContent></AlertDialog>
  </Card>;
}
