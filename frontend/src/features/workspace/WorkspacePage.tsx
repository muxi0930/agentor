import { useEffect, useState, type ReactNode } from "react";
import { ArrowDown, ArrowUp, ChevronRight, Folder, Layers3, Pencil, Plus, RefreshCw, Tags, Trash2 } from "lucide-react";
import * as Work from "../../../bindings/agentor/internal/workspace/service";
import type { State, StepContext } from "../../../bindings/agentor/internal/workspace/models";
import type { Settings } from "../../../bindings/agentor/internal/appdata/models";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { NativeSelect, NativeSelectOption } from "@/components/ui/native-select";
import { MarkdownView } from "@/features/chat/MarkdownView";
import { KnowledgePanel } from "@/features/knowledge/KnowledgePanel";
import { permissionLabels } from "@/features/accounts/PermissionSelect";
import { errorText } from "@/app/errors";
import { EntityEditor, makeDraft, nodeLabels, type EntityDraft } from "./EntityEditor";
import { TagEditor } from "./TagEditor";
import { locate, nodeIdentity, type NodeKind, type Selection } from "./selection";

function NavNode({id,title,selected,open,onOpen,onSelect,disabled,children}:{id:string;title:string;selected:string;open:boolean;onOpen:(id:string,open:boolean)=>void;onSelect:(id:string)=>void;disabled:boolean;children?:ReactNode}){
  return <Collapsible open={open} onOpenChange={value=>onOpen(id,value)}><div className={`flex items-center rounded-md ${selected===id?"bg-accent":"hover:bg-muted"}`}>
    {children?<CollapsibleTrigger disabled={disabled} aria-label={`展开或折叠 ${title}`} className="flex size-7 shrink-0 items-center justify-center"><ChevronRight className={`size-3 transition-transform ${open?"rotate-90":""}`}/></CollapsibleTrigger>:<span className="w-7 shrink-0"/>}
    <Button disabled={disabled} variant="ghost" className="h-8 min-w-0 flex-1 justify-start px-1 text-xs font-normal" title={title} onClick={()=>onSelect(id)}><span className="truncate">{title}</span></Button>
  </div>{children&&<CollapsibleContent className="ml-3 border-l pl-1">{children}</CollapsibleContent>}</Collapsible>;
}

function TextCard({title,text,onError}:{title:string;text:string;onError:(text:string)=>void}){
  return <Card><CardHeader><CardTitle className="text-sm">{title}</CardTitle></CardHeader><CardContent>{text?<MarkdownView text={text} onError={onError}/>:<p className="text-sm text-muted-foreground">尚未填写。</p>}</CardContent></Card>;
}

function ContextPreview({stepId,state,onError}:{stepId:string;state:State;onError:(message:string)=>void}){
  const [context,setContext]=useState<StepContext>();
  useEffect(()=>{let cancelled=false;setContext(undefined);void Work.GetStepContext(stepId).then(value=>{if(!cancelled)setContext(value);}).catch(error=>{if(!cancelled)onError(errorText(error));});return()=>{cancelled=true;};},[stepId,state]);
  if(!context)return <p className="text-sm text-muted-foreground">正在读取步骤上下文…</p>;
  const text=[`## 项目：${context.projectName}`,context.sharedContext,context.projectPrompt,context.structure,`## 版本：${context.versionName}`,`## 功能：${context.featureTitle}`,context.featureDescription,context.featureContext,context.featurePrompt,`## 步骤：${context.step.title}`,context.step.description,context.step.prompt,`## 绑定文档\n${(context.documents||[]).map(document=>`- ${document.title}`).join("\n")||"暂无绑定文档。"}`].filter(Boolean).join("\n\n");
  return <Collapsible><CollapsibleTrigger className="flex items-center gap-2 text-sm font-medium"><ChevronRight className="size-4"/>查看当前步骤完整上下文</CollapsibleTrigger><CollapsibleContent className="mt-3 rounded-lg border p-4"><MarkdownView text={text} onError={onError}/></CollapsibleContent></Collapsible>;
}

export function WorkspacePage({settings}:{settings:Settings}){
  const [state,setState]=useState<State>();
  const [busy,setBusy]=useState(false),[error,setError]=useState("");
  const [expanded,setExpanded]=useState<Record<string,boolean>>({});
  const [editing,setEditing]=useState<EntityDraft>();
  const [tagProject,setTagProject]=useState("");
  const [removing,setRemoving]=useState<Selection>();
  async function run(action:()=>Promise<State>){setBusy(true);setError("");try{setState(await action());}catch(error){setError(errorText(error));}finally{setBusy(false);}}
  useEffect(()=>{void run(()=>Work.GetState());},[]);
  useEffect(()=>{if(!state)return;const selected=locate(state,state.selectedId);if(selected)setExpanded(old=>({...old,[selected.project.id]:true,...(selected.version?{[selected.version.id]:true}:{}),...(selected.feature?{[selected.feature.id]:true}:{})}));},[state?.selectedId]);
  const selected=state&&(locate(state,state.selectedId)||(state.projects?.[0]?{kind:"project" as const,project:state.projects[0]}:undefined));
  const identity=selected&&nodeIdentity(selected);
  const taggedProject=state?.projects?.find(project=>project.id===tagProject);
  const newEntity=(kind:NodeKind)=>setEditing(makeDraft(kind,selected,settings.projectDirectory));
  const select=(id:string)=>void run(()=>Work.Select(id));
  const open=(id:string,value:boolean)=>setExpanded(old=>({...old,[id]:value}));
  const nav=(id:string,title:string,children?:ReactNode)=><NavNode key={id} id={id} title={title} selected={state?.selectedId||selected?.project.id||""} open={expanded[id]!==false} onOpen={open} onSelect={select} disabled={busy}>{children}</NavNode>;

  return <div className="flex size-full min-h-0 min-w-0">
    <aside className="flex w-60 shrink-0 flex-col border-r bg-muted/15 max-lg:w-52" aria-label="工作区层级"><div className="flex items-center gap-2 p-3"><Button variant="outline" className="flex-1" disabled={busy} onClick={()=>newEntity("project")}><Plus/>新建项目</Button><Button size="icon" variant="ghost" aria-label="刷新工作区" disabled={busy} onClick={()=>void run(()=>Work.GetState())}><RefreshCw/></Button></div>
      <div className="min-h-0 flex-1 space-y-2 overflow-y-auto px-2 pb-5">{state?.projects?.map(project=>nav(project.id,project.name,!!project.versions?.length&&project.versions.map(version=>nav(version.id,version.name,!!version.features?.length&&version.features.map(feature=>nav(feature.id,feature.title,!!feature.steps?.length&&feature.steps.map(step=>nav(step.id,step.title))))))))}</div>
    </aside>
    <section className="min-h-0 min-w-0 flex-1 overflow-y-auto">
      {error&&<div className="m-5"><Alert variant="destructive"><AlertDescription className="flex items-start justify-between gap-3"><span>{error}</span><Button variant="ghost" size="sm" onClick={()=>setError("")}>关闭</Button></AlertDescription></Alert></div>}
      {!selected||!state?<Empty className="m-auto min-h-96"><EmptyHeader><EmptyMedia variant="icon"><Layers3/></EmptyMedia><EmptyTitle>{busy?"正在读取工作区":"把工作安排成清晰的步骤"}</EmptyTitle><EmptyDescription>从本地项目开始，建立版本、功能与步骤，并绑定需要的知识。</EmptyDescription></EmptyHeader><EmptyContent><Button disabled={busy} onClick={()=>newEntity("project")}><Plus/>新建项目</Button></EmptyContent></Empty>:<div className="mx-auto max-w-5xl space-y-6 p-5 lg:p-8">
        <div className="flex flex-wrap items-start gap-3"><div className="mr-auto min-w-0"><p className="mb-2 text-xs text-muted-foreground">{[selected.project.name,selected.version?.name,selected.feature?.title].filter(Boolean).join(" / ")}</p><h1 className="break-words text-2xl font-semibold tracking-tight">{identity?.title}</h1><p className="mt-2 text-xs text-muted-foreground">{nodeLabels[selected.kind]}</p></div>
          <div className="flex flex-wrap gap-1"><Button variant="outline" size="sm" disabled={busy} onClick={()=>setEditing(makeDraft(selected.kind,selected,settings.projectDirectory,true))}><Pencil/>编辑</Button><Button variant="ghost" size="icon-sm" aria-label="上移当前对象" disabled={busy} onClick={()=>void run(()=>Work.MoveNode(selected.kind,identity!.id,-1))}><ArrowUp/></Button><Button variant="ghost" size="icon-sm" aria-label="下移当前对象" disabled={busy} onClick={()=>void run(()=>Work.MoveNode(selected.kind,identity!.id,1))}><ArrowDown/></Button><Button variant="ghost" size="icon-sm" aria-label="删除当前对象" disabled={busy} onClick={()=>setRemoving(selected)}><Trash2/></Button></div>
        </div>
        {selected.kind==="project"&&<><div className="flex flex-wrap gap-2"><Button onClick={()=>newEntity("version")}><Plus/>新建版本</Button><Button variant="outline" onClick={()=>setTagProject(selected.project.id)}><Tags/>管理标签</Button></div><p className="break-all text-sm text-muted-foreground"><Folder className="mr-2 inline size-4"/>{selected.project.directory}</p>
          <div className="grid gap-4 sm:grid-cols-2">{selected.project.versions?.map(version=><Card key={version.id}><CardHeader><CardTitle><Button variant="link" className="h-auto p-0 text-base" onClick={()=>select(version.id)}>{version.name}</Button></CardTitle><CardDescription>{version.description||"暂无说明"}</CardDescription></CardHeader><CardContent><div className="flex flex-wrap gap-2">{version.tagIds?.map(id=>{const tag=selected.project.tags?.find(tag=>tag.id===id);return tag&&<Badge key={id} variant="outline"><span className="mr-1 size-2 rounded-full" style={{background:tag.color}}/>{tag.name}</Badge>;})}<Badge variant="secondary">{version.features?.length||0} 个功能</Badge></div></CardContent></Card>)}</div>
          <TextCard title="共享上下文" text={selected.project.sharedContext} onError={setError}/><TextCard title="项目提示词" text={selected.project.prompt} onError={setError}/><TextCard title="项目结构说明" text={selected.project.structure} onError={setError}/>
        </>}
        {selected.kind==="version"&&<><div className="flex gap-2"><Button onClick={()=>newEntity("feature")}><Plus/>新建功能</Button></div><TextCard title="版本说明" text={selected.version!.description} onError={setError}/><div className="grid gap-4 sm:grid-cols-2">{selected.version!.features?.map(feature=><Card key={feature.id}><CardHeader><CardTitle><Button variant="link" className="h-auto p-0 text-base" onClick={()=>select(feature.id)}>{feature.title}</Button></CardTitle><CardDescription>{feature.description||"暂无说明"}</CardDescription></CardHeader><CardContent><Badge variant="secondary">{feature.steps?.length||0} 个步骤</Badge></CardContent></Card>)}</div></>}
        {selected.kind==="feature"&&<><div className="flex flex-wrap gap-2"><Button onClick={()=>newEntity("step")}><Plus/>新建步骤</Button><NativeSelect aria-label="功能所在版本" value={selected.version!.id} disabled={busy} onChange={event=>void run(()=>Work.MoveFeature(selected.project.id,selected.feature!.id,event.target.value))}>{selected.project.versions?.map(version=><NativeSelectOption key={version.id} value={version.id}>{version.name}</NativeSelectOption>)}</NativeSelect></div><TextCard title="功能说明" text={selected.feature!.description} onError={setError}/><div className="space-y-2">{selected.feature!.steps?.map((step,index)=><Button key={step.id} variant="outline" className="h-auto w-full justify-start gap-3 px-4 py-3" onClick={()=>select(step.id)}><span className="text-xs text-muted-foreground">{index+1}</span><span className="min-w-0 flex-1 truncate text-left">{step.title}</span><Badge variant="secondary">{step.threadId?"已绑定会话":"未绑定会话"}</Badge></Button>)}</div><TextCard title="功能上下文" text={selected.feature!.context} onError={setError}/><TextCard title="功能提示词" text={selected.feature!.prompt} onError={setError}/></>}
        {selected.kind==="step"&&<><div className="flex flex-wrap gap-2"><Badge variant="outline">{selected.step!.threadId?"会话已绑定":"未绑定会话"}</Badge><Badge variant="outline">权限：{selected.step!.permissionMode?permissionLabels[selected.step!.permissionMode]:"继承项目与账号"}</Badge></div><TextCard title="步骤说明" text={selected.step!.description} onError={setError}/><TextCard title="步骤提示词" text={selected.step!.prompt} onError={setError}/><ContextPreview stepId={selected.step!.id} state={state} onError={setError}/></>}
        <KnowledgePanel key={selected.project.id} project={selected.project} feature={selected.feature} documents={state.documents||[]} onSaved={setState} onError={setError}/>
      </div>}
    </section>
    {editing&&<EntityEditor key={`${editing.kind}:${editing.id}`} initial={editing} project={state?.projects?.find(project=>project.id===editing.projectId)} onSaved={setState} close={()=>setEditing(undefined)}/>}
    {taggedProject&&<TagEditor project={taggedProject} onSaved={setState} close={()=>setTagProject("")}/>}
    <AlertDialog open={!!removing} onOpenChange={open=>{if(!open)setRemoving(undefined);}}><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>删除{removing&&nodeLabels[removing.kind]}“{removing&&nodeIdentity(removing).title}”？</AlertDialogTitle><AlertDialogDescription>此对象及其下级工作定义会被移除。项目目录和已有 Codex 会话保留。</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel>取消</AlertDialogCancel><AlertDialogAction onClick={()=>{if(removing)void run(()=>Work.DeleteNode(removing.kind,nodeIdentity(removing).id));setRemoving(undefined);}}>删除工作定义</AlertDialogAction></AlertDialogFooter></AlertDialogContent></AlertDialog>
  </div>;
}
