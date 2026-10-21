import { PageContent, PageHeader } from "@/components/ui/page";
import { type ComponentProps, type ReactNode, useEffect, useState } from "react";
import { Events } from "@wailsio/runtime";
import { ArrowUp, Play, RotateCcw, Square } from "lucide-react";
import * as Tasks from "../../../bindings/agentor/internal/tasks/service";
import * as Files from "../../../bindings/agentor/internal/artifact/service";
import type { State } from "../../../bindings/agentor/internal/tasks/models";
import type { Step } from "../../../bindings/agentor/internal/workspace/models";
import type { Attachment } from "../../../bindings/agentor/internal/artifact/models";
import type { Settings } from "../../../bindings/agentor/internal/appdata/models";
import type { Status } from "../../../bindings/agentor/internal/codex/models";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Field, FieldLabel } from "@/components/ui/field";
import { Textarea } from "@/components/ui/textarea";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { MarkdownView } from "@/features/chat/MarkdownView";
import { AttachmentView } from "@/features/chat/AttachmentView";
import { permissionLabels } from "@/features/accounts/PermissionSelect";
import { TaskConversation } from "./TaskConversation";
import { errorText } from "@/app/errors";

export const executionLabels:Record<string,string>={queued:"排队中",running:"运行中",completed:"已完成",error:"失败",interrupted:"已中断",cancelled:"已取消"};

export function ExecutionPanel({header,information,step,settings,status,onError,focusExecutionId}:{header:ComponentProps<typeof PageHeader>;information:ReactNode;step:Step;settings:Settings;status?:Status;onError:(message:string)=>void;focusExecutionId?:string}){
  const [tab,setTab]=useState("information");
  const [state,setState]=useState<State>(),[selected,setSelected]=useState("");
  const [instruction,setInstruction]=useState(""),[criteria,setCriteria]=useState(""),[timeout,setTimeout]=useState("");
  const [steer,setSteer]=useState(""),[busy,setBusy]=useState(false);
  const [artifacts,setArtifacts]=useState<Attachment[]>([]);
  const [rebuilt,setRebuilt]=useState(false);
  useEffect(()=>{if(!rebuilt)return;const timer=window.setTimeout(()=>setRebuilt(false),2000);return()=>window.clearTimeout(timer);},[rebuilt]);
  async function refresh(){try{setState(await Tasks.GetState());}catch(error){onError(errorText(error));}}
  useEffect(()=>{void refresh();return Events.On("tasks:changed",()=>void refresh());},[]);
  const executions=(state?.executions||[]).filter(execution=>execution.stepId===step.id).reverse();
  const active=executions.find(execution=>execution.status==="running");
  const current=executions.find(execution=>execution.id===selected)||executions[0];
  const hasFocusedExecution=executions.some(execution=>execution.id===focusExecutionId);
  useEffect(()=>{if(focusExecutionId&&hasFocusedExecution){setSelected(focusExecutionId);setTab("history");}},[focusExecutionId,hasFocusedExecution]);
  useEffect(()=>{let cancelled=false;setArtifacts([]);if(current)void Files.GetExecutionArtifacts(current.id).then(value=>{if(!cancelled)setArtifacts(value||[]);}).catch(error=>{if(!cancelled)onError(errorText(error));});return()=>{cancelled=true;};},[current?.id,state]);
  async function run(action:()=>Promise<void>){setBusy(true);try{await action();await refresh();}catch(error){onError(errorText(error));}finally{setBusy(false);}}
  return <Tabs value={tab} onValueChange={setTab} className="gap-6">
    <PageHeader {...header}><TabsList aria-label="步骤页面内容" className="h-auto max-w-full flex-wrap justify-start"><TabsTrigger value="information">步骤信息</TabsTrigger><TabsTrigger value="manual">手动执行</TabsTrigger><TabsTrigger value="history">执行记录</TabsTrigger></TabsList></PageHeader>
    <PageContent>
    <TabsContent value="information" className="space-y-6">{information}</TabsContent>
    <TabsContent value="manual"><Card><CardHeader><CardTitle className="text-base">手动执行</CardTitle><CardDescription>同一步骤按顺序执行，其他步骤可以并行。每次执行保存独立记录。</CardDescription></CardHeader><CardContent className="space-y-5">
    {state?.paused&&<div className="flex items-center justify-between gap-3 rounded-md border p-3 text-sm"><p>队列已暂停，排队记录等待手动恢复。</p><Button size="sm" disabled={busy||status?.phase!=="ready"} onClick={()=>void run(()=>Tasks.ResumeQueue())}>恢复队列</Button></div>}
    <form className="space-y-3" onSubmit={event=>{event.preventDefault();void run(async()=>{
      const execution=await Tasks.Dispatch({stepId:step.id,instruction,completionCriteria:criteria,model:settings.model,effort:settings.effort,source:{type:"desktop",appId:"",chatId:"",senderId:"",messageId:crypto.randomUUID(),replyToMessageId:""},requestId:"",attachmentRequestId:"",pipelineRunId:"",timeoutSeconds:Number(timeout)||0});setSelected(execution.id);setInstruction("");
    });}}>
      <Field><FieldLabel htmlFor="execution-instruction">本次执行指令</FieldLabel><Textarea id="execution-instruction" value={instruction} onChange={event=>setInstruction(event.target.value)} placeholder="补充本次任务；留空时按步骤提示词执行。"/></Field>
      <div className="grid gap-3 sm:grid-cols-[1fr_8rem]"><Field><FieldLabel htmlFor="execution-criteria">完成标准</FieldLabel><Input id="execution-criteria" value={criteria} onChange={event=>setCriteria(event.target.value)}/></Field><Field><FieldLabel htmlFor="execution-timeout">超时（秒）</FieldLabel><Input id="execution-timeout" type="number" min="0" value={timeout} placeholder="不限" onChange={event=>setTimeout(event.target.value)}/></Field></div>
      <div className="flex flex-wrap items-center gap-2"><Button type="submit" disabled={busy||status?.phase!=="ready"||(!instruction.trim()&&!step.prompt.trim())}><Play/>{active?"排队执行":"执行步骤"}</Button>{active&&<Button type="button" variant="outline" disabled={busy||active.stopRequested} onClick={()=>void run(()=>Tasks.StopExecution(active.id))}><Square/>{active.stopRequested?"正在停止…":"停止当前执行"}</Button>}<Button type="button" variant="ghost" size="sm" disabled={busy||!!active||status?.phase!=="ready"} onClick={()=>{setRebuilt(false);void run(async()=>{await Tasks.RebuildStepContext(step.id);setRebuilt(true);});}}><RotateCcw className={rebuilt?"text-green-700 dark:text-green-400":undefined}/><span aria-live="polite" className={rebuilt?"text-green-700 dark:text-green-400":undefined}>{rebuilt?"重建成功":"重建上下文"}</span></Button><span className="text-xs text-muted-foreground">{executions.filter(execution=>execution.status==="queued").length} 项排队 · {settings.model||"当前默认模型"} {settings.effort}</span></div>
    </form>
    {active&&<form className="flex gap-2" onSubmit={event=>{event.preventDefault();void run(async()=>{await Tasks.SteerExecution(active.id,steer);setSteer("");});}}><Input aria-label="追加步骤指令" value={steer} onChange={event=>setSteer(event.target.value)} placeholder="给当前执行追加指令…"/><Button type="submit" variant="outline" size="icon" aria-label="追加步骤指令" disabled={busy||!steer.trim()||active.stopRequested}><ArrowUp/></Button></form>}
  </CardContent></Card></TabsContent>
    <TabsContent value="history"><Card><CardHeader><CardTitle className="text-base">执行记录</CardTitle></CardHeader><CardContent>
    {executions.length>0?<div className="space-y-3"><div className="max-h-48 space-y-2 overflow-y-auto">{executions.map(execution=><div key={execution.id} className={`flex items-center gap-2 rounded-md border p-2 ${current?.id===execution.id?"bg-accent":""}`}><Button variant="ghost" className="h-auto min-w-0 flex-1 justify-start p-1 text-left" onClick={()=>setSelected(execution.id)}><span className="truncate text-xs">{new Date(execution.createdAt).toLocaleString()} · {execution.instruction||"按步骤提示词执行"}</span></Button><Badge variant={execution.status==="error"?"destructive":"outline"}>{executionLabels[execution.status]||execution.status}</Badge>{execution.status==="queued"&&<Button size="xs" variant="ghost" onClick={()=>void run(()=>Tasks.StopExecution(execution.id))}>取消排队</Button>}</div>)}</div>
      {current&&<><div className="flex flex-wrap gap-2 text-xs text-muted-foreground"><span>{current.model} · {current.effort||"默认强度"}</span><span>{permissionLabels[current.permission]}</span><span>来源：{current.source.type}</span></div>{current.error&&<p className="whitespace-pre-wrap text-sm text-destructive">{current.error}</p>}<Tabs defaultValue="messages"><TabsList><TabsTrigger value="messages">执行对话</TabsTrigger><TabsTrigger value="result">结果与产物</TabsTrigger><TabsTrigger value="context">执行上下文</TabsTrigger></TabsList><TabsContent value="messages"><TaskConversation key={current.threadId} threadId={current.threadId} turnId={current.turnId} running={current.status==="running"} status={status} onError={onError} onFollowup={text=>{if(active)setSteer(text);else setInstruction(text);setTab("manual");}}/></TabsContent><TabsContent value="result" className="space-y-4">{current.result?<MarkdownView text={current.result} onError={onError}/>:<p className="text-sm text-muted-foreground">{current.status==="running"||current.status==="queued"?"等待执行结果…":"本次没有最终文本结果。"}</p>}{artifacts.length>0&&<AttachmentView files={artifacts} onError={onError}/>}</TabsContent><TabsContent value="context"><MarkdownView text={[`## 项目：${current.context.projectName}`,current.context.sharedContext,current.context.projectPrompt,current.context.structure,`## 功能：${current.context.featureTitle}`,current.context.featureDescription,current.context.featureContext,current.context.featurePrompt,`## 步骤：${current.context.step.title}`,current.context.step.description,current.context.step.prompt,`## 本次指令\n${current.instruction}`,`## 完成标准\n${current.completionCriteria}`,`## 知识文档\n${current.context.documents?.map(document=>`- ${document.title}`).join("\n")||"暂无"}`].filter(Boolean).join("\n\n")} onError={onError}/></TabsContent></Tabs></>}
    </div>:<p className="text-sm text-muted-foreground">暂无执行记录。</p>}
  </CardContent></Card></TabsContent>
    </PageContent>
  </Tabs>;
}
