import { useEffect, useState } from "react";
import { Events } from "@wailsio/runtime";
import { Play, RotateCcw, Square } from "lucide-react";
import * as Tasks from "../../../bindings/agentor/internal/tasks/service";
import * as Work from "../../../bindings/agentor/internal/workspace/service";
import type { State } from "../../../bindings/agentor/internal/tasks/models";
import type { State as WorkState } from "../../../bindings/agentor/internal/workspace/models";
import type { Status } from "../../../bindings/agentor/internal/codex/models";
import type { Settings } from "../../../bindings/agentor/internal/appdata/models";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Field, FieldLabel } from "@/components/ui/field";
import { Textarea } from "@/components/ui/textarea";
import { NativeSelect, NativeSelectOption } from "@/components/ui/native-select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { MarkdownView } from "@/features/chat/MarkdownView";
import { TaskConversation } from "@/features/workspace/TaskConversation";
import { executionLabels } from "@/features/workspace/ExecutionPanel";
import { locate } from "@/features/workspace/selection";
import { errorText } from "@/app/errors";

export function ControllerPage({settings,status,focusRequestId}:{settings:Settings;status?:Status;focusRequestId?:string}){
  const [state,setState]=useState<State>(),[work,setWork]=useState<WorkState>();
  const [text,setText]=useState(""),[projectId,setProject]=useState(""),[stepId,setStep]=useState("");
  const [selected,setSelected]=useState(""),[busy,setBusy]=useState(false),[error,setError]=useState("");
  async function refresh(){try{const [next,definitions]=await Promise.all([Tasks.GetState(),Work.GetState()]);setState(next);setWork(definitions);}catch(error){setError(errorText(error));}}
  useEffect(()=>{void refresh();void Work.GetState().then(value=>{const selected=locate(value,value.selectedId);if(selected)setProject(selected.project.id);}).catch(error=>setError(errorText(error)));return Events.On("tasks:changed",()=>void refresh());},[]);
  const requests=[...(state?.requests||[])].reverse();
  const current=requests.find(request=>request.id===selected)||requests[0];
  const active=requests.find(request=>request.status==="running");
  const steps=(work?.projects||[]).filter(project=>!projectId||project.id===projectId).flatMap(project=>(project.versions||[]).flatMap(version=>(version.features||[]).flatMap(feature=>(feature.steps||[]).map(step=>({id:step.id,label:`${project.name} / ${feature.title} / ${step.title}`})))));
  useEffect(()=>{if(focusRequestId&&state?.requests?.some(request=>request.id===focusRequestId))setSelected(focusRequestId);},[focusRequestId,state]);
  async function run(action:()=>Promise<void>){setBusy(true);setError("");try{await action();await refresh();}catch(error){setError(errorText(error));}finally{setBusy(false);}}
  return <div className="mx-auto w-full max-w-5xl space-y-6 p-5 lg:p-8"><div className="flex flex-wrap items-center gap-3"><div className="mr-auto"><h1 className="text-2xl font-semibold tracking-tight">工作控制器</h1><p className="mt-2 text-sm text-muted-foreground">查询工作、安排步骤、查看结果。请求按顺序处理，正式步骤共用执行队列。</p></div><Badge variant="outline">{active?"正在处理":"空闲"}</Badge><Button variant="outline" size="sm" disabled={busy||!!active||status?.phase!=="ready"} onClick={()=>void run(()=>Tasks.RebuildControllerContext())}><RotateCcw/>重建上下文</Button></div>
    {error&&<Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}
    {state?.paused&&<div className="flex items-center justify-between gap-3 rounded-lg border p-3 text-sm"><p>排队任务等待手动恢复。</p><Button size="sm" disabled={busy||status?.phase!=="ready"} onClick={()=>void run(()=>Tasks.ResumeQueue())}>恢复队列</Button></div>}
    <Card><CardHeader><CardTitle className="text-base">安排工作</CardTitle><CardDescription>控制器只读项目文件，通过工作工具创建定义与派发任务。无法确定目标时会先询问。</CardDescription></CardHeader><CardContent><form className="space-y-4" onSubmit={event=>{event.preventDefault();void run(async()=>{const request=await Tasks.SubmitRequest({text,projectId,stepId,model:settings.model,effort:settings.effort,source:{type:"desktop",appId:"",chatId:"",senderId:"",messageId:crypto.randomUUID(),replyToMessageId:""}});setSelected(request.id);setText("");});}}>
      <div className="grid gap-3 sm:grid-cols-2"><Field><FieldLabel htmlFor="controller-project">项目范围</FieldLabel><NativeSelect id="controller-project" value={projectId} onChange={event=>{setProject(event.target.value);setStep("");}}><NativeSelectOption value="">全部工作区项目</NativeSelectOption>{work?.projects?.map(project=><NativeSelectOption key={project.id} value={project.id}>{project.name}</NativeSelectOption>)}</NativeSelect></Field><Field><FieldLabel htmlFor="controller-step">明确步骤（可选）</FieldLabel><NativeSelect id="controller-step" value={stepId} onChange={event=>setStep(event.target.value)}><NativeSelectOption value="">由请求说明目标</NativeSelectOption>{steps.map(step=><NativeSelectOption key={step.id} value={step.id}>{step.label}</NativeSelectOption>)}</NativeSelect></Field></div>
      <Field><FieldLabel htmlFor="controller-text">工作请求</FieldLabel><Textarea id="controller-text" className="min-h-28" value={text} onChange={event=>setText(event.target.value)} placeholder="例如：查看当前项目的步骤，安排尚未完成的知识检查。"/></Field><div className="flex flex-wrap items-center gap-3"><Button type="submit" disabled={busy||!text.trim()||status?.phase!=="ready"}><Play/>{active?"加入请求队列":"发送请求"}</Button>{active&&<Button type="button" variant="outline" disabled={busy} onClick={()=>void run(()=>Tasks.StopRequest(active.id))}><Square/>停止当前请求</Button>}<span className="text-xs text-muted-foreground">{settings.model||"当前默认模型"} · {settings.effort||"默认强度"} · {requests.filter(request=>request.status==="queued").length} 项排队</span></div>
    </form></CardContent></Card>
    <Card><CardHeader><CardTitle className="text-base">请求与汇报</CardTitle></CardHeader><CardContent className="space-y-4">{!requests.length&&<p className="text-sm text-muted-foreground">还没有控制器请求。</p>}<div className="max-h-64 space-y-2 overflow-y-auto">{requests.map(request=><div key={request.id} className={`flex items-center gap-2 rounded-lg border p-2 ${current?.id===request.id?"bg-accent":""}`}><Button variant="ghost" className="h-auto min-w-0 flex-1 justify-start p-1 text-left" onClick={()=>setSelected(request.id)}><span className="truncate text-xs">{request.kind==="execution_result"?"完成汇报":request.text} · {request.source.type} · {new Date(request.createdAt).toLocaleString()}</span></Button><Badge variant={request.status==="error"?"destructive":"outline"}>{executionLabels[request.status]||request.status}</Badge>{request.status==="queued"&&<Button variant="ghost" size="xs" onClick={()=>void run(()=>Tasks.StopRequest(request.id))}>取消排队</Button>}</div>)}</div>
      {current&&<>{current.error&&<p className="whitespace-pre-wrap text-sm text-destructive">{current.error}</p>}<Tabs defaultValue="result"><TabsList><TabsTrigger value="result">回复</TabsTrigger><TabsTrigger value="conversation">会话过程</TabsTrigger><TabsTrigger value="request">原始请求</TabsTrigger></TabsList><TabsContent value="result">{current.result?<MarkdownView text={current.result} onError={setError}/>:<p className="text-sm text-muted-foreground">等待控制器回复…</p>}</TabsContent><TabsContent value="conversation"><TaskConversation threadId={current.threadId} turnId={current.turnId} running={current.status==="running"} status={status} onError={setError} onFollowup={setText}/></TabsContent><TabsContent value="request"><p className="whitespace-pre-wrap text-sm">{current.text}</p></TabsContent></Tabs></>}
    </CardContent></Card>
  </div>;
}
