import "./pipeline.css";
import { useEffect, useState } from "react";
import { Plus } from "lucide-react";
import { Events } from "@wailsio/runtime";
import * as Pipeline from "../../../bindings/agentor/internal/pipeline/service";
import * as Files from "../../../bindings/agentor/internal/artifact/service";
import type { ActionSpec, Schedule, State, Task } from "../../../bindings/agentor/internal/pipeline/model/models";
import type { Attachment } from "../../../bindings/agentor/internal/artifact/models";
import type { Settings } from "../../../bindings/agentor/internal/appdata/model/models";
import type { Status } from "../../../bindings/agentor/internal/codex/model/models";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { AttachmentView } from "@/features/chat/AttachmentView";
import { runLabels as executionLabels, actionName } from "./labels";
import { PipelineFlow } from "./PipelineFlow";
import { errorText } from "@/app/errors";
import { TaskEditor } from "./TaskEditor";
import { ScheduleEditor } from "./ScheduleEditor";
import { RunInputs } from "./RunInputs";
import { PipelineSessions } from "./PipelineSessions";
import { ActionCatalog } from "./ActionCatalog";

export function PipelinePage({settings,status,projectId,focusRunId,focusRequestId}:{settings:Settings;status?:Status;projectId:string;focusRunId?:string;focusRequestId?:string}){
  const [state,setState]=useState<State>(),[actions,setActions]=useState<ActionSpec[]>([]);
  const [taskId,setTask]=useState(""),[runId,setRun]=useState(focusRunId||"");
  const [tab,setTab]=useState(focusRunId?"runs":focusRequestId?"sessions":"tasks"),[inputs,setInputs]=useState<Record<string,unknown>>({});
  const [detailOpen,setDetailOpen]=useState(false),[detailTab,setDetailTab]=useState("run");
  const [editor,setEditor]=useState<Task|null>(),[scheduleEditor,setScheduleEditor]=useState<Schedule|null>();
  const [deletion,setDeletion]=useState<{kind:string;id:string;name:string}>(),[error,setError]=useState(""),[busy,setBusy]=useState(false),[files,setFiles]=useState<Attachment[]>([]);
  async function refresh(){try{const [next,specs]=await Promise.all([Pipeline.GetState(),Pipeline.GetActions()]);setState(next);setActions(specs||[]);}catch(error){setError(errorText(error));}}
  useEffect(()=>{void refresh();return Events.On("pipeline:changed",()=>void refresh());},[]);
  const tasks=(state?.tasks||[]).filter(task=>task.projectId===projectId),schedules=(state?.schedules||[]).filter(schedule=>schedule.projectId===projectId);
  const task=tasks.find(task=>task.id===taskId)||tasks[0];
  const runs=[...(state?.runs||[])].filter(run=>run.projectId===projectId).reverse(),current=runs.find(run=>run.id===runId)||runs[0];
  useEffect(()=>setInputs({}),[task?.id,projectId]);
  useEffect(()=>{let cancelled=false;setFiles([]);if(current)void Files.GetPipelineArtifacts(current.id).then(value=>{if(!cancelled)setFiles(value||[]);}).catch(error=>{if(!cancelled)setError(errorText(error));});return()=>{cancelled=true;};},[current?.id,current?.status]);
  async function run(action:()=>Promise<unknown>){setBusy(true);setError("");try{await action();await refresh();}catch(error){setError(errorText(error));}finally{setBusy(false);}}
  return <div className="space-y-5">
    {error&&<Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}
    {!projectId?<p className="text-sm text-muted-foreground">先在工作区创建项目，再配置流水线。</p>:<Tabs value={tab} onValueChange={setTab}><div className="flex flex-wrap items-center justify-between gap-3"><TabsList aria-label="流水线页面内容"><TabsTrigger value="tasks">流水线列表</TabsTrigger><TabsTrigger value="schedules">定时触发</TabsTrigger><TabsTrigger value="runs">运行记录</TabsTrigger></TabsList><div className="flex flex-wrap gap-1"><Button size="sm" variant="ghost" aria-pressed={tab==="sessions"} onClick={()=>setTab("sessions")}>管理与配置会话</Button><Button size="sm" variant="ghost" aria-pressed={tab==="actions"} onClick={()=>setTab("actions")}>动作目录</Button></div></div>
      <TabsContent value="tasks" className="space-y-5">
        <Button onClick={()=>setEditor(null)}><Plus/>新建流水线</Button>
        <Table aria-label="流水线列表">
          <TableHeader><TableRow><TableHead>名称</TableHead><TableHead className="text-right">节点数</TableHead><TableHead className="text-right">操作</TableHead></TableRow></TableHeader>
          <TableBody>{tasks.map(item=><TableRow key={item.id}>
            <TableCell><Button variant="link" className="h-auto max-w-80 justify-start whitespace-normal break-words p-0 text-left" onClick={()=>{setTask(item.id);setDetailTab("run");setDetailOpen(true);}}>{item.name}</Button></TableCell>
            <TableCell className="text-right">{item.actions?.length||0}</TableCell>
            <TableCell className="text-right"><Button size="xs" variant="ghost" onClick={()=>setDeletion({kind:"task",id:item.id,name:item.name})}>删除</Button></TableCell>
          </TableRow>)}{!tasks.length&&<TableRow><TableCell colSpan={3} className="py-8 text-center text-muted-foreground">此项目暂无流水线。</TableCell></TableRow>}</TableBody>
        </Table>
      </TabsContent>
      <TabsContent value="schedules" className="space-y-4"><div className="flex items-center justify-between"><p className="text-sm text-muted-foreground">请保持 agentor 应用运行。退出期间错过的触发按设置处理；已中断的任务需在运行记录中重试。</p><Button size="sm" disabled={!tasks.length} onClick={()=>setScheduleEditor(null)}>新建定时触发</Button></div>{!schedules.length?<p className="text-sm text-muted-foreground">暂无定时触发。</p>:schedules.map(schedule=><Card key={schedule.id}><CardHeader><CardTitle className="flex items-center gap-2 text-base">{schedule.name}<Badge variant="outline">{schedule.enabled?"已启用":"已停用"}</Badge></CardTitle></CardHeader><CardContent className="space-y-3"><p className="text-sm">{schedule.kind==="once"?"一次性":`每 ${schedule.intervalSeconds} 秒`} · {schedule.timezone} · {schedule.misfirePolicy==="fire_once"?"错过后补运行一次":"错过后跳过"}</p><p className="text-xs text-muted-foreground">下次：{schedule.enabled&&schedule.nextAt?new Date(schedule.nextAt).toLocaleString(undefined,{timeZone:schedule.timezone}):"—"} · 最近：{schedule.lastOutcome||"尚未触发"}</p><div className="flex flex-wrap gap-2"><Button variant="outline" size="xs" disabled={busy} onClick={()=>void run(()=>Pipeline.SetScheduleEnabled(projectId,schedule.id,!schedule.enabled))}>{schedule.enabled?"停用":"启用"}</Button><Button variant="outline" size="xs" onClick={()=>setScheduleEditor(schedule)}>编辑</Button><Button variant="ghost" size="xs" disabled={busy} onClick={()=>void run(async()=>{const value=await Pipeline.TriggerSchedule(projectId,schedule.id);setRun(value.id);setTab("runs");})}>立即触发</Button><Button variant="ghost" size="xs" onClick={()=>setDeletion({kind:"schedule",id:schedule.id,name:schedule.name})}>删除</Button></div></CardContent></Card>)}</TabsContent>
      <TabsContent value="runs" className="space-y-5"><div className="max-h-64 space-y-2 overflow-y-auto">{!runs.length&&<p className="text-sm text-muted-foreground">暂无运行记录。</p>}{runs.map(run=><div key={run.id} className="flex items-center gap-2 rounded-lg border p-2"><Button variant="ghost" className="min-w-0 flex-1 justify-start" onClick={()=>setRun(run.id)}><span className="truncate">{run.task.name} · {new Date(run.createdAt).toLocaleString()}</span></Button><Badge variant={run.status==="error"?"destructive":"outline"}>{executionLabels[run.status]}</Badge>{(run.status==="queued"||run.status==="running"||run.status==="paused")&&<Button size="xs" variant="outline" disabled={busy} onClick={()=>void runActionStop(run.id)}>停止运行</Button>}</div>)}</div>{current&&<Card><CardHeader><CardTitle className="text-base">{current.task.name}</CardTitle><p className="text-xs text-muted-foreground">{current.id} · {current.model} {current.effort} · 来源：{current.source.type}</p></CardHeader><CardContent className="space-y-4"><PipelineFlow actions={current.task.actions || []} results={current.actions || []}/><div className="flex flex-wrap gap-2">{(current.status==="running"||current.status==="queued")&&<Button disabled={busy||current.pauseRequested} variant="outline" onClick={()=>void run(()=>Pipeline.ControlRun(projectId,current.id,"pause"))}>{current.pauseRequested?"等待当前节点完成":"暂停推进"}</Button>}{current.status==="paused"&&<Button disabled={busy} onClick={()=>void run(()=>Pipeline.ControlRun(projectId,current.id,"resume"))}>继续执行</Button>}{(current.status==="error"||current.status==="interrupted")&&<Button disabled={busy} onClick={()=>void run(()=>Pipeline.ControlRun(projectId,current.id,"retry"))}>从失败处重试</Button>}</div>{current.error&&<p className="text-sm text-destructive">{current.error}</p>}{current.actions?.map((action,index)=><div key={`${action.id}:${index}`} className="rounded-lg border p-3"><div className="flex gap-2 text-sm"><span>{actionName(action.type)} · {action.id} · 第 {action.attempt || 1} 次尝试</span><Badge variant="outline">{executionLabels[action.status]}</Badge></div>{action.error&&<p className="mt-2 text-sm text-destructive">{action.error}</p>}<details className="mt-3"><summary className="cursor-pointer text-xs text-muted-foreground">输入与结果</summary><pre className="mt-2 max-h-72 overflow-auto whitespace-pre-wrap rounded bg-muted p-3 text-xs">{JSON.stringify({inputs:action.inputs,output:action.output},null,2)}</pre></details></div>)}{files.length>0&&<AttachmentView files={files} onError={setError}/>}<details><summary className="cursor-pointer text-sm">本次配置与输入快照</summary><pre className="mt-3 overflow-auto whitespace-pre-wrap text-xs">{JSON.stringify({task:current.task,inputs:current.inputs},null,2)}</pre></details></CardContent></Card>}</TabsContent>
      <TabsContent value="sessions"><Card><CardHeader><CardTitle className="text-base">管理与配置会话</CardTitle></CardHeader><CardContent><PipelineSessions key={projectId+":"+(focusRequestId||"")} projectId={projectId} state={state} settings={settings} status={status} onRefresh={refresh} onError={setError} focusRequestId={focusRequestId}/></CardContent></Card></TabsContent>
      <TabsContent value="actions" className="min-w-0"><ActionCatalog actions={actions}/></TabsContent>
    </Tabs>}
    <Dialog open={detailOpen} onOpenChange={setDetailOpen}><DialogContent className="min-w-0 max-h-[88dvh] overflow-y-auto overflow-x-hidden sm:max-w-3xl"><DialogHeader><DialogTitle>{task?.name||"流水线详情"}</DialogTitle></DialogHeader>
      {error&&<Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}
      <Tabs value={detailTab} onValueChange={setDetailTab}><TabsList aria-label="流水线详情"><TabsTrigger value="run">查看与运行</TabsTrigger><TabsTrigger value="config">配置</TabsTrigger></TabsList>
        <TabsContent value="run">{task&&<Card><CardHeader><CardTitle className="text-base">运行 · {task.name}</CardTitle></CardHeader><CardContent className="space-y-4"><PipelineFlow actions={task.actions || []}/><RunInputs task={task} values={inputs} onChange={setInputs}/><div className="flex items-center gap-3"><Button disabled={busy||runs.some(run=>run.taskId===task.id&&(run.status==="queued"||run.status==="running"||run.status==="paused"))} onClick={()=>void run(async()=>{const value=await Pipeline.RunTask({taskId:task.id,inputs,model:settings.model,effort:settings.effort,source:{type:"desktop",appId:"",chatId:"",senderId:"",messageId:crypto.randomUUID(),replyToMessageId:""},requestId:"",sessionRequestId:""});setRun(value.id);setDetailOpen(false);setTab("runs");})}>运行任务</Button><p className="text-xs text-muted-foreground">{settings.model||"默认模型"} · {settings.effort} · 同一任务同时只能运行一次</p></div></CardContent></Card>}</TabsContent>
        <TabsContent value="config">{task&&<TaskEditor key={task.id} projectId={projectId} task={task} actions={actions} onError={setError} onSaved={value=>{setTask(value.id);setDetailTab("run");void refresh();}}/>}</TabsContent>
      </Tabs>
    </DialogContent></Dialog>
    <Dialog open={editor!==undefined} onOpenChange={open=>{if(!open)setEditor(undefined);}}><DialogContent className="min-w-0 max-h-[88dvh] overflow-y-auto overflow-x-hidden sm:max-w-3xl"><DialogHeader><DialogTitle>{editor?"编辑流水线":"新建流水线"}</DialogTitle></DialogHeader>{error&&<Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}{editor!==undefined&&<TaskEditor key={editor?.id||"new"} projectId={projectId} task={editor} actions={actions} onError={setError} onSaved={task=>{setTask(task.id);setEditor(undefined);setDetailTab("run");setDetailOpen(true);void refresh();}}/>}</DialogContent></Dialog>
    <Dialog open={scheduleEditor!==undefined} onOpenChange={open=>{if(!open)setScheduleEditor(undefined);}}><DialogContent className="min-w-0 max-h-[88dvh] overflow-y-auto overflow-x-hidden sm:max-w-2xl"><DialogHeader><DialogTitle>{scheduleEditor?"编辑定时触发":"新建定时触发"}</DialogTitle></DialogHeader>{error&&<Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}{scheduleEditor!==undefined&&<ScheduleEditor key={scheduleEditor?.id||"new"} projectId={projectId} tasks={tasks} defaultTaskId={task?.id} schedule={scheduleEditor} onError={setError} onSaved={()=>{setScheduleEditor(undefined);void refresh();}}/>}</DialogContent></Dialog>
    <AlertDialog open={!!deletion} onOpenChange={open=>{if(!open)setDeletion(undefined);}}><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>删除 {deletion?.name}？</AlertDialogTitle><AlertDialogDescription>移除此配置，保留已有运行记录和产物。</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel>取消</AlertDialogCancel><AlertDialogAction onClick={()=>{const value=deletion;setDeletion(undefined);if(value)void run(()=>value.kind==="task"?Pipeline.DeleteTask(projectId,value.id):Pipeline.DeleteSchedule(projectId,value.id));}}>删除配置</AlertDialogAction></AlertDialogFooter></AlertDialogContent></AlertDialog>
  </div>;
  async function runActionStop(id:string){await run(()=>Pipeline.StopRun(id));}
}
