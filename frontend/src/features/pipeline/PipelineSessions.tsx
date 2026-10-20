import { useState } from "react";
import type { State } from "../../../bindings/agentor/internal/pipeline/models";
import type { Settings } from "../../../bindings/agentor/internal/appdata/models";
import type { Status } from "../../../bindings/agentor/internal/codex/models";
import * as Pipeline from "../../../bindings/agentor/internal/pipeline/service";
import { Button } from "@/components/ui/button";
import { Field, FieldLabel } from "@/components/ui/field";
import { Textarea } from "@/components/ui/textarea";
import { NativeSelect, NativeSelectOption } from "@/components/ui/native-select";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { MarkdownView } from "@/features/chat/MarkdownView";
import { TaskConversation } from "@/features/workspace/TaskConversation";
import { executionLabels } from "@/features/workspace/ExecutionPanel";
import { errorText } from "@/app/errors";

export function PipelineSessions({projectId,state,settings,status,onRefresh,onError,focusRequestId}:{projectId:string;state?:State;settings:Settings;status?:Status;onRefresh:()=>Promise<void>;onError:(text:string)=>void;focusRequestId?:string}){
  const [role,setRole]=useState("pipeline_config"),[text,setText]=useState(""),[selected,setSelected]=useState(focusRequestId||""),[busy,setBusy]=useState(false);
  const requests=[...(state?.requests||[])].filter(request=>request.input.projectId===projectId).reverse();
  const current=requests.find(request=>request.id===selected)||requests[0];
  async function run(action:()=>Promise<unknown>){setBusy(true);try{await action();await onRefresh();}catch(error){onError(errorText(error));}finally{setBusy(false);}}
  return <div className="space-y-5"><form className="space-y-3" onSubmit={event=>{event.preventDefault();void run(async()=>{const request=await Pipeline.SubmitSession({projectId,role,text,model:settings.model,effort:settings.effort,source:{type:"desktop",appId:"",chatId:"",senderId:"",messageId:crypto.randomUUID(),replyToMessageId:""},requestId:""});setSelected(request.id);setText("");});}}><Field><FieldLabel>会话角色</FieldLabel><NativeSelect aria-label="流水线会话角色" value={role} onChange={event=>setRole(event.target.value)}><NativeSelectOption value="pipeline_config">配置会话</NativeSelectOption><NativeSelectOption value="pipeline_manager">管理会话</NativeSelectOption></NativeSelect></Field><p className="text-sm text-muted-foreground">{role==="pipeline_config"?"通过工具保存 JSON 任务与调度，配置保存后由您决定何时运行。":"查询动作、任务与结果，运行已有任务。"}同一项目的同类会话按顺序处理。</p><Field><FieldLabel htmlFor="pipeline-session-text">会话请求</FieldLabel><Textarea id="pipeline-session-text" value={text} onChange={event=>setText(event.target.value)} placeholder="例如：创建读取项目快照并生成 Markdown 报告的任务。"/></Field><div className="flex gap-2"><Button type="submit" disabled={busy||!text.trim()||status?.phase!=="ready"}>提交会话请求</Button>{state?.paused&&<Button type="button" variant="outline" disabled={busy||status?.phase!=="ready"} onClick={()=>void run(()=>Pipeline.ResumeSessions())}>恢复会话队列</Button>}</div></form>
    <div className="max-h-56 space-y-2 overflow-y-auto">{requests.map(request=><div key={request.id} className="flex items-center gap-2 rounded-lg border p-2"><Button variant="ghost" className="min-w-0 flex-1 justify-start" onClick={()=>setSelected(request.id)}><span className="truncate">{request.input.role==="pipeline_config"?"配置":"管理"} · {request.kind==="instruction"?request.input.text:"完成汇报"}</span></Button><Badge variant="outline">{executionLabels[request.status]}</Badge>{(request.status==="queued"||request.status==="running")&&<Button size="xs" variant="outline" disabled={busy} onClick={()=>void run(()=>Pipeline.StopSession(request.id))}>停止</Button>}</div>)}</div>
    {current&&<><p className="text-xs text-muted-foreground">{current.input.model} · {current.input.effort} · 只读会话</p>{current.error&&<p className="text-sm text-destructive">{current.error}</p>}<Tabs defaultValue="result"><TabsList><TabsTrigger value="result">会话回复</TabsTrigger><TabsTrigger value="messages">会话过程</TabsTrigger><TabsTrigger value="request">请求内容</TabsTrigger></TabsList><TabsContent value="result">{current.result?<MarkdownView text={current.result} onError={onError}/>:<p className="text-sm text-muted-foreground">等待会话回复…</p>}</TabsContent><TabsContent value="messages"><TaskConversation threadId={current.threadId} turnId={current.turnId} running={current.status==="running"} status={status} onError={onError} onFollowup={setText}/></TabsContent><TabsContent value="request"><p className="whitespace-pre-wrap text-sm">{current.input.text}</p></TabsContent></Tabs></>}
  </div>;
}
