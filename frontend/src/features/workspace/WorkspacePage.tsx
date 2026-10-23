import { Page } from "@/components/ui/page";
import { createPortal } from "react-dom";
import { useEffect, useState } from "react";
import { Events } from "@wailsio/runtime";
import { Check, ChevronRight, Folder, GitBranch, Layers3, ListChecks, Pencil, Plus, Puzzle, RefreshCw, Tags, Trash2 } from "lucide-react";
import * as Work from "../../../bindings/agentor/internal/workspace/service";
import type { State, StepContext } from "../../../bindings/agentor/internal/workspace/model/models";
import type { Settings } from "../../../bindings/agentor/internal/appdata/model/models";
import type { Status } from "../../../bindings/agentor/internal/codex/model/models";
import { SidebarMenu, SidebarMenuItem } from "@/components/ui/sidebar";
import { PipelinePage } from "@/features/pipeline/PipelinePage";
import type { NotificationTarget } from "@/features/notifications/NotificationCenter";
import { Button } from "@/components/ui/button";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { MarkdownView } from "@/features/chat/MarkdownView";
import { permissionLabels } from "@/features/accounts/PermissionSelect";
import { errorText } from "@/app/errors";
import { EntityEditor, makeDraft, nodeLabels, type EntityDraft } from "./EntityEditor";
import { TagEditor } from "./TagEditor";
import { locate, nodeIdentity, type NodeKind, type Selection } from "./selection";
import { ExecutionPanel } from "./ExecutionPanel";
import { ProjectDetails, VersionDetails, FeatureDetails, TextCard } from "./WorkspaceDetails";
import { ProjectNavigation } from "./ProjectNavigation";
import { NodeActions } from "./NodeActions";
import { useNodeDrag } from "./useNodeDrag";
import { SidebarSelectionButton } from "./SidebarSelectionButton";

function ContextPreview({stepId,state,onError}:{stepId:string;state:State;onError:(message:string)=>void}){
  const [context,setContext]=useState<StepContext>();
  useEffect(()=>{let cancelled=false;setContext(undefined);void Work.GetStepContext(stepId).then(value=>{if(!cancelled)setContext(value);}).catch(error=>{if(!cancelled)onError(errorText(error));});return()=>{cancelled=true;};},[stepId,state]);
  if(!context)return <p className="text-sm text-muted-foreground">正在读取步骤上下文…</p>;
  const text=[`## 项目：${context.projectName}`,context.sharedContext,context.projectPrompt,context.structure,`## 版本：${context.versionName}`,`## 功能：${context.featureTitle}`,context.featureDescription,context.featureContext,context.featurePrompt,`## 步骤：${context.step.title}`,context.step.description,context.step.prompt,`## 绑定文档\n${(context.documents||[]).map(document=>`- ${document.title}`).join("\n")||"暂无绑定文档。"}`].filter(Boolean).join("\n\n");
  return <Collapsible><CollapsibleTrigger className="flex items-center gap-2 text-sm font-medium"><ChevronRight className="size-4"/>查看当前步骤完整上下文</CollapsibleTrigger><CollapsibleContent className="mt-3 rounded-lg border p-4"><MarkdownView text={text} onError={onError}/></CollapsibleContent></Collapsible>;
}

export function WorkspacePage({active,settings,status,focusExecutionId,focusPipeline,headerContainer,sidebarContainer,onActivate,onOpenThread}:{active:boolean;settings:Settings;status?:Status;focusExecutionId?:string;focusPipeline?:NotificationTarget;headerContainer:HTMLElement|null;sidebarContainer:HTMLElement|null;onActivate:()=>void;onOpenThread:(threadId:string)=>void}){
  const [expandedProjects,setExpandedProjects]=useState<Record<string,boolean>>({});
  const [projectTab,setProjectTab]=useState(focusPipeline?"pipeline":"versions");
  const [state,setState]=useState<State>();
  const [busy,setBusy]=useState(false),[error,setError]=useState("");
  const [editing,setEditing]=useState<EntityDraft>();
  const [tagProject,setTagProject]=useState("");
  const [removing,setRemoving]=useState<Selection>();
  async function run(action:()=>Promise<State>){setBusy(true);setError("");try{setState(await action());}catch(error){setError(errorText(error));}finally{setBusy(false);}}
  useEffect(()=>{void run(()=>Work.GetState());},[]);
  useEffect(()=>Events.On("tasks:changed",()=>{void Work.GetState().then(setState).catch(error=>setError(errorText(error)));}),[]);
  const selected=state&&(locate(state,state.selectedId)||(state.projects?.[0]?{kind:"project" as const,project:state.projects[0]}:undefined));
  const identity=selected&&nodeIdentity(selected);
  const project=selected?.project;
  const taggedProject=state?.projects?.find(project=>project.id===tagProject);
  const newEntity=(kind:NodeKind)=>setEditing(makeDraft(kind,selected,settings.projectDirectory));
  const select=(id:string)=>{setProjectTab("versions");void run(()=>Work.Select(id));};
  const drag=useNodeDrag(state,busy,(kind,id,offset)=>{onActivate();void run(()=>Work.MoveNode(kind,id,offset));});
  const nodeActions=(id:string)=>{
    const target=state&&locate(state,id);
    if(!target)return null;
    const childKind=target.kind==="project"?"version":target.kind==="version"?"feature":target.kind==="feature"?"step":undefined;
    return <NodeActions project={active&&target.kind==="project"&&project?.id===id} busy={busy} create={childKind?{label:"新建"+nodeLabels[childKind],onClick:()=>{onActivate();setEditing(makeDraft(childKind,target,settings.projectDirectory));}}:undefined} onDelete={()=>{onActivate();setRemoving(target);}}/>;
  };

  const TypeIcon = selected ? {project:Folder,version:GitBranch,feature:Puzzle,step:ListChecks}[selected.kind] : undefined;
  const pageHeader = selected ? {type:nodeLabels[selected.kind],typeIcon:TypeIcon&&<TypeIcon aria-hidden="true" className="size-4 shrink-0"/>,title:identity?.title,description:selected.step?<div className="flex flex-wrap gap-x-4 gap-y-1 text-xs"><span>{selected.step.threadId?"会话已绑定":"未绑定会话"}</span><span>权限：{selected.step.permissionMode?permissionLabels[selected.step.permissionMode]:"继承项目与账号"}</span></div>:undefined,actions:<div className="flex flex-wrap gap-1">{selected.step&&<Button variant="outline" size="sm" disabled={busy||selected.step.completed} onClick={()=>void run(()=>Work.CompleteStep(selected.step!.id))}><Check/>标记完成</Button>}<Button variant="outline" size="sm" disabled={busy} onClick={()=>setEditing(makeDraft(selected.kind,selected,settings.projectDirectory,true))}><Pencil/>编辑</Button>{selected.kind==="project"&&<Button variant="outline" size="sm" disabled={busy} onClick={()=>setTagProject(selected.project.id)}><Tags/>管理标签</Button>}<Button variant="outline" size="sm" disabled={busy} onClick={()=>setRemoving(selected)}><Trash2/>删除</Button></div>} : undefined;
  return <div className="flex size-full min-h-0 min-w-0">
    {headerContainer&&selected&&selected.kind!=="project"&&createPortal(<nav aria-label="工作区上级位置" className="flex w-max items-center gap-2 text-sm text-muted-foreground">
      <span aria-hidden="true">/</span><button className="whitespace-nowrap py-2 hover:text-foreground" disabled={busy} onClick={()=>select(selected.project.id)}>{selected.project.name}</button>
      {selected.feature&&<><span aria-hidden="true">/</span><button className="whitespace-nowrap py-2 hover:text-foreground" disabled={busy} onClick={()=>select(selected.version!.id)}>{selected.version!.name}</button></>}
      {selected.step&&<><span aria-hidden="true">/</span><button className="whitespace-nowrap py-2 hover:text-foreground" disabled={busy} onClick={()=>select(selected.feature!.id)}>{selected.feature!.title}</button></>}
    </nav>,headerContainer)}
    {sidebarContainer&&settings.mode==="workspace"&&createPortal(<div className="flex min-h-0 flex-1 flex-col" aria-label="工作区项目列表">
      <div className="flex items-center gap-2 p-3"><Button variant="outline" className="min-w-0 flex-1" disabled={busy} onClick={()=>{onActivate();newEntity("project");}}><Plus/>新建项目</Button><Button size="icon" variant="ghost" aria-label="刷新项目" disabled={busy} onClick={()=>void run(()=>Work.GetState())}><RefreshCw/></Button></div>
      <div className="min-h-0 flex-1 overflow-y-auto px-2 pb-4" onDragOver={drag.scroll}><SidebarMenu>{state?.projects?.map(item=><Collapsible key={item.id} open={active&&project?.id===item.id&&expandedProjects[item.id]!==false} render={<SidebarMenuItem className={active&&project?.id===item.id?"rounded-md ring-1 ring-inset ring-foreground":undefined}/>}><div {...drag.row(item.id)} className="group/nav-row relative cursor-grab active:cursor-grabbing"><SidebarSelectionButton sidebar tooltip={item.name} className="group-hover/nav-row:pr-16 group-focus-within/nav-row:pr-16 aria-expanded:rounded-b-none" active={active&&project?.id===item.id} aria-expanded={active&&project?.id===item.id&&expandedProjects[item.id]!==false} disabled={busy} onClick={()=>{setExpandedProjects(old=>({...old,[item.id]:!(active&&project?.id===item.id&&old[item.id]!==false)}));onActivate();select(item.id);}}><Folder/><span>{item.name}</span></SidebarSelectionButton>{nodeActions(item.id)}</div><CollapsibleContent className="overflow-hidden"><ProjectNavigation selected={selected&&active&&project?.id===item.id?selected:{kind:"project",project:item}} busy={busy} actions={nodeActions} dragRow={drag.row} onSelect={id=>{onActivate();select(id);}}/></CollapsibleContent></Collapsible>)}</SidebarMenu>{state&&!state.projects?.length&&<p className="p-3 text-xs text-muted-foreground">创建项目后，在这里切换。</p>}</div>
    </div>,sidebarContainer)}
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
    <section className="min-h-0 min-w-0 flex-1 overflow-y-auto">

      {error&&<div className="m-5"><Alert variant="destructive"><AlertDescription className="flex items-start justify-between gap-3"><span>{error}</span><Button variant="ghost" size="sm" onClick={()=>setError("")}>关闭</Button></AlertDescription></Alert></div>}
      {!selected||!state?<Empty className="m-auto min-h-96"><EmptyHeader><EmptyMedia variant="icon"><Layers3/></EmptyMedia><EmptyTitle>{busy?"正在读取工作区":"把工作安排成清晰的步骤"}</EmptyTitle><EmptyDescription>从本地项目开始，建立版本、功能与步骤，并绑定需要的知识。</EmptyDescription></EmptyHeader><EmptyContent><Button disabled={busy} onClick={()=>newEntity("project")}><Plus/>新建项目</Button></EmptyContent></Empty>:<Page>
        {selected.kind==="project"&&<ProjectDetails header={pageHeader!} key={selected.project.id} project={selected.project} busy={busy} documents={state.documents} onSelect={select} onSaved={setState} onError={setError} onCreateVersion={()=>newEntity("version")} tab={projectTab} onTabChange={setProjectTab} pipeline={<PipelinePage key={selected.project.id+":"+(focusPipeline?.projectId===selected.project.id?focusPipeline.pipelineRunId||"":"")} projectId={selected.project.id} settings={settings} status={status} focusRunId={focusPipeline?.projectId===selected.project.id?focusPipeline.pipelineRunId:undefined}/>}/>}
        {selected.kind==="version"&&<VersionDetails header={pageHeader!} key={selected.version!.id} project={selected.project} version={selected.version!} busy={busy} onSelect={select} onSaved={setState} onError={setError} onCreateFeature={()=>newEntity("feature")}/>}
        {selected.kind==="feature"&&<FeatureDetails header={pageHeader!} key={selected.feature!.id} project={selected.project} version={selected.version!} feature={selected.feature!} busy={busy} documents={state.documents} onSelect={select} onSaved={setState} onError={setError} onCreateStep={()=>newEntity("step")}/>}
        {selected.kind==="step"&&<ExecutionPanel key={selected.step!.id} header={pageHeader!} information={<><TextCard title="步骤说明" text={selected.step!.description} onError={setError}/><TextCard title="步骤提示词" text={selected.step!.prompt} onError={setError}/><ContextPreview stepId={selected.step!.id} state={state} onError={setError}/></>} step={selected.step!} settings={settings} status={status} onError={setError} onOpenThread={onOpenThread} focusExecutionId={focusExecutionId}/> }
      </Page>}
    </section>
    </div>
    {editing&&<EntityEditor key={`${editing.kind}:${editing.id}`} initial={editing} project={state?.projects?.find(project=>project.id===editing.projectId)} onSaved={setState} close={()=>setEditing(undefined)}/>}
    {taggedProject&&<TagEditor project={taggedProject} onSaved={setState} close={()=>setTagProject("")}/>}
    <AlertDialog open={!!removing} onOpenChange={open=>{if(!open)setRemoving(undefined);}}><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>删除{removing&&nodeLabels[removing.kind]}“{removing&&nodeIdentity(removing).title}”？</AlertDialogTitle><AlertDialogDescription>此对象及其下级工作定义会被移除。项目目录和已有 Codex 会话保留。</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel>取消</AlertDialogCancel><AlertDialogAction onClick={()=>{if(removing)void run(()=>Work.DeleteNode(removing.kind,nodeIdentity(removing).id));setRemoving(undefined);}}>删除工作定义</AlertDialogAction></AlertDialogFooter></AlertDialogContent></AlertDialog>
  </div>;
}
