import { Page, PageContent, PageHeader } from "@/components/ui/page";
import { useEffect, useState } from "react";
import { Events } from "@wailsio/runtime";
import { ArrowUp, LoaderCircle, RotateCcw, Settings2, Square } from "lucide-react";
import * as Codex from "../../../bindings/agentor/internal/codex/service";
import * as Data from "../../../bindings/agentor/internal/appdata/service";
import * as Tasks from "../../../bindings/agentor/internal/tasks/service";
import * as Work from "../../../bindings/agentor/internal/workspace/service";
import type { State } from "../../../bindings/agentor/internal/tasks/models";
import type { State as WorkState } from "../../../bindings/agentor/internal/workspace/models";
import type { Model, Status } from "../../../bindings/agentor/internal/codex/models";
import type { Settings } from "../../../bindings/agentor/internal/appdata/models";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Field, FieldLabel } from "@/components/ui/field";
import { Textarea } from "@/components/ui/textarea";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { MarkdownView } from "@/features/chat/MarkdownView";
import { TaskConversation } from "@/features/workspace/TaskConversation";
import { executionLabels } from "@/features/workspace/ExecutionPanel";
import { locate } from "@/features/workspace/selection";
import { errorText } from "@/app/errors";

export function ControllerPage({settings,onSettings,status,focusRequestId,openSettings,onOpenThread}:{settings:Settings;onSettings:(settings:Settings)=>void;status?:Status;focusRequestId?:string;openSettings:()=>void;onOpenThread:(threadId:string)=>void}){
  const [state,setState]=useState<State>(),[work,setWork]=useState<WorkState>();
  const [text,setText]=useState(""),[projectId,setProject]=useState(""),[stepId,setStep]=useState("");
  const [selected,setSelected]=useState(""),[busy,setBusy]=useState(false),[error,setError]=useState("");
  const [rebuilt,setRebuilt]=useState(false);
  useEffect(()=>{if(!rebuilt)return;const timer=window.setTimeout(()=>setRebuilt(false),2000);return()=>window.clearTimeout(timer);},[rebuilt]);
  async function refresh(){try{const [next,definitions]=await Promise.all([Tasks.GetState(),Work.GetState()]);setState(next);setWork(definitions);}catch(error){setError(errorText(error));}}
  useEffect(()=>{void refresh();void Work.GetState().then(value=>{const selected=locate(value,value.selectedId);if(selected)setProject(selected.project.id);}).catch(error=>setError(errorText(error)));return Events.On("tasks:changed",()=>void refresh());},[]);
  const [models,setModels]=useState<Model[]>([]);
  useEffect(()=>{
    if(status?.phase!=="ready")return;
    let cancelled=false;
    void Codex.GetModels().then(value=>{if(!cancelled)setModels(value||[]);}).catch(error=>{if(!cancelled)setError(errorText(error));});
    return ()=>{cancelled=true;};
  },[status?.generation,status?.phase]);
  const model=models.find(model=>model.model===settings.model);
  const requests=[...(state?.requests||[])].reverse();
  const current=requests.find(request=>request.id===selected);
  const currentIndex=requests.findIndex(request=>request.id===selected);
  const focusedRequestExists=!!focusRequestId&&requests.some(request=>request.id===focusRequestId);
  const active=requests.find(request=>request.status==="running");
  const steps=(work?.projects||[]).filter(project=>!projectId||project.id===projectId).flatMap(project=>(project.versions||[]).flatMap(version=>(version.features||[]).flatMap(feature=>(feature.steps||[]).map(step=>({id:step.id,label:`${project.name} / ${feature.title} / ${step.title}`})))));
  useEffect(()=>{if(focusRequestId&&focusedRequestExists)setSelected(focusRequestId);},[focusRequestId,focusedRequestExists]);
  async function run(action:()=>Promise<void>){setBusy(true);setError("");try{await action();await refresh();}catch(error){setError(errorText(error));}finally{setBusy(false);}}
  return <Page><PageHeader title="控制中心" description="查询工作、安排步骤、查看结果。请求按顺序处理，正式步骤共用执行队列。" actions={<><Badge variant="outline">{active?"正在处理":"空闲"}</Badge><Button variant="outline" size="sm" disabled={busy||!!active||status?.phase!=="ready"} onClick={()=>{setRebuilt(false);void run(async()=>{await Tasks.RebuildControllerContext();setRebuilt(true);});}}><RotateCcw className={rebuilt?"text-green-700 dark:text-green-400":undefined}/><span aria-live="polite" className={rebuilt?"text-green-700 dark:text-green-400":undefined}>{rebuilt?"重建成功":"重建Session"}</span></Button></>}/><PageContent>
    {error&&<Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}
    {state?.paused&&<div className="flex items-center justify-between gap-3 rounded-lg border p-3 text-sm"><p>排队任务等待手动恢复。</p><Button size="sm" disabled={busy||status?.phase!=="ready"} onClick={()=>void run(()=>Tasks.ResumeQueue())}>恢复队列</Button></div>}
    <Card><CardHeader><CardTitle className="text-base">安排工作</CardTitle><CardDescription>控制中心只读项目文件，通过工作工具创建定义与派发任务。无法确定目标时会先询问。</CardDescription></CardHeader><CardContent><form className="space-y-4" onSubmit={event=>{event.preventDefault();void run(async()=>{const request=await Tasks.SubmitRequest({text,projectId,stepId,model:settings.model,effort:settings.effort,source:{type:"desktop",appId:"",chatId:"",senderId:"",messageId:crypto.randomUUID(),replyToMessageId:""}});setSelected(request.id);setText("");});}}>
      <div className="grid gap-3 sm:grid-cols-2"><Field><FieldLabel htmlFor="controller-project">项目范围</FieldLabel><Select value={projectId} onValueChange={selectedValue => { if (selectedValue === null) return; setProject(selectedValue);setStep(""); }} items={[{ value: "", label: "全部工作区项目" }, ...(work?.projects?.map(project=>({ value: project.id, label: project.name })) ?? [])]}><SelectTrigger id="controller-project"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="">全部工作区项目</SelectItem>{work?.projects?.map(project=><SelectItem key={project.id} value={project.id}>{project.name}</SelectItem>)}</SelectContent></Select></Field><Field><FieldLabel htmlFor="controller-step">明确步骤（可选）</FieldLabel><Select value={stepId} onValueChange={selectedValue => { if (selectedValue === null) return; setStep(selectedValue); }} items={[{ value: "", label: "由请求说明目标" }, ...(steps.map(step=>({ value: step.id, label: step.label })))]}><SelectTrigger id="controller-step"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="">由请求说明目标</SelectItem>{steps.map(step=><SelectItem key={step.id} value={step.id}>{step.label}</SelectItem>)}</SelectContent></Select></Field></div>
      <Field><FieldLabel htmlFor="controller-text">工作请求</FieldLabel><Textarea id="controller-text" className="min-h-28" value={text} onChange={event=>setText(event.target.value)} placeholder="例如：查看当前项目的步骤，安排尚未完成的知识检查。"/></Field><div className="flex flex-wrap items-center gap-2">
              <Select disabled={busy||status?.phase!=="ready"} value={settings.model} onValueChange={selectedValue => { if (selectedValue === null) return; void run(async () => {
                const selected = models.find(model => model.model === selectedValue);
                onSettings(await Data.SetModelSelection(selectedValue, selected?.defaultReasoningEffort || ""));
              }); }} items={[{ value: "", label: settings.reuseCLI?"CLI 默认模型":"账号默认模型" }, ...(models.map(model => ({ value: model.model, label: model.displayName })))]}><SelectTrigger aria-label="选择模型" size="sm" className="max-w-40 min-w-0 gap-1 border-0 bg-transparent px-2 text-[11px] shadow-none [&_svg]:size-3"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="">{settings.reuseCLI?"CLI 默认模型":"账号默认模型"}</SelectItem>{models.map(model => <SelectItem key={model.id} value={model.model}>{model.displayName}</SelectItem>)}</SelectContent></Select>
              {model && !!model.supportedReasoningEfforts?.length && <Select disabled={busy||status?.phase!=="ready"} value={settings.effort || model.defaultReasoningEffort} onValueChange={selectedValue => { if (selectedValue === null) return; void run(async () => onSettings(await Data.SetModelSelection(settings.model, selectedValue))); }} items={(model.supportedReasoningEfforts || []).map(option => ({ value: option.reasoningEffort, label: option.reasoningEffort }))}><SelectTrigger aria-label="思考强度" size="sm" className="w-20 min-w-0 gap-1 border-0 bg-transparent px-2 text-[11px] shadow-none [&_svg]:size-3"><SelectValue /></SelectTrigger><SelectContent>
                {(model.supportedReasoningEfforts || []).map(option => <SelectItem key={option.reasoningEffort} value={option.reasoningEffort}>{option.reasoningEffort}</SelectItem>)}
              </SelectContent></Select>}
              <span className="text-xs text-muted-foreground">{requests.filter(request=>request.status==="queued").length} 项排队</span>
              <div className="ml-auto flex items-center gap-2">
                {active&&<Button type="button" variant="outline" disabled={busy} onClick={()=>void run(()=>Tasks.StopRequest(active.id))}><Square className="size-3"/>停止当前请求</Button>}
                <Button type="button" variant="outline" onClick={openSettings}><Settings2 />设置</Button>
                <Button type="submit" aria-label={active?"加入请求队列":"发送请求"} disabled={busy||!text.trim()||status?.phase!=="ready"}>{busy?<LoaderCircle className="animate-spin"/>:<ArrowUp/>}{active?"加入请求队列":"发送请求"}</Button>
              </div>
            </div>
    </form></CardContent></Card>
    <Card><CardHeader className="items-center"><CardTitle className="text-base">请求与汇报</CardTitle><CardAction><Button variant="outline" size="sm" disabled={!state?.controllerThreadId} onClick={()=>{if(state?.controllerThreadId)onOpenThread(state.controllerThreadId);}}>跳转到当前活跃Session</Button></CardAction></CardHeader><CardContent>
      <div className="max-h-64 overflow-y-auto"><Table className="table-fixed">
        <TableHeader><TableRow><TableHead className="w-44">时间</TableHead><TableHead className="w-28">来源</TableHead><TableHead>内容</TableHead></TableRow></TableHeader>
        <TableBody>{requests.map(request=><TableRow key={request.id} className="cursor-pointer" data-state={current?.id===request.id?"selected":undefined} onClick={()=>setSelected(request.id)}>
          <TableCell className="text-xs text-muted-foreground">{new Date(request.createdAt).toLocaleString()}</TableCell>
          <TableCell className="truncate" title={request.source.type}>{request.source.type}</TableCell>
          <TableCell><div className="flex min-w-0 items-center gap-2">
            <Button variant="link" className="h-auto min-w-0 flex-1 justify-start p-0 text-left" aria-haspopup="dialog" onClick={event=>{event.stopPropagation();setSelected(request.id);}}><span className="truncate">{request.kind==="execution_result"?"完成汇报":request.text}</span></Button>
            <Badge variant={request.status==="error"?"destructive":"outline"}>{executionLabels[request.status]||request.status}</Badge>
            {request.status==="queued"&&<Button variant="ghost" size="xs" disabled={busy} onClick={event=>{event.stopPropagation();void run(()=>Tasks.StopRequest(request.id));}}>取消排队</Button>}
          </div></TableCell>
        </TableRow>)}{!requests.length&&<TableRow><TableCell colSpan={3} className="py-6 text-center text-muted-foreground">还没有控制中心请求。</TableCell></TableRow>}</TableBody>
      </Table></div>
    </CardContent></Card>
    <Sheet open={!!current} onOpenChange={open=>{if(!open)setSelected("");}}><SheetContent side="right" className="w-full sm:max-w-3xl">
      <SheetHeader><SheetTitle>请求与汇报详情</SheetTitle><SheetDescription>{current&&<>{new Date(current.createdAt).toLocaleString()} · {current.source.type} · {current.kind==="execution_result"?"完成汇报":"工作请求"}</>}</SheetDescription></SheetHeader>
      {current&&<div className="min-h-0 flex-1 space-y-4 overflow-y-auto px-4 pb-6 [overflow-wrap:anywhere]">
        <div className="flex flex-wrap items-center gap-2"><Badge variant={current.status==="error"?"destructive":"outline"}>{executionLabels[current.status]||current.status}</Badge>
          {current.status==="queued"&&<Button variant="outline" size="sm" disabled={busy} onClick={()=>void run(()=>Tasks.StopRequest(current.id))}>取消排队</Button>}
          {current.status==="running"&&<Button variant="outline" size="sm" disabled={busy} onClick={()=>void run(()=>Tasks.StopRequest(current.id))}><Square className="size-3"/>停止当前请求</Button>}
          <div className="ml-auto flex gap-2"><Button variant="ghost" size="sm" disabled={currentIndex<=0} onClick={()=>setSelected(requests[currentIndex-1].id)}>上一条</Button><Button variant="ghost" size="sm" disabled={currentIndex>=requests.length-1} onClick={()=>setSelected(requests[currentIndex+1].id)}>下一条</Button></div>
        </div>
        {current.error&&<p className="whitespace-pre-wrap text-sm text-destructive">{current.error}</p>}
        <Tabs key={current.id} defaultValue="result"><TabsList><TabsTrigger value="result">回复</TabsTrigger><TabsTrigger value="conversation">会话过程</TabsTrigger><TabsTrigger value="request">原始请求</TabsTrigger></TabsList>
          <TabsContent value="result">{current.result?<MarkdownView text={current.result} onError={setError}/>:<p className="text-sm text-muted-foreground">等待控制中心回复…</p>}</TabsContent>
          <TabsContent value="conversation"><TaskConversation key={current.id} threadId={current.threadId} turnId={current.turnId} running={current.status==="running"} status={status} onError={setError} onFollowup={value=>{setText(value);setSelected("");}}/></TabsContent>
          <TabsContent value="request"><p className="whitespace-pre-wrap text-sm">{current.text}</p></TabsContent>
        </Tabs>
      </div>}
    </SheetContent></Sheet>
  </PageContent></Page>;
}
